// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package gitcache

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

// Cache implements port.GitRepoCache using local bare git clones.
type Cache struct {
	baseDir string
	logger  *slog.Logger
}

// NewCache creates a new git cache rooted at baseDir. If logger is nil, a no-op logger is used.
func NewCache(baseDir string, logger *slog.Logger) *Cache {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Cache{baseDir: baseDir, logger: logger}
}

// CacheDir returns the base directory for cached repos.
func (c *Cache) CacheDir() string {
	return c.baseDir
}

// repoPath converts a clone URL to a local filesystem path under baseDir.
// e.g. "https://github.com/org/repo.git" → "<baseDir>/github.com/org/repo.git"
func (c *Cache) repoPath(cloneURL string) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		return "", fmt.Errorf("parsing clone URL %q: %w", cloneURL, err)
	}

	host := u.Host
	if host == "" {
		if u.Scheme == "file" {
			// file:// URLs use the path directly.
			host = "localhost"
		} else {
			return "", fmt.Errorf("clone URL %q has no host", cloneURL)
		}
	}

	// Strip port if present.
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}

	p := strings.TrimPrefix(u.Path, "/")
	if p == "" {
		return "", fmt.Errorf("clone URL %q has no path", cloneURL)
	}

	// Ensure path ends with .git.
	if !strings.HasSuffix(p, ".git") {
		p += ".git"
	}

	resolved := filepath.Join(c.baseDir, host, p)
	c.logger.Debug("resolved repo path", "cloneURL", cloneURL, "path", resolved)
	return resolved, nil
}

// EnsureRepo clones the repository if missing, or fetches if it already exists.
func (c *Cache) EnsureRepo(ctx context.Context, cloneURL string) (string, error) {
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err == nil {
		// Repo exists, fetch.
		c.logger.Debug("fetching existing cached repo", "url", cloneURL, "path", path)
		if fetchErr := c.fetchRepo(ctx, path); fetchErr != nil {
			c.logger.Warn("fetch failed, repo may be stale", "url", cloneURL, "error", fetchErr)
		}
		return path, nil
	}

	c.logger.Info("cloning repo into cache", "url", cloneURL, "path", path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	_, err = git.PlainCloneContext(ctx, path, true, &git.CloneOptions{
		URL: cloneURL,
	})
	if err != nil {
		return "", fmt.Errorf("cloning %s: %w", cloneURL, err)
	}

	return path, nil
}

// Fetch updates an existing cached repository from origin.
func (c *Cache) Fetch(ctx context.Context, cloneURL string) error {
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return err
	}
	return c.fetchRepo(ctx, path)
}

func (c *Cache) fetchRepo(ctx context.Context, path string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("opening repo at %s: %w", path, err)
	}

	err = repo.FetchContext(ctx, &git.FetchOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("fetching: %w", err)
	}
	return nil
}

// ListCommits reads commit history from a cached repository.
func (c *Cache) ListCommits(ctx context.Context, cloneURL string, opts forge.ListCommitsOpts) ([]forge.Commit, error) {
	c.logger.Debug("listing commits from cache", "cloneURL", cloneURL, "branch", opts.Branch)
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("opening repo at %s: %w", path, err)
	}

	// Resolve the branch to a remote ref.
	branch := opts.Branch
	if branch == "" {
		branch = "main"
	}

	refName := plumbing.NewRemoteReferenceName("origin", branch)
	ref, err := repo.Reference(refName, true)
	if err != nil {
		// Try "master" as fallback if "main" was the default.
		if opts.Branch == "" {
			refName = plumbing.NewRemoteReferenceName("origin", "master")
			ref, err = repo.Reference(refName, true)
		}
		if err != nil {
			return nil, fmt.Errorf("resolving ref %s: %w", refName, err)
		}
	}

	logOpts := &git.LogOptions{
		From:  ref.Hash(),
		Order: git.LogOrderCommitterTime,
	}
	if opts.Since != nil {
		logOpts.Since = opts.Since
	}

	iter, err := repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("reading log: %w", err)
	}
	defer iter.Close()

	var result []forge.Commit
	err = iter.ForEach(func(co *object.Commit) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		author := co.Author.Name
		if opts.Author != "" && author != opts.Author {
			return nil
		}

		result = append(result, forge.Commit{
			SHA:     co.Hash.String(),
			Message: co.Message,
			Author:  author,
			Date:    co.Author.When,
			BugRefs: forge.ExtractBugRefs(co.Message),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	c.logger.Debug("commits read from cache", "cloneURL", cloneURL, "count", len(result))
	return result, nil
}

// Remove deletes a single cached repository.
func (c *Cache) Remove(cloneURL string) error {
	c.logger.Debug("removing cached repo", "cloneURL", cloneURL)
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

// RemoveAll deletes all cached repositories.
func (c *Cache) RemoveAll() error {
	c.logger.Debug("removing all cached repos")
	return os.RemoveAll(c.baseDir)
}
