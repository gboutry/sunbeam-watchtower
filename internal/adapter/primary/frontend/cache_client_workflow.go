// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// CacheSyncGitRequest describes one git-cache sync workflow.
type CacheSyncGitRequest struct {
	Project string
}

// CacheSyncGitResponse contains the outcome of one git-cache sync.
type CacheSyncGitResponse struct {
	Synced   int
	Warnings []string
}

// CacheSyncPackagesIndexRequest describes one packages-index sync workflow.
type CacheSyncPackagesIndexRequest struct {
	Distros   []string
	Releases  []string
	Backports []string
}

// CacheSyncUpstreamResponse contains the outcome of one upstream-cache sync.
type CacheSyncUpstreamResponse struct {
	Status string
}

// CacheSyncBugsRequest describes one bug-cache sync workflow.
type CacheSyncBugsRequest struct {
	Project string
}

// CacheSyncBugsResponse contains the outcome of one bug-cache sync.
type CacheSyncBugsResponse struct {
	Synced int
}

// CacheSyncExcusesRequest describes one excuses-cache sync workflow.
type CacheSyncExcusesRequest struct {
	Trackers []string
}

// CacheSyncExcusesResponse contains the outcome of one excuses-cache sync.
type CacheSyncExcusesResponse struct {
	Status string
}

// CacheSyncReleasesResponse contains the outcome of one releases-cache sync.
type CacheSyncReleasesResponse struct {
	Status string
}

// CacheClearRequest describes one cache-clear workflow.
type CacheClearRequest struct {
	Type     string
	Project  string
	Trackers []string
}

// CacheEntry describes one named cache entry.
type CacheEntry struct {
	Name string
	Size string
}

// CacheStatusResponse contains the full cache status snapshot.
type CacheStatusResponse struct {
	Git struct {
		Directory string
		Repos     []CacheEntry
	}
	Packages struct {
		Directory string
		Sources   []dto.CacheStatus
		Error     string
	}
	Upstream struct {
		Directory string
		Repos     []CacheEntry
	}
	Bugs struct {
		Directory string
		Entries   []dto.BugCacheStatus
		Error     string
	}
	Excuses struct {
		Directory string
		Entries   []dto.ExcusesCacheStatus
		Error     string
	}
	Releases struct {
		Directory string
		Entries   []dto.ReleaseCacheStatus
		Error     string
	}
}

// CacheClientWorkflow exposes reusable client-side cache workflows for CLI/TUI/MCP frontends.
type CacheClientWorkflow struct {
	client *ClientTransport
}

// NewCacheClientWorkflow creates a client-side cache workflow.
func NewCacheClientWorkflow(apiClient *ClientTransport) *CacheClientWorkflow {
	return &CacheClientWorkflow{client: apiClient}
}

// SyncGit syncs git caches for configured projects.
func (w *CacheClientWorkflow) SyncGit(ctx context.Context, req CacheSyncGitRequest) (*CacheSyncGitResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	result, err := apiClient.CacheSyncGit(ctx, client.CacheSyncGitOptions{Project: req.Project})
	if err != nil {
		return nil, err
	}
	return &CacheSyncGitResponse{
		Synced:   result.Synced,
		Warnings: result.Warnings,
	}, nil
}

// SyncPackagesIndex syncs package index caches.
func (w *CacheClientWorkflow) SyncPackagesIndex(ctx context.Context, req CacheSyncPackagesIndexRequest) error {
	apiClient, err := w.resolveClient()
	if err != nil {
		return err
	}
	return apiClient.PackagesCacheSync(ctx, client.PackagesCacheSyncOptions{
		Distros:   req.Distros,
		Releases:  req.Releases,
		Backports: req.Backports,
	})
}

// SyncUpstream syncs upstream repository caches.
func (w *CacheClientWorkflow) SyncUpstream(ctx context.Context) (*CacheSyncUpstreamResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	result, err := apiClient.CacheSyncUpstream(ctx)
	if err != nil {
		return nil, err
	}
	return &CacheSyncUpstreamResponse{Status: result.Status}, nil
}

// SyncBugs syncs bug caches for configured projects.
func (w *CacheClientWorkflow) SyncBugs(ctx context.Context, req CacheSyncBugsRequest) (*CacheSyncBugsResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	result, err := apiClient.CacheSyncBugs(ctx, client.CacheSyncBugsOptions{Project: req.Project})
	if err != nil {
		return nil, err
	}
	return &CacheSyncBugsResponse{Synced: result.Synced}, nil
}

// SyncExcuses syncs excuses caches for the requested trackers.
func (w *CacheClientWorkflow) SyncExcuses(ctx context.Context, req CacheSyncExcusesRequest) (*CacheSyncExcusesResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	result, err := apiClient.CacheSyncExcuses(ctx, client.CacheSyncExcusesOptions{Trackers: req.Trackers})
	if err != nil {
		return nil, err
	}
	return &CacheSyncExcusesResponse{Status: result.Status}, nil
}

// SyncReleases syncs cached published release state.
func (w *CacheClientWorkflow) SyncReleases(ctx context.Context) (*CacheSyncReleasesResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	result, err := apiClient.CacheSyncReleases(ctx)
	if err != nil {
		return nil, err
	}
	return &CacheSyncReleasesResponse{Status: result.Status}, nil
}

// Clear clears one cache type.
func (w *CacheClientWorkflow) Clear(ctx context.Context, req CacheClearRequest) error {
	apiClient, err := w.resolveClient()
	if err != nil {
		return err
	}
	if len(req.Trackers) > 0 {
		return apiClient.CacheDeleteWithTrackers(ctx, req.Type, req.Project, req.Trackers)
	}
	return apiClient.CacheDelete(ctx, req.Type, req.Project)
}

// Status returns the full cache status snapshot.
func (w *CacheClientWorkflow) Status(ctx context.Context) (*CacheStatusResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	result, err := apiClient.CacheStatus(ctx)
	if err != nil {
		return nil, err
	}

	response := &CacheStatusResponse{}
	response.Git.Directory = result.Git.Directory
	for _, repo := range result.Git.Repos {
		response.Git.Repos = append(response.Git.Repos, CacheEntry{Name: repo.Name, Size: repo.Size})
	}
	response.Packages.Directory = result.Packages.Directory
	response.Packages.Sources = append(response.Packages.Sources, result.Packages.Sources...)
	response.Packages.Error = result.Packages.Error
	response.Upstream.Directory = result.Upstream.Directory
	for _, repo := range result.Upstream.Repos {
		response.Upstream.Repos = append(response.Upstream.Repos, CacheEntry{Name: repo.Name, Size: repo.Size})
	}
	response.Bugs.Directory = result.Bugs.Directory
	response.Bugs.Entries = append(response.Bugs.Entries, result.Bugs.Entries...)
	response.Bugs.Error = result.Bugs.Error
	response.Excuses.Directory = result.Excuses.Directory
	response.Excuses.Entries = append(response.Excuses.Entries, result.Excuses.Entries...)
	response.Excuses.Error = result.Excuses.Error
	response.Releases.Directory = result.Releases.Directory
	response.Releases.Entries = append(response.Releases.Entries, result.Releases.Entries...)
	response.Releases.Error = result.Releases.Error
	return response, nil
}

func (w *CacheClientWorkflow) resolveClient() (*ClientTransport, error) {
	if w.client == nil {
		return nil, errors.New("cache client workflow requires an API client")
	}
	return w.client, nil
}
