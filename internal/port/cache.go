// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

// GitRepoCache manages local bare git clones used for reading commit history.
type GitRepoCache interface {
	// EnsureRepo clones the repository if missing, or fetches if it already exists.
	// Returns the local filesystem path of the bare clone.
	EnsureRepo(ctx context.Context, cloneURL string) (path string, err error)

	// Fetch updates an existing cached repository from origin.
	Fetch(ctx context.Context, cloneURL string) error

	// ListCommits reads commit history from a cached repository.
	ListCommits(ctx context.Context, cloneURL string, opts forge.ListCommitsOpts) ([]forge.Commit, error)

	// Remove deletes a single cached repository.
	Remove(cloneURL string) error

	// RemoveAll deletes all cached repositories.
	RemoveAll() error

	// CacheDir returns the base directory used for cached repositories.
	CacheDir() string
}
