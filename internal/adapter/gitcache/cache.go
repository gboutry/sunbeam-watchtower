// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package gitcache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

const mrMetadataFile = ".watchtower-mrs.json"

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
func (c *Cache) EnsureRepo(ctx context.Context, cloneURL string, opts *port.SyncOptions) (string, error) {
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err == nil {
		// Repo exists, fetch.
		c.logger.Debug("fetching existing cached repo", "url", cloneURL, "path", path)
		if fetchErr := c.fetchRepo(ctx, path, opts); fetchErr != nil {
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

	// After initial clone, fetch extra refspecs if provided.
	if opts != nil && len(opts.ExtraRefSpecs) > 0 {
		c.logger.Debug("fetching extra refspecs after clone", "url", cloneURL, "refspecs", opts.ExtraRefSpecs)
		if fetchErr := c.fetchRepo(ctx, path, opts); fetchErr != nil {
			c.logger.Warn("extra refspec fetch failed", "url", cloneURL, "error", fetchErr)
		}
	}

	return path, nil
}

// Fetch updates an existing cached repository from origin.
func (c *Cache) Fetch(ctx context.Context, cloneURL string, opts *port.SyncOptions) error {
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return err
	}
	return c.fetchRepo(ctx, path, opts)
}

func (c *Cache) fetchRepo(ctx context.Context, path string, opts *port.SyncOptions) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("opening repo at %s: %w", path, err)
	}

	// Default fetch (branches).
	err = repo.FetchContext(ctx, &git.FetchOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("fetching: %w", err)
	}

	// Fetch extra refspecs if provided.
	if opts != nil && len(opts.ExtraRefSpecs) > 0 {
		refSpecs := make([]gitconfig.RefSpec, len(opts.ExtraRefSpecs))
		for i, rs := range opts.ExtraRefSpecs {
			refSpecs[i] = gitconfig.RefSpec(rs)
		}
		c.logger.Debug("fetching extra refspecs", "path", path, "refspecs", opts.ExtraRefSpecs)
		err = repo.FetchContext(ctx, &git.FetchOptions{
			RefSpecs: refSpecs,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			c.logger.Warn("extra refspec fetch failed", "path", path, "error", err)
		}
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

// StoreMRMetadata writes merge request metadata as a sidecar JSON file.
func (c *Cache) StoreMRMetadata(cloneURL string, mrs []port.MRMetadata) error {
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return err
	}

	metaPath := filepath.Join(path, mrMetadataFile)
	c.logger.Debug("storing MR metadata", "cloneURL", cloneURL, "count", len(mrs), "path", metaPath)

	data, err := json.MarshalIndent(mrs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling MR metadata: %w", err)
	}

	return os.WriteFile(metaPath, data, 0o644)
}

// LoadMRMetadata reads merge request metadata from the sidecar JSON file.
func (c *Cache) LoadMRMetadata(cloneURL string) ([]port.MRMetadata, error) {
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return nil, err
	}

	metaPath := filepath.Join(path, mrMetadataFile)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading MR metadata: %w", err)
	}

	var mrs []port.MRMetadata
	if err := json.Unmarshal(data, &mrs); err != nil {
		return nil, fmt.Errorf("parsing MR metadata: %w", err)
	}

	c.logger.Debug("loaded MR metadata", "cloneURL", cloneURL, "count", len(mrs))
	return mrs, nil
}

// ListMRCommits reads the head commit for each cached merge request ref.
func (c *Cache) ListMRCommits(ctx context.Context, cloneURL string) ([]forge.Commit, error) {
	c.logger.Debug("listing MR commits", "cloneURL", cloneURL)

	mrs, err := c.LoadMRMetadata(cloneURL)
	if err != nil {
		return nil, err
	}
	if len(mrs) == 0 {
		return nil, nil
	}

	path, err := c.repoPath(cloneURL)
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("opening repo at %s: %w", path, err)
	}

	var result []forge.Commit
	for _, mr := range mrs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if mr.GitRef == "" {
			// Try resolving by HeadSHA directly.
			if mr.HeadSHA == "" {
				continue
			}
			co, err := repo.CommitObject(plumbing.NewHash(mr.HeadSHA))
			if err != nil {
				c.logger.Debug("MR commit not found by SHA", "mr_id", mr.ID, "sha", mr.HeadSHA, "error", err)
				continue
			}
			result = append(result, commitFromObject(co, &mr))
			continue
		}

		ref, err := repo.Reference(plumbing.ReferenceName(mr.GitRef), true)
		if err != nil {
			c.logger.Debug("MR ref not found", "mr_id", mr.ID, "ref", mr.GitRef, "error", err)
			continue
		}

		co, err := repo.CommitObject(ref.Hash())
		if err != nil {
			c.logger.Debug("MR commit not found", "mr_id", mr.ID, "ref", mr.GitRef, "error", err)
			continue
		}

		result = append(result, commitFromObject(co, &mr))
	}

	c.logger.Debug("MR commits read", "cloneURL", cloneURL, "count", len(result))
	return result, nil
}

func commitFromObject(co *object.Commit, mr *port.MRMetadata) forge.Commit {
	return forge.Commit{
		SHA:     co.Hash.String(),
		Message: co.Message,
		Author:  co.Author.Name,
		Date:    co.Author.When,
		BugRefs: forge.ExtractBugRefs(co.Message),
		MergeRequest: &forge.CommitMergeRequest{
			ID:    mr.ID,
			State: mr.State,
			URL:   mr.URL,
		},
	}
}

// ListBranches returns the branch names available in a cached repository.
func (c *Cache) ListBranches(_ context.Context, cloneURL string) ([]string, error) {
	c.logger.Debug("listing branches from cache", "cloneURL", cloneURL)
	path, err := c.repoPath(cloneURL)
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("opening repo at %s: %w", path, err)
	}

	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("listing references: %w", err)
	}

	const prefix = "refs/remotes/origin/"
	var branches []string
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if strings.HasPrefix(name, prefix) {
			branch := strings.TrimPrefix(name, prefix)
			if branch != "HEAD" {
				branches = append(branches, branch)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterating references: %w", err)
	}

	c.logger.Debug("branches found", "cloneURL", cloneURL, "count", len(branches))
	return branches, nil
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
