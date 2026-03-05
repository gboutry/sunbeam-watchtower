// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

// SyncOptions controls additional behavior when syncing a cached repository.
type SyncOptions struct {
	ExtraRefSpecs []string // additional refspecs to fetch, e.g. "+refs/pull/*/head:refs/pull/*/head"
}

// MRMetadata stores merge request information as a sidecar to the cached repo.
type MRMetadata struct {
	ID      string          `json:"id"`
	State   forge.MergeState `json:"state"`
	URL     string          `json:"url"`
	HeadSHA string          `json:"head_sha"`
	GitRef  string          `json:"git_ref"` // e.g. "refs/pull/123/head"
}

// GitRepoCache manages local bare git clones used for reading commit history.
type GitRepoCache interface {
	// EnsureRepo clones the repository if missing, or fetches if it already exists.
	// Returns the local filesystem path of the bare clone.
	// If opts is non-nil, extra refspecs are fetched in addition to the defaults.
	EnsureRepo(ctx context.Context, cloneURL string, opts *SyncOptions) (path string, err error)

	// Fetch updates an existing cached repository from origin.
	// If opts is non-nil, extra refspecs are fetched in addition to the defaults.
	Fetch(ctx context.Context, cloneURL string, opts *SyncOptions) error

	// ListCommits reads commit history from a cached repository.
	ListCommits(ctx context.Context, cloneURL string, opts forge.ListCommitsOpts) ([]forge.Commit, error)

	// StoreMRMetadata writes merge request metadata as a sidecar JSON file.
	StoreMRMetadata(cloneURL string, mrs []MRMetadata) error

	// LoadMRMetadata reads merge request metadata from the sidecar JSON file.
	LoadMRMetadata(cloneURL string) ([]MRMetadata, error)

	// ListMRCommits reads the head commit for each cached merge request ref.
	ListMRCommits(ctx context.Context, cloneURL string) ([]forge.Commit, error)

	// ListBranches returns the branch names available in a cached repository.
	ListBranches(ctx context.Context, cloneURL string) ([]string, error)

	// Remove deletes a single cached repository.
	Remove(cloneURL string) error

	// RemoveAll deletes all cached repositories.
	RemoveAll() error

	// CacheDir returns the base directory used for cached repositories.
	CacheDir() string
}
