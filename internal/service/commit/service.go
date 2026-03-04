// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package commit

import (
	"context"
	"fmt"
	"sort"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
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
	Branch   string            // branch to list commits from (empty = default)
	Author   string            // filter by author
	BugID    string            // filter to commits referencing this bug ID
}

// ProjectResult holds commits from one project, or an error.
type ProjectResult struct {
	ProjectName string
	Commits     []forge.Commit
	Err         error
}

// Service aggregates commits across multiple forges.
type Service struct {
	projects map[string]ProjectForge
}

// NewService creates a commit service with the given project-to-forge mappings.
func NewService(projects map[string]ProjectForge) *Service {
	return &Service{projects: projects}
}

// List returns commits across all configured projects, applying filters.
// Per-project errors are collected but do not stop aggregation (graceful degradation).
func (s *Service) List(ctx context.Context, opts ListOptions) ([]forge.Commit, []ProjectResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}
	forgeFilter := make(map[forge.ForgeType]bool, len(opts.Forges))
	for _, f := range opts.Forges {
		forgeFilter[f] = true
	}

	var results []ProjectResult
	var all []forge.Commit

	for name, pf := range s.projects {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}
		if len(forgeFilter) > 0 && !forgeFilter[pf.Forge.Type()] {
			continue
		}

		commits, err := pf.Forge.ListCommits(ctx, pf.ProjectID, forge.ListCommitsOpts{
			Branch: opts.Branch,
			Author: opts.Author,
		})

		result := ProjectResult{ProjectName: name}
		if err != nil {
			result.Err = fmt.Errorf("%s: %w", name, err)
		} else {
			for i := range commits {
				commits[i].Repo = name
			}

			if opts.BugID != "" {
				commits = filterByBugRef(commits, opts.BugID)
			}

			result.Commits = commits
			all = append(all, commits...)
		}
		results = append(results, result)
	}

	// Sort by Date descending (newest first).
	sort.Slice(all, func(i, j int) bool {
		return all[i].Date.After(all[j].Date)
	})

	return all, results, nil
}

// filterByBugRef returns only commits whose BugRefs contain the given bug ID.
func filterByBugRef(commits []forge.Commit, bugID string) []forge.Commit {
	var filtered []forge.Commit
	for _, c := range commits {
		for _, ref := range c.BugRefs {
			if ref == bugID {
				filtered = append(filtered, c)
				break
			}
		}
	}
	return filtered
}
