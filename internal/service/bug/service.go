package bug

import (
	"context"
	"fmt"
	"sort"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

// ProjectBugTracker pairs a BugTracker with the project identifier it expects.
type ProjectBugTracker struct {
	Tracker   forge.BugTracker
	ProjectID string // e.g. LP project name
}

// ListOptions controls filtering for List.
type ListOptions struct {
	Projects   []string // filter to these config project names (empty = all)
	Status     []string
	Importance []string
	Assignee   string
	Tags       []string
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
}

// NewService creates a bug service.
// trackers maps dedup key to the tracker, projectMap maps dedup key to watchtower project names.
func NewService(trackers map[string]ProjectBugTracker, projectMap map[string][]string) *Service {
	return &Service{trackers: trackers, projectMap: projectMap}
}

// List returns bug tasks across all configured trackers, applying filters.
// Per-tracker errors are collected but do not stop aggregation (graceful degradation).
func (s *Service) List(ctx context.Context, opts ListOptions) ([]forge.BugTask, []ProjectResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

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
				continue
			}
		}

		tasks, err := pt.Tracker.ListBugTasks(ctx, pt.ProjectID, forge.ListBugTasksOpts{
			Status:     opts.Status,
			Importance: opts.Importance,
			Assignee:   opts.Assignee,
			Tags:       opts.Tags,
		})

		if err != nil {
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
		}
	}

	// Sort by UpdatedAt descending.
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})

	return all, results, nil
}
