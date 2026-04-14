// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import "context"

// GitClient handles local git operations.
type GitClient interface {
	IsRepo(path string) bool
	HeadSHA(path string) (string, error)
	HasUncommittedChanges(path string) (bool, error)
	Push(path, remote, localRef, remoteRef string, force bool) error
	AddRemote(path, name, url string) error
	RemoveRemote(path, name string) error

	// Branch operations
	CreateBranch(path, branchName, startPoint string) error
	CheckoutBranch(path, branchName string) error
	CurrentBranch(path string) (string, error)
	DeleteLocalBranch(path, branchName string) error

	// Staging and committing
	AddAll(path string) error
	Commit(path, message string) error

	// Detached-worktree operations for isolated prepare/push flows.
	//
	// CreateDetachedWorktree materialises a temporary linked worktree of
	// repoPath at the given sha on a new local branch named `branch`. It
	// honours $TMPDIR for the worktree directory (required for
	// snap-confined invocations). The returned cleanup closure must be
	// called to remove the worktree, the local branch, prune stale
	// `.git/worktrees/<name>` metadata from repoPath, and remove the
	// temporary directory. Cleanup is safe to call multiple times.
	CreateDetachedWorktree(ctx context.Context, repoPath, branch, sha string) (worktreePath string, cleanup func(), err error)

	// Reset
	ResetHard(path, ref string) error
}
