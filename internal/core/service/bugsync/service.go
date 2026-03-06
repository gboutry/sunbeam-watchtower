// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package bugsync

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// ActionType describes the kind of sync action.
type ActionType = dto.BugSyncActionType

const (
	ActionStatusUpdate     ActionType = dto.BugSyncActionStatusUpdate
	ActionSeriesAssignment ActionType = dto.BugSyncActionSeriesAssignment
	ActionAddProjectTask   ActionType = dto.BugSyncActionAddProjectTask
)

// SyncAction represents a single action taken (or planned) during sync.
type SyncAction = dto.BugSyncAction

// SyncResult holds the outcome of a sync operation.
type SyncResult = dto.BugSyncResult

// SyncOptions controls the sync behavior.
type SyncOptions struct {
	Projects []string // filter to these watchtower project names (empty = all)
	DryRun   bool
	Since    *time.Time // only consider commits after this time
}

// BugBranch tracks which bug was found on which branch of which project.
type BugBranch struct {
	BugID   string
	Project string           // watchtower project name
	Branch  string           // "main", "stable/2024.1", etc.
	RefType forge.BugRefType // strongest ref type for this occurrence
}

// Service performs bug status synchronization from cached commits to LP.
type Service struct {
	commitSources map[string]port.CommitSource
	bugTracker    port.BugTracker
	lpProjects    []string // LP project names for searchTasks queries
	// Maps watchtower project name → LP bug project names.
	lpProjectMap map[string][]string
	logger       *slog.Logger

	// Caches to avoid redundant API calls.
	projectCache map[string]*forge.Project        // lpProject → Project
	seriesCache  map[string][]forge.ProjectSeries // lpProject → series list
}

// NewService creates a bug sync service.
// lpProjectMap maps watchtower project names to their associated LP bug project names.
func NewService(
	commitSources map[string]port.CommitSource,
	bugTracker port.BugTracker,
	lpProjects []string,
	lpProjectMap map[string][]string,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if lpProjectMap == nil {
		lpProjectMap = make(map[string][]string)
	}
	return &Service{
		commitSources: commitSources,
		bugTracker:    bugTracker,
		lpProjects:    lpProjects,
		lpProjectMap:  lpProjectMap,
		logger:        logger,
		projectCache:  make(map[string]*forge.Project),
		seriesCache:   make(map[string][]forge.ProjectSeries),
	}
}

// Sync scans cached commits for bug references and updates LP bug tasks.
func (s *Service) Sync(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

	// Phase 1: Collect bug references from all branches of all projects.
	bugBranches := make(map[string][]BugBranch) // bugID → []BugBranch

	for name, ps := range s.commitSources {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}

		branches, err := ps.ListBranches(ctx)
		if err != nil {
			s.logger.Warn("failed to list branches", "project", name, "error", err)
			continue
		}

		for _, branch := range branches {
			if !isRelevantBranch(branch) {
				continue
			}

			commits, err := ps.ListCommits(ctx, forge.ListCommitsOpts{Branch: branch})
			if err != nil {
				s.logger.Warn("failed to list commits", "project", name, "branch", branch, "error", err)
				continue
			}

			for _, c := range commits {
				for _, bugRef := range c.BugRefs {
					bugBranches[bugRef.ID] = appendUnique(bugBranches[bugRef.ID], BugBranch{
						BugID:   bugRef.ID,
						Project: name,
						Branch:  branch,
						RefType: bugRef.Type,
					})
				}
			}
		}
	}

	s.logger.Debug("bug references collected", "unique_bugs", len(bugBranches))

	// Phase 1.5: If Since is set, use searchTasks to restrict to recent bugs.
	if opts.Since != nil {
		eligible, err := s.fetchRecentBugIDs(ctx, *opts.Since)
		if err != nil {
			return nil, fmt.Errorf("fetching recent bugs: %w", err)
		}
		for bugID := range bugBranches {
			if !eligible[bugID] {
				s.logger.Debug("skipping bug (not in recent search)", "bug_id", bugID)
				delete(bugBranches, bugID)
			}
		}
		s.logger.Debug("filtered to recent bugs", "remaining", len(bugBranches))
	}

	// Phase 2: For each bug, update status and assign to series.
	result := &SyncResult{}

	for bugID, branches := range bugBranches {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Determine the strongest ref type across all branches for this bug.
		strongest := strongestRefType(branches)

		// Related-Bug references are informational — skip status changes entirely.
		if strongest == forge.BugRefRelated {
			s.logger.Debug("skipping bug (Related-Bug only)", "bug_id", bugID)
			result.Skipped++
			continue
		}

		bug, err := s.bugTracker.GetBug(ctx, bugID)
		if err != nil {
			s.logger.Warn("failed to fetch bug", "bug_id", bugID, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("bug %s: %w", bugID, err))
			continue
		}

		// Determine target status from ref type.
		targetStatus := refTypeToStatus(strongest)

		// Ensure bug has tasks for all LP projects associated with the watchtower projects.
		if err := s.ensureProjectTasks(ctx, bugID, bug, branches, opts.DryRun, result); err != nil {
			s.logger.Warn("failed to ensure project tasks", "bug_id", bugID, "error", err)
			result.Errors = append(result.Errors, err)
		}

		// Re-fetch bug after potential task additions to get updated task list.
		if !opts.DryRun {
			bug, err = s.bugTracker.GetBug(ctx, bugID)
			if err != nil {
				s.logger.Warn("failed to re-fetch bug after task addition", "bug_id", bugID, "error", err)
				result.Errors = append(result.Errors, fmt.Errorf("bug %s: %w", bugID, err))
				continue
			}
		}

		// Update task statuses.
		for _, task := range bug.Tasks {
			if task.Status == "Fix Released" {
				s.logger.Debug("skipping task (already Fix Released)", "bug_id", bugID, "task", task.Title)
				result.Skipped++
				continue
			}
			if task.Status == "Fix Committed" && targetStatus == "Fix Committed" {
				s.logger.Debug("skipping task (already Fix Committed)", "bug_id", bugID, "task", task.Title)
				result.Skipped++
				continue
			}
			// Don't downgrade: Fix Committed is stronger than In Progress.
			if task.Status == "Fix Committed" && targetStatus == "In Progress" {
				s.logger.Debug("skipping task (Fix Committed > In Progress)", "bug_id", bugID, "task", task.Title)
				result.Skipped++
				continue
			}
			// Don't update if already at target status.
			if task.Status == targetStatus {
				s.logger.Debug("skipping task (already at target)", "bug_id", bugID, "task", task.Title, "status", targetStatus)
				result.Skipped++
				continue
			}

			action := SyncAction{
				BugID:      bugID,
				TaskTitle:  task.Title,
				OldStatus:  task.Status,
				NewStatus:  targetStatus,
				SelfLink:   task.SelfLink,
				URL:        task.URL,
				ActionType: ActionStatusUpdate,
			}

			if !opts.DryRun {
				if err := s.bugTracker.UpdateBugTaskStatus(ctx, task.SelfLink, targetStatus); err != nil {
					s.logger.Warn("failed to update task status", "bug_id", bugID, "task", task.Title, "error", err)
					result.Errors = append(result.Errors, fmt.Errorf("bug %s task %q: %w", bugID, task.Title, err))
					continue
				}
			}

			result.Actions = append(result.Actions, action)
		}

		// Assign to series based on branches.
		if err := s.assignToSeries(ctx, bugID, bug, branches, opts.DryRun, result); err != nil {
			s.logger.Warn("series assignment failed", "bug_id", bugID, "error", err)
			result.Errors = append(result.Errors, err)
		}
	}

	return result, nil
}

// strongestRefType returns the strongest (lowest enum value) ref type from branches.
func strongestRefType(branches []BugBranch) forge.BugRefType {
	strongest := forge.BugRefRelated
	for _, bb := range branches {
		if bb.RefType < strongest {
			strongest = bb.RefType
		}
	}
	return strongest
}

// refTypeToStatus maps a BugRefType to the target LP bug status.
func refTypeToStatus(rt forge.BugRefType) string {
	switch rt {
	case forge.BugRefCloses:
		return "Fix Committed"
	case forge.BugRefPartial:
		return "In Progress"
	default:
		return ""
	}
}

// ensureProjectTasks adds bug tasks for LP projects associated with the watchtower
// projects where the bug was found, if those tasks don't already exist.
func (s *Service) ensureProjectTasks(ctx context.Context, bugID string, bug *forge.Bug, branches []BugBranch, dryRun bool, result *SyncResult) error {
	// Collect LP projects that should have tasks.
	neededProjects := make(map[string]bool)
	for _, bb := range branches {
		for _, lpProj := range s.lpProjectMap[bb.Project] {
			neededProjects[lpProj] = true
		}
	}

	// Check which LP projects already have tasks on this bug.
	existingProjects := make(map[string]bool)
	for _, task := range bug.Tasks {
		proj := task.TargetName
		if idx := strings.Index(proj, "/"); idx != -1 {
			proj = proj[:idx]
		}
		existingProjects[proj] = true
	}

	bugIDInt, err := strconv.Atoi(bugID)
	if err != nil {
		return fmt.Errorf("invalid bug ID %q: %w", bugID, err)
	}

	for lpProj := range neededProjects {
		if existingProjects[lpProj] {
			continue
		}

		// Get project self_link for AddBugTask.
		proj, err := s.getCachedProject(ctx, lpProj)
		if err != nil {
			s.logger.Warn("failed to get project for task addition", "project", lpProj, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("bug %s project %s: %w", bugID, lpProj, err))
			continue
		}

		action := SyncAction{
			BugID:      bugID,
			Project:    lpProj,
			ActionType: ActionAddProjectTask,
		}

		if !dryRun {
			if err := s.bugTracker.AddBugTask(ctx, bugIDInt, proj.SelfLink); err != nil {
				s.logger.Warn("failed to add project task", "bug_id", bugID, "project", lpProj, "error", err)
				result.Errors = append(result.Errors, fmt.Errorf("bug %s add task %s: %w", bugID, lpProj, err))
				continue
			}
		}

		result.Actions = append(result.Actions, action)
	}

	return nil
}

// assignToSeries handles series task assignment for a bug based on which branches it appears on.
func (s *Service) assignToSeries(ctx context.Context, bugID string, bug *forge.Bug, branches []BugBranch, dryRun bool, result *SyncResult) error {
	// Collect LP project slugs from bug tasks via TargetName.
	// TargetName may be "project" or "project/series" — use only the project part.
	lpProjects := make(map[string]bool)
	for _, task := range bug.Tasks {
		if task.TargetName != "" {
			proj := task.TargetName
			if idx := strings.Index(proj, "/"); idx != -1 {
				proj = proj[:idx]
			}
			lpProjects[proj] = true
		}
	}

	bugIDInt, err := strconv.Atoi(bugID)
	if err != nil {
		return fmt.Errorf("invalid bug ID %q: %w", bugID, err)
	}

	for lpProject := range lpProjects {
		for _, bb := range branches {
			seriesName := branchToSeriesName(bb.Branch)
			if seriesName == "" {
				continue
			}

			seriesLink, err := s.resolveSeriesLink(ctx, lpProject, seriesName)
			if err != nil {
				s.logger.Warn("failed to resolve series", "project", lpProject, "series", seriesName, "error", err)
				continue
			}
			if seriesLink == "" {
				s.logger.Debug("series not found", "project", lpProject, "series", seriesName)
				continue
			}

			action := SyncAction{
				BugID:      bugID,
				Series:     seriesName,
				Project:    lpProject,
				ActionType: ActionSeriesAssignment,
			}

			if !dryRun {
				if err := s.bugTracker.AddBugTask(ctx, bugIDInt, seriesLink); err != nil {
					// Assignment may fail if task already exists — log and continue.
					s.logger.Warn("series assignment failed (may already exist)", "bug_id", bugID, "series", seriesName, "error", err)
					continue
				}
			}

			result.Actions = append(result.Actions, action)
		}
	}

	return nil
}

// resolveSeriesLink finds the LP API self_link for a series, using cached results.
func (s *Service) resolveSeriesLink(ctx context.Context, lpProject, seriesName string) (string, error) {
	if seriesName == "development" {
		proj, err := s.getCachedProject(ctx, lpProject)
		if err != nil {
			return "", err
		}
		return proj.DevelopmentFocusLink, nil
	}

	series, err := s.getCachedSeries(ctx, lpProject)
	if err != nil {
		return "", err
	}
	for _, ps := range series {
		if ps.Name == seriesName {
			return ps.SelfLink, nil
		}
	}
	return "", nil
}

// getCachedProject returns a cached project or fetches it.
func (s *Service) getCachedProject(ctx context.Context, lpProject string) (*forge.Project, error) {
	if proj, ok := s.projectCache[lpProject]; ok {
		return proj, nil
	}
	proj, err := s.bugTracker.GetProject(ctx, lpProject)
	if err != nil {
		return nil, err
	}
	s.projectCache[lpProject] = proj
	return proj, nil
}

// getCachedSeries returns cached series or fetches them.
func (s *Service) getCachedSeries(ctx context.Context, lpProject string) ([]forge.ProjectSeries, error) {
	if series, ok := s.seriesCache[lpProject]; ok {
		return series, nil
	}
	series, err := s.bugTracker.GetProjectSeries(ctx, lpProject)
	if err != nil {
		return nil, err
	}
	s.seriesCache[lpProject] = series
	return series, nil
}

// fetchRecentBugIDs uses searchTasks with created_since to build a set of bug IDs.
func (s *Service) fetchRecentBugIDs(ctx context.Context, since time.Time) (map[string]bool, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	eligible := make(map[string]bool)

	for _, proj := range s.lpProjects {
		tasks, err := s.bugTracker.ListBugTasks(ctx, proj, forge.ListBugTasksOpts{
			CreatedSince: sinceStr,
		})
		if err != nil {
			s.logger.Warn("failed to search recent bugs", "project", proj, "error", err)
			continue
		}
		for _, t := range tasks {
			eligible[t.BugID] = true
		}
		s.logger.Debug("fetched recent bugs for project", "project", proj, "count", len(tasks))
	}

	return eligible, nil
}

// branchToSeriesName maps a git branch to an LP series name.
// Returns "" for branches that don't map to a series.
func branchToSeriesName(branch string) string {
	switch branch {
	case "main", "master":
		return "development" // sentinel for development focus
	}
	if strings.HasPrefix(branch, "stable/") {
		return strings.TrimPrefix(branch, "stable/")
	}
	return ""
}

// isRelevantBranch returns true for branches we should scan (main, master, stable/*).
func isRelevantBranch(branch string) bool {
	if branch == "main" || branch == "master" {
		return true
	}
	return strings.HasPrefix(branch, "stable/")
}

// appendUnique appends a BugBranch if not already present for the same project+branch.
// If already present, promotes to the stronger ref type.
func appendUnique(slice []BugBranch, bb BugBranch) []BugBranch {
	for i, existing := range slice {
		if existing.Project == bb.Project && existing.Branch == bb.Branch {
			if bb.RefType < existing.RefType {
				slice[i].RefType = bb.RefType
			}
			return slice
		}
	}
	return append(slice, bb)
}
