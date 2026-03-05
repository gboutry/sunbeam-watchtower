// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package commit

import (
	"context"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// Ensure implementations satisfy CommitSource.
var (
	_ CommitSource = (*ForgeCommitSource)(nil)
	_ CommitSource = (*CachedGitSource)(nil)
)

// CommitSource can list commits for a single project.
type CommitSource interface {
	ListCommits(ctx context.Context, opts forge.ListCommitsOpts) ([]forge.Commit, error)
}

// ProjectSource pairs a CommitSource with metadata about the forge.
type ProjectSource struct {
	Source    CommitSource
	ForgeType forge.ForgeType
}

// ForgeCommitSource wraps an existing ForgeClient for backward compatibility.
type ForgeCommitSource struct {
	Forge     ForgeClient
	ProjectID string
}

// ListCommits delegates to the underlying Forge.
func (f *ForgeCommitSource) ListCommits(ctx context.Context, opts forge.ListCommitsOpts) ([]forge.Commit, error) {
	return f.Forge.ListCommits(ctx, f.ProjectID, opts)
}

// CachedGitSource reads commits from a local git cache.
type CachedGitSource struct {
	Cache    port.GitRepoCache
	CloneURL string
	Code     config.CodeConfig
}

// ListCommits reads commits from the local git cache and populates commit URLs.
func (c *CachedGitSource) ListCommits(ctx context.Context, opts forge.ListCommitsOpts) ([]forge.Commit, error) {
	commits, err := c.Cache.ListCommits(ctx, c.CloneURL, opts)
	if err != nil {
		return nil, err
	}

	forgeType := forgeTypeFromConfig(c.Code.Forge)
	for i := range commits {
		commits[i].Forge = forgeType
		commits[i].URL = c.Code.CommitURL(commits[i].SHA)
	}

	return commits, nil
}

func forgeTypeFromConfig(forgeName string) forge.ForgeType {
	switch forgeName {
	case "github":
		return forge.ForgeGitHub
	case "launchpad":
		return forge.ForgeLaunchpad
	case "gerrit":
		return forge.ForgeGerrit
	default:
		return forge.ForgeGitHub
	}
}
