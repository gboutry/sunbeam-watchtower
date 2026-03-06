// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package git_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	adapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/git"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initTestRepo creates a temporary git repo with one commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	// Create a file and commit it.
	f, err := os.Create(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	f.Close()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return dir
}

func TestIsRepo(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)
	if !c.IsRepo(dir) {
		t.Error("expected IsRepo to return true for a git repo")
	}
	nonGit := t.TempDir()
	if c.IsRepo(nonGit) {
		t.Error("expected IsRepo to return false for a non-git directory")
	}
}

func TestHeadSHA(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)
	sha, err := c.HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len %d)", sha, len(sha))
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	// Clean repo should have no uncommitted changes.
	dirty, err := c.HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if dirty {
		t.Error("expected no uncommitted changes in clean repo")
	}

	// Create an untracked file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dirty, err = c.HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !dirty {
		t.Error("expected uncommitted changes after adding a file")
	}
}

func TestAddAndRemoveRemote(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	if err := c.AddRemote(dir, "upstream", "https://example.com/repo.git"); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}

	// Verify remote exists by opening repo.
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := repo.Remote("upstream"); err != nil {
		t.Fatalf("expected remote upstream to exist: %v", err)
	}

	if err := c.RemoveRemote(dir, "upstream"); err != nil {
		t.Fatalf("RemoveRemote: %v", err)
	}
	if _, err := repo.Remote("upstream"); err == nil {
		t.Error("expected remote upstream to be removed")
	}
}
