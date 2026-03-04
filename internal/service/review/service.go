package review

import (
	"context"
	"fmt"
	"sort"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	port "github.com/gboutry/sunbeam-watchtower/internal/port"
)

// ProjectForge pairs a Forge client with the project identifier it expects.
type ProjectForge struct {
	Forge     port.Forge
	ProjectID string // e.g. "owner/repo" for GitHub, project path for Gerrit/LP
}

// ListOptions controls filtering for List.
type ListOptions struct {
	Projects []string          // filter to these config project names (empty = all)
	Forges   []forge.ForgeType // filter to these forge types (empty = all)
	State    forge.MergeState
	Author   string
}

// ProjectResult holds merge requests from one project, or an error.
type ProjectResult struct {
	ProjectName   string
	MergeRequests []forge.MergeRequest
	Err           error
}

// Service aggregates reviews across multiple forges.
type Service struct {
	projects map[string]ProjectForge
}

// NewService creates a review service with the given project-to-forge mappings.
func NewService(projects map[string]ProjectForge) *Service {
	return &Service{projects: projects}
}

// Get returns a single merge request by project name and ID.
func (s *Service) Get(ctx context.Context, project string, id string) (*forge.MergeRequest, error) {
	pf, ok := s.projects[project]
	if !ok {
		return nil, fmt.Errorf("unknown project %q", project)
	}

	mr, err := pf.Forge.GetMergeRequest(ctx, pf.ProjectID, id)
	if err != nil {
		return nil, err
	}
	mr.Repo = project
	return mr, nil
}

// List returns merge requests across all configured projects, applying filters.
// Per-project errors are collected but do not stop aggregation (graceful degradation).
func (s *Service) List(ctx context.Context, opts ListOptions) ([]forge.MergeRequest, []ProjectResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}
	forgeFilter := make(map[forge.ForgeType]bool, len(opts.Forges))
	for _, f := range opts.Forges {
		forgeFilter[f] = true
	}

	var results []ProjectResult
	var all []forge.MergeRequest

	for name, pf := range s.projects {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}
		if len(forgeFilter) > 0 && !forgeFilter[pf.Forge.Type()] {
			continue
		}

		mrs, err := pf.Forge.ListMergeRequests(ctx, pf.ProjectID, forge.ListMergeRequestsOpts{
			State:  opts.State,
			Author: opts.Author,
		})

		result := ProjectResult{ProjectName: name}
		if err != nil {
			result.Err = fmt.Errorf("%s: %w", name, err)
		} else {
			for i := range mrs {
				mrs[i].Repo = name
			}
			result.MergeRequests = mrs
			all = append(all, mrs...)
		}
		results = append(results, result)
	}

	// Sort by UpdatedAt descending.
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})

	return all, results, nil
}
