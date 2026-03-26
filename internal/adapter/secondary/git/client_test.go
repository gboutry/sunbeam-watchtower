// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package git_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestPush_DetachedHEAD(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	// Detach HEAD by checking out the commit hash directly.
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if err := wt.Checkout(&gogit.CheckoutOptions{Hash: head.Hash()}); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err = c.Push(dir, "origin", "HEAD", "refs/heads/main", false)
	if err == nil {
		t.Fatal("expected error for detached HEAD push")
	}
	if got := err.Error(); !strings.Contains(got, "detached") {
		t.Fatalf("expected detached HEAD error, got: %v", err)
	}
}

func TestPush_NonexistentRemote(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	err := c.Push(dir, "nonexistent", "refs/heads/master", "refs/heads/main", false)
	if err == nil {
		t.Fatal("expected error for nonexistent remote")
	}
}

func TestCreateBranchAndCheckout(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	if err := c.CreateBranch(dir, "feature", "HEAD"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := c.CheckoutBranch(dir, "feature"); err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}
	branch, err := c.CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "feature" {
		t.Errorf("CurrentBranch = %q, want feature", branch)
	}
}

func TestDeleteLocalBranch(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	if err := c.CreateBranch(dir, "to-delete", "HEAD"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := c.DeleteLocalBranch(dir, "to-delete"); err != nil {
		t.Fatalf("DeleteLocalBranch: %v", err)
	}
	// Checkout to deleted branch should fail.
	if err := c.CheckoutBranch(dir, "to-delete"); err == nil {
		t.Error("expected error checking out deleted branch")
	}
}

func TestCurrentBranch(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	branch, err := c.CurrentBranch(dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "master" {
		t.Errorf("CurrentBranch = %q, want master", branch)
	}
}

func TestAddAllAndCommit(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	origSHA, err := c.HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	// Write a new file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := c.AddAll(dir); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if err := c.Commit(dir, "add new file"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Repo should be clean now.
	dirty, err := c.HasUncommittedChanges(dir)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if dirty {
		t.Error("expected clean repo after commit")
	}

	// SHA should have changed.
	newSHA, err := c.HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if newSHA == origSHA {
		t.Error("expected SHA to change after commit")
	}
}

func TestResetHard(t *testing.T) {
	c := adapter.NewClient(nil)
	dir := initTestRepo(t)

	origSHA, err := c.HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	// Create a second commit.
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := c.AddAll(dir); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if err := c.Commit(dir, "second commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	newSHA, err := c.HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if newSHA == origSHA {
		t.Fatal("expected different SHA after second commit")
	}

	// Reset to original commit.
	if err := c.ResetHard(dir, origSHA); err != nil {
		t.Fatalf("ResetHard: %v", err)
	}

	resetSHA, err := c.HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if resetSHA != origSHA {
		t.Errorf("expected SHA %s after reset, got %s", origSHA, resetSHA)
	}
}
