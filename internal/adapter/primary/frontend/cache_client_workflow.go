// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
)

// CacheClientWorkflow exposes reusable client-side cache workflows for CLI/TUI/MCP frontends.
type CacheClientWorkflow struct {
	client *client.Client
}

// NewCacheClientWorkflow creates a client-side cache workflow.
func NewCacheClientWorkflow(apiClient *client.Client) *CacheClientWorkflow {
	return &CacheClientWorkflow{client: apiClient}
}

// SyncGit syncs git caches for configured projects.
func (w *CacheClientWorkflow) SyncGit(ctx context.Context, project string) (*client.CacheSyncGitResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.CacheSyncGit(ctx, client.CacheSyncGitOptions{Project: project})
}

// SyncPackagesIndex syncs package index caches.
func (w *CacheClientWorkflow) SyncPackagesIndex(ctx context.Context, distros, releases, backports []string) error {
	apiClient, err := w.resolveClient()
	if err != nil {
		return err
	}
	return apiClient.PackagesCacheSync(ctx, client.PackagesCacheSyncOptions{
		Distros:   distros,
		Releases:  releases,
		Backports: backports,
	})
}

// SyncUpstream syncs upstream repository caches.
func (w *CacheClientWorkflow) SyncUpstream(ctx context.Context) (*client.CacheSyncUpstreamResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.CacheSyncUpstream(ctx)
}

// SyncBugs syncs bug caches for configured projects.
func (w *CacheClientWorkflow) SyncBugs(ctx context.Context, project string) (*client.CacheSyncBugsResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.CacheSyncBugs(ctx, client.CacheSyncBugsOptions{Project: project})
}

// SyncExcuses syncs excuses caches for the requested trackers.
func (w *CacheClientWorkflow) SyncExcuses(ctx context.Context, trackers []string) (*client.CacheSyncExcusesResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.CacheSyncExcuses(ctx, client.CacheSyncExcusesOptions{Trackers: trackers})
}

// Clear clears one cache type.
func (w *CacheClientWorkflow) Clear(ctx context.Context, cacheType, project string) error {
	apiClient, err := w.resolveClient()
	if err != nil {
		return err
	}
	return apiClient.CacheDelete(ctx, cacheType, project)
}

// ClearExcuses clears excuses cache entries for the requested trackers.
func (w *CacheClientWorkflow) ClearExcuses(ctx context.Context, trackers []string) error {
	apiClient, err := w.resolveClient()
	if err != nil {
		return err
	}
	return apiClient.CacheDeleteWithTrackers(ctx, "excuses", "", trackers)
}

// Status returns the full cache status snapshot.
func (w *CacheClientWorkflow) Status(ctx context.Context) (*client.CacheStatusResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.CacheStatus(ctx)
}

func (w *CacheClientWorkflow) resolveClient() (*client.Client, error) {
	if w.client == nil {
		return nil, errors.New("cache client workflow requires an API client")
	}
	return w.client, nil
}
