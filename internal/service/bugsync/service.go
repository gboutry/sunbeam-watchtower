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

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"github.com/gboutry/sunbeam-watchtower/internal/service/commit"
)

// ActionType describes the kind of sync action.
type ActionType string

const (
	ActionStatusUpdate      ActionType = "status_update"
	ActionSeriesAssignment  ActionType = "series_assignment"
)

// SyncAction represents a single action taken (or planned) during sync.
type SyncAction struct {
	BugID      string
	TaskTitle  string
	OldStatus  string
	NewStatus  string
	SelfLink   string
	URL        string
	Series     string // series name if assigned
	Project    string // LP project name
	ActionType ActionType
}

// SyncResult holds the outcome of a sync operation.
type SyncResult struct {
	Actions []SyncAction
	Skipped int
	Errors  []error
}

// SyncOptions controls the sync behavior.
type SyncOptions struct {
	Projects []string   // filter to these watchtower project names (empty = all)
	DryRun   bool
	Since    *time.Time // only consider commits after this time
}

// BugBranch tracks which bug was found on which branch of which project.
type BugBranch struct {
	BugID   string
	Project string // watchtower project name
	Branch  string // "main", "stable/2024.1", etc.
}

// Service performs bug status synchronization from cached commits to LP.
type Service struct {
	commitSources map[string]commit.ProjectSource
	bugTracker    port.BugTracker
	lpProjects    []string // LP project names for searchTasks queries
	logger        *slog.Logger

	// Caches to avoid redundant API calls.
	projectCache map[string]*forge.Project       // lpProject → Project
	seriesCache  map[string][]forge.ProjectSeries // lpProject → series list
}

// NewService creates a bug sync service.
func NewService(
	commitSources map[string]commit.ProjectSource,
	bugTracker port.BugTracker,
	lpProjects []string,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		commitSources: commitSources,
		bugTracker:    bugTracker,
		lpProjects:    lpProjects,
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

		branches, err := ps.Source.ListBranches(ctx)
		if err != nil {
			s.logger.Warn("failed to list branches", "project", name, "error", err)
			continue
		}

		for _, branch := range branches {
			if !isRelevantBranch(branch) {
				continue
			}

			commits, err := ps.Source.ListCommits(ctx, forge.ListCommitsOpts{Branch: branch})
			if err != nil {
				s.logger.Warn("failed to list commits", "project", name, "branch", branch, "error", err)
				continue
			}

			for _, c := range commits {
				for _, bugID := range c.BugRefs {
					bugBranches[bugID] = appendUnique(bugBranches[bugID], BugBranch{
						BugID:   bugID,
						Project: name,
						Branch:  branch,
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

		bug, err := s.bugTracker.GetBug(ctx, bugID)
		if err != nil {
			s.logger.Warn("failed to fetch bug", "bug_id", bugID, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("bug %s: %w", bugID, err))
			continue
		}

		// Update task statuses.
		for _, task := range bug.Tasks {
			if task.Status == "Fix Released" {
				s.logger.Debug("skipping task (already Fix Released)", "bug_id", bugID, "task", task.Title)
				result.Skipped++
				continue
			}
			if task.Status == "Fix Committed" {
				s.logger.Debug("skipping task (already Fix Committed)", "bug_id", bugID, "task", task.Title)
				result.Skipped++
				continue
			}

			action := SyncAction{
				BugID:      bugID,
				TaskTitle:  task.Title,
				OldStatus:  task.Status,
				NewStatus:  "Fix Committed",
				SelfLink:   task.SelfLink,
				URL:        task.URL,
				ActionType: ActionStatusUpdate,
			}

			if !opts.DryRun {
				if err := s.bugTracker.UpdateBugTaskStatus(ctx, task.SelfLink, "Fix Committed"); err != nil {
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

// appendUnique appends a BugBranch if not already present.
func appendUnique(slice []BugBranch, bb BugBranch) []BugBranch {
	for _, existing := range slice {
		if existing.Project == bb.Project && existing.Branch == bb.Branch {
			return slice
		}
	}
	return append(slice, bb)
}
