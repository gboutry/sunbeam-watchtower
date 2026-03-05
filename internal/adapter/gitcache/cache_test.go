// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package gitcache

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

// setupTestRepo creates a temporary bare repo with some commits and returns its URL.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	bareDir := filepath.Join(dir, "bare.git")

	// Create a working repo and add commits.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test Author",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test Author",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test Author")

	// First commit.
	if err := os.WriteFile(filepath.Join(workDir, "file1.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "file1.txt")
	run("git", "commit", "-m", "initial commit\n\nLP: #12345")

	// Second commit.
	if err := os.WriteFile(filepath.Join(workDir, "file2.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "file2.txt")
	run("git", "commit", "-m", "second commit\n\nCloses-Bug: #67890")

	// Clone to bare.
	cmd := exec.Command("git", "clone", "--bare", workDir, bareDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bare clone failed: %v\n%s", err, out)
	}

	return bareDir
}

func TestCache_EnsureRepo_CloneAndList(t *testing.T) {
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)

	ctx := context.Background()
	// Use file:// URL so repoPath can parse it.
	cloneURL := "file://" + bareRepo

	// EnsureRepo should clone.
	path, err := cache.EnsureRepo(ctx, cloneURL)
	if err != nil {
		t.Fatalf("EnsureRepo() error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cloned repo not found at %s", path)
	}

	// ListCommits should return 2 commits.
	commits, err := cache.ListCommits(ctx, cloneURL, forge.ListCommitsOpts{Branch: "main"})
	if err != nil {
		t.Fatalf("ListCommits() error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// Newest first (committer time order).
	if commits[0].Message == "" {
		t.Error("commit message should not be empty")
	}
	if commits[0].Author != "Test Author" {
		t.Errorf("author = %q, want %q", commits[0].Author, "Test Author")
	}

	// Check bug refs were extracted.
	allBugRefs := make(map[string]bool)
	for _, c := range commits {
		for _, ref := range c.BugRefs {
			allBugRefs[ref] = true
		}
	}
	if !allBugRefs["12345"] {
		t.Error("expected bug ref 12345")
	}
	if !allBugRefs["67890"] {
		t.Error("expected bug ref 67890")
	}
}

func TestCache_EnsureRepo_FetchExisting(t *testing.T) {
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	// Clone first.
	_, err := cache.EnsureRepo(ctx, cloneURL)
	if err != nil {
		t.Fatalf("first EnsureRepo() error: %v", err)
	}

	// EnsureRepo again should fetch (not fail).
	path, err := cache.EnsureRepo(ctx, cloneURL)
	if err != nil {
		t.Fatalf("second EnsureRepo() error: %v", err)
	}
	if path == "" {
		t.Error("path should not be empty")
	}
}

func TestCache_ListCommits_SinceFilter(t *testing.T) {
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	if _, err := cache.EnsureRepo(ctx, cloneURL); err != nil {
		t.Fatal(err)
	}

	// Since far in the future should return no commits.
	future := time.Now().Add(24 * time.Hour)
	commits, err := cache.ListCommits(ctx, cloneURL, forge.ListCommitsOpts{
		Branch: "main",
		Since:  &future,
	})
	if err != nil {
		t.Fatalf("ListCommits() error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits with future Since, got %d", len(commits))
	}
}

func TestCache_Remove(t *testing.T) {
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	path, err := cache.EnsureRepo(ctx, cloneURL)
	if err != nil {
		t.Fatal(err)
	}

	if err := cache.Remove(cloneURL); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("repo should have been removed")
	}
}

func TestCache_RemoveAll(t *testing.T) {
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	if _, err := cache.EnsureRepo(ctx, cloneURL); err != nil {
		t.Fatal(err)
	}

	if err := cache.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll() error: %v", err)
	}

	if _, err := os.Stat(cache.CacheDir()); !os.IsNotExist(err) {
		t.Error("cache dir should have been removed")
	}
}

func TestCache_repoPath(t *testing.T) {
	cache := NewCache("/tmp/cache/repos", nil)

	tests := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{
			url:  "https://github.com/org/repo.git",
			want: "/tmp/cache/repos/github.com/org/repo.git",
		},
		{
			url:  "https://github.com/org/repo",
			want: "/tmp/cache/repos/github.com/org/repo.git",
		},
		{
			url:  "https://review.opendev.org/openstack/nova",
			want: "/tmp/cache/repos/review.opendev.org/openstack/nova.git",
		},
		{
			url:  "https://git.launchpad.net/sunbeam",
			want: "/tmp/cache/repos/git.launchpad.net/sunbeam.git",
		},
		{
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got, err := cache.repoPath(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("repoPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
