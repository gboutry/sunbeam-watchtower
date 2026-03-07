// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package gitcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestReadHEADFileAndFindHEADFilesByBaseName(t *testing.T) {
	repoPath := createBareRepoWithFiles(t, map[string]string{
		"snap/snapcraft.yaml":             "name: snap-openstack\n",
		"charms/keystone/charmcraft.yaml": "name: keystone-k8s\n",
		"charms/glance/charmcraft.yaml":   "name: glance-k8s\n",
	})

	content, err := ReadHEADFile(repoPath, "snap/snapcraft.yaml")
	if err != nil {
		t.Fatalf("ReadHEADFile() error = %v", err)
	}
	if string(content) != "name: snap-openstack\n" {
		t.Fatalf("ReadHEADFile() = %q, want snapcraft contents", content)
	}

	files, err := FindHEADFilesByBaseName(repoPath, "charmcraft.yaml")
	if err != nil {
		t.Fatalf("FindHEADFilesByBaseName() error = %v", err)
	}
	if len(files) != 2 || files[0].Path != "charms/glance/charmcraft.yaml" || files[1].Path != "charms/keystone/charmcraft.yaml" {
		t.Fatalf("FindHEADFilesByBaseName() = %+v, want two sorted charmcraft files", files)
	}
}

func createBareRepoWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()

	worktreeDir := filepath.Join(t.TempDir(), "work")
	repo, err := git.PlainInit(worktreeDir, false)
	if err != nil {
		t.Fatalf("PlainInit() error = %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}
	for name, content := range files {
		fullPath := filepath.Join(worktreeDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", fullPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", fullPath, err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("Add(%q) error = %v", name, err)
		}
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com"},
	}); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	bareDir := filepath.Join(t.TempDir(), "bare.git")
	if _, err := git.PlainClone(bareDir, true, &git.CloneOptions{URL: worktreeDir}); err != nil {
		t.Fatalf("PlainClone() error = %v", err)
	}
	return bareDir
}
