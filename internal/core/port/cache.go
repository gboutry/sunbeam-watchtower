// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// GitRepoCache manages local bare git clones used for reading commit history.
type GitRepoCache interface {
	EnsureRepo(ctx context.Context, cloneURL string, opts *dto.SyncOptions) (path string, err error)
	Fetch(ctx context.Context, cloneURL string, opts *dto.SyncOptions) error
	ListCommits(ctx context.Context, cloneURL string, opts forge.ListCommitsOpts) ([]forge.Commit, error)
	StoreMRMetadata(cloneURL string, mrs []dto.MRMetadata) error
	LoadMRMetadata(cloneURL string) ([]dto.MRMetadata, error)
	ListMRCommits(ctx context.Context, cloneURL string) ([]forge.Commit, error)
	ListBranches(ctx context.Context, cloneURL string) ([]string, error)
	Remove(cloneURL string) error
	RemoveAll() error
	CacheDir() string
}
