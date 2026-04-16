// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package gitcache

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"

	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

var (
	testRepoOnce sync.Once
	testRepoPath string
	errTestRepo  error
)

// setupTestRepo creates a package-shared bare repo fixture with some commits.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary required")
	}

	testRepoOnce.Do(func() {
		dir, err := os.MkdirTemp("", "watchtower-gitcache-fixture-*")
		if err != nil {
			errTestRepo = err
			return
		}
		workDir := filepath.Join(dir, "work")
		bareDir := filepath.Join(dir, "bare.git")

		run := func(args ...string) error {
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
				return fmt.Errorf("command %v failed: %w\n%s", args, err, out)
			}
			return nil
		}

		if err := os.MkdirAll(workDir, 0o755); err != nil {
			errTestRepo = err
			return
		}

		if err := run("git", "init", "-b", "main"); err != nil {
			errTestRepo = err
			return
		}
		if err := run("git", "config", "user.email", "test@example.com"); err != nil {
			errTestRepo = err
			return
		}
		if err := run("git", "config", "user.name", "Test Author"); err != nil {
			errTestRepo = err
			return
		}

		if err := os.WriteFile(filepath.Join(workDir, "file1.txt"), []byte("hello"), 0o644); err != nil {
			errTestRepo = err
			return
		}
		if err := run("git", "add", "file1.txt"); err != nil {
			errTestRepo = err
			return
		}
		if err := run("git", "commit", "-m", "initial commit\n\nLP: #12345"); err != nil {
			errTestRepo = err
			return
		}

		if err := os.WriteFile(filepath.Join(workDir, "file2.txt"), []byte("world"), 0o644); err != nil {
			errTestRepo = err
			return
		}
		if err := run("git", "add", "file2.txt"); err != nil {
			errTestRepo = err
			return
		}
		if err := run("git", "commit", "-m", "second commit\n\nCloses-Bug: #67890"); err != nil {
			errTestRepo = err
			return
		}

		cmd := exec.Command("git", "clone", "--bare", workDir, bareDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			errTestRepo = fmt.Errorf("bare clone failed: %w\n%s", err, out)
			return
		}

		testRepoPath = bareDir
	})
	if errTestRepo != nil {
		t.Fatalf("setupTestRepo() error: %v", errTestRepo)
	}
	return testRepoPath
}

func TestCache_EnsureRepo_CloneAndList(t *testing.T) {
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)

	ctx := context.Background()
	// Use file:// URL so repoPath can parse it.
	cloneURL := "file://" + bareRepo

	// EnsureRepo should clone.
	path, err := cache.EnsureRepo(ctx, cloneURL, nil)
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
			allBugRefs[ref.ID] = true
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
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	// Clone first.
	_, err := cache.EnsureRepo(ctx, cloneURL, nil)
	if err != nil {
		t.Fatalf("first EnsureRepo() error: %v", err)
	}

	// EnsureRepo again should fetch (not fail).
	path, err := cache.EnsureRepo(ctx, cloneURL, nil)
	if err != nil {
		t.Fatalf("second EnsureRepo() error: %v", err)
	}
	if path == "" {
		t.Error("path should not be empty")
	}
}

func TestCache_ListCommits_SinceFilter(t *testing.T) {
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	if _, err := cache.EnsureRepo(ctx, cloneURL, nil); err != nil {
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
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	path, err := cache.EnsureRepo(ctx, cloneURL, nil)
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
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	if _, err := cache.EnsureRepo(ctx, cloneURL, nil); err != nil {
		t.Fatal(err)
	}

	if err := cache.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll() error: %v", err)
	}

	if _, err := os.Stat(cache.CacheDir()); !os.IsNotExist(err) {
		t.Error("cache dir should have been removed")
	}
}

func TestCache_StoreMRMetadata_RoundTrip(t *testing.T) {
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	if _, err := cache.EnsureRepo(ctx, cloneURL, nil); err != nil {
		t.Fatal(err)
	}

	// Store metadata.
	mrs := []dto.MRMetadata{
		{ID: "#42", State: forge.MergeStateOpen, URL: "https://github.com/org/repo/pull/42", HeadSHA: "abc123", GitRef: "refs/pull/42/head"},
		{ID: "#43", State: forge.MergeStateMerged, URL: "https://github.com/org/repo/pull/43", HeadSHA: "def456", GitRef: "refs/pull/43/head"},
	}
	if err := cache.StoreMRMetadata(cloneURL, mrs); err != nil {
		t.Fatalf("StoreMRMetadata() error: %v", err)
	}

	// Load and verify.
	loaded, err := cache.LoadMRMetadata(cloneURL)
	if err != nil {
		t.Fatalf("LoadMRMetadata() error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 MR metadata entries, got %d", len(loaded))
	}
	if loaded[0].ID != "#42" {
		t.Errorf("first MR ID = %q, want %q", loaded[0].ID, "#42")
	}
	if loaded[0].State != forge.MergeStateOpen {
		t.Errorf("first MR state = %v, want Open", loaded[0].State)
	}
	if loaded[1].State != forge.MergeStateMerged {
		t.Errorf("second MR state = %v, want Merged", loaded[1].State)
	}
}

func TestCache_LoadMRMetadata_NoFile(t *testing.T) {
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()

	cache := NewCache(filepath.Join(cacheDir, "repos"), nil)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	if _, err := cache.EnsureRepo(ctx, cloneURL, nil); err != nil {
		t.Fatal(err)
	}

	// No metadata file should return nil without error.
	mrs, err := cache.LoadMRMetadata(cloneURL)
	if err != nil {
		t.Fatalf("LoadMRMetadata() error: %v", err)
	}
	if mrs != nil {
		t.Errorf("expected nil, got %v", mrs)
	}
}

func TestCache_ListMRCommits(t *testing.T) {
	t.Parallel()
	bareRepo := setupTestRepo(t)
	cacheDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cache := NewCache(filepath.Join(cacheDir, "repos"), logger)
	ctx := context.Background()
	cloneURL := "file://" + bareRepo

	if _, err := cache.EnsureRepo(ctx, cloneURL, nil); err != nil {
		t.Fatal(err)
	}

	// Get a real commit SHA from the repo to use in MR metadata.
	commits, err := cache.ListCommits(ctx, cloneURL, forge.ListCommitsOpts{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) == 0 {
		t.Fatal("no commits in test repo")
	}
	headSHA := commits[0].SHA

	// Store MR metadata pointing to a real commit by SHA.
	mrs := []dto.MRMetadata{
		{ID: "#1", State: forge.MergeStateOpen, URL: "https://example.com/pr/1", HeadSHA: headSHA},
	}
	if err := cache.StoreMRMetadata(cloneURL, mrs); err != nil {
		t.Fatal(err)
	}

	// ListMRCommits should resolve the commit by SHA.
	mrCommits, err := cache.ListMRCommits(ctx, cloneURL)
	if err != nil {
		t.Fatalf("ListMRCommits() error: %v", err)
	}
	if len(mrCommits) != 1 {
		t.Fatalf("expected 1 MR commit, got %d", len(mrCommits))
	}
	if mrCommits[0].SHA != headSHA {
		t.Errorf("MR commit SHA = %q, want %q", mrCommits[0].SHA, headSHA)
	}
	if mrCommits[0].MergeRequest == nil {
		t.Fatal("MR commit should have MergeRequest annotation")
	}
	if mrCommits[0].MergeRequest.ID != "#1" {
		t.Errorf("MR ID = %q, want %q", mrCommits[0].MergeRequest.ID, "#1")
	}
	if mrCommits[0].MergeRequest.State != forge.MergeStateOpen {
		t.Errorf("MR state = %v, want Open", mrCommits[0].MergeRequest.State)
	}
}

func TestCache_repoPath(t *testing.T) {
	t.Parallel()
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
