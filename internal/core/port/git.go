// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

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

	// Reset
	ResetHard(path, ref string) error
}
