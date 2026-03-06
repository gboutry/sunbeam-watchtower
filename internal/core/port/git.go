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
}
