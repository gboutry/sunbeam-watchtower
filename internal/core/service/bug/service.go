package bug

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"

	port "github.com/gboutry/sunbeam-watchtower/internal/core/port"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// ProjectBugTracker pairs a BugTracker with the project identifier it expects.
type ProjectBugTracker struct {
	Tracker   port.BugTracker
	ProjectID string // e.g. LP project name
}

// ListOptions controls filtering for List.
type ListOptions struct {
	Projects   []string // filter to these config project names (empty = all)
	Status     []string
	Importance []string
	Assignee   string
	Tags       []string
	Since      string // ISO 8601 date — filter tasks created/modified since this date
}

// ProjectResult holds bug tasks from one query, or an error.
type ProjectResult struct {
	ProjectName string
	BugTasks    []forge.BugTask
	Err         error
}

// Service aggregates bug tasks across multiple bug trackers.
type Service struct {
	// keyed by dedup key "forge:projectID" to avoid querying the same tracker project twice
	trackers map[string]ProjectBugTracker
	// maps dedup key → list of watchtower project names
	projectMap map[string][]string
	logger     *slog.Logger
}

// NewService creates a bug service.
// trackers maps dedup key to the tracker, projectMap maps dedup key to watchtower project names.
func NewService(trackers map[string]ProjectBugTracker, projectMap map[string][]string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{trackers: trackers, projectMap: projectMap, logger: logger}
}

// Get fetches a single bug by ID. It tries each configured tracker until one succeeds.
func (s *Service) Get(ctx context.Context, id string) (*forge.Bug, error) {
	s.logger.Debug("getting bug", "id", id)

	// Try each unique tracker (deduplicated).
	seen := make(map[port.BugTracker]bool)
	for _, pt := range s.trackers {
		if seen[pt.Tracker] {
			continue
		}
		seen[pt.Tracker] = true

		bug, err := pt.Tracker.GetBug(ctx, id)
		if err == nil {
			return bug, nil
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("no bug trackers configured")
	}
	return nil, fmt.Errorf("bug %s not found", id)
}

// List returns bug tasks across all configured trackers, applying filters.
// Per-tracker errors are collected but do not stop aggregation (graceful degradation).
func (s *Service) List(ctx context.Context, opts ListOptions) ([]forge.BugTask, []ProjectResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

	s.logger.Debug("listing bugs",
		"tracker_count", len(s.trackers),
		"projects_filter", opts.Projects,
		"status_filter", opts.Status,
		"importance_filter", opts.Importance,
		"assignee", opts.Assignee,
		"tags", opts.Tags,
	)

	var results []ProjectResult
	var all []forge.BugTask

	for key, pt := range s.trackers {
		projectNames := s.projectMap[key]

		// If project filter is set, check if any associated project matches.
		if len(projFilter) > 0 {
			var matched bool
			for _, name := range projectNames {
				if projFilter[name] {
					matched = true
					break
				}
			}
			if !matched {
				s.logger.Debug("skipping tracker (filtered)", "tracker", key)
				continue
			}
		}

		s.logger.Debug("querying tracker", "tracker", key, "projects", projectNames)

		tasks, err := pt.Tracker.ListBugTasks(ctx, pt.ProjectID, forge.ListBugTasksOpts{
			Status:        opts.Status,
			Importance:    opts.Importance,
			Assignee:      opts.Assignee,
			Tags:          opts.Tags,
			CreatedSince:  opts.Since,
			ModifiedSince: opts.Since,
		})

		if err != nil {
			s.logger.Warn("tracker query failed", "tracker", key, "error", err)
			for _, name := range projectNames {
				results = append(results, ProjectResult{
					ProjectName: name,
					Err:         fmt.Errorf("%s: %w", name, err),
				})
			}
			continue
		}

		// Expand results to all associated watchtower projects.
		for _, name := range projectNames {
			if len(projFilter) > 0 && !projFilter[name] {
				continue
			}
			expanded := make([]forge.BugTask, len(tasks))
			for i, t := range tasks {
				t.Project = name
				expanded[i] = t
			}
			results = append(results, ProjectResult{
				ProjectName: name,
				BugTasks:    expanded,
			})
			all = append(all, expanded...)
			s.logger.Debug("tracker bugs fetched", "tracker", key, "project", name, "count", len(expanded))
		}
	}

	// Sort by UpdatedAt descending.
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})

	s.logger.Debug("bugs aggregated", "total_count", len(all))

	return all, results, nil
}
