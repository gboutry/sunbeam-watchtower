// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/url"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// CacheSyncGitOptions holds the request body for syncing git caches.
type CacheSyncGitOptions struct {
	Project string `json:"project"`
}

// CacheSyncGitResult is the response returned by CacheSyncGit.
type CacheSyncGitResult struct {
	Synced   int      `json:"synced"`
	Warnings []string `json:"warnings"`
}

// CacheSyncGit syncs git caches for configured projects.
func (c *Client) CacheSyncGit(ctx context.Context, opts CacheSyncGitOptions) (*CacheSyncGitResult, error) {
	var result CacheSyncGitResult
	err := c.post(ctx, "/api/v1/cache/sync/git", opts, &result)
	return &result, err
}

// CacheSyncUpstreamResult is the response returned by CacheSyncUpstream.
type CacheSyncUpstreamResult struct {
	Status string `json:"status"`
}

// CacheSyncUpstream syncs upstream repos (releases, requirements).
func (c *Client) CacheSyncUpstream(ctx context.Context) (*CacheSyncUpstreamResult, error) {
	var result CacheSyncUpstreamResult
	err := c.post(ctx, "/api/v1/cache/sync/upstream", nil, &result)
	return &result, err
}

// CacheSyncBugsOptions holds the request body for syncing bug caches.
type CacheSyncBugsOptions struct {
	Project string `json:"project"`
}

// CacheSyncBugsResult is the response returned by CacheSyncBugs.
type CacheSyncBugsResult struct {
	Synced int `json:"synced"`
}

// CacheSyncBugs syncs bug caches for configured projects.
func (c *Client) CacheSyncBugs(ctx context.Context, opts CacheSyncBugsOptions) (*CacheSyncBugsResult, error) {
	var result CacheSyncBugsResult
	err := c.post(ctx, "/api/v1/cache/sync/bugs", opts, &result)
	return &result, err
}

// CacheDelete clears a specific cache type.
func (c *Client) CacheDelete(ctx context.Context, cacheType string, project string) error {
	q := url.Values{}
	if project != "" {
		q.Set("project", project)
	}
	return c.delete(ctx, "/api/v1/cache/"+url.PathEscape(cacheType), q, nil)
}

// CacheEntry describes a single cached directory entry.
type CacheEntry struct {
	Name string `json:"name"`
	Size string `json:"size"`
}

// CacheStatusResult is the response returned by CacheStatus.
type CacheStatusResult struct {
	Git struct {
		Directory string       `json:"directory"`
		Repos     []CacheEntry `json:"repos"`
	} `json:"git"`
	Packages struct {
		Directory string            `json:"directory"`
		Sources   []dto.CacheStatus `json:"sources"`
		Error     string            `json:"error,omitempty"`
	} `json:"packages"`
	Upstream struct {
		Directory string       `json:"directory"`
		Repos     []CacheEntry `json:"repos"`
	} `json:"upstream"`
	Bugs struct {
		Directory string               `json:"directory"`
		Entries   []dto.BugCacheStatus `json:"entries"`
		Error     string               `json:"error,omitempty"`
	} `json:"bugs"`
}

// CacheStatus returns the full cache status (git + packages + upstream).
func (c *Client) CacheStatus(ctx context.Context) (*CacheStatusResult, error) {
	var result CacheStatusResult
	err := c.get(ctx, "/api/v1/cache/status", nil, &result)
	return &result, err
}
