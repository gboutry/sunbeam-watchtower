// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package bugcache

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// CachedBugTracker decorates a port.BugTracker with local cache support.
// Read operations serve from cache when populated, falling back to the inner
// tracker. Write operations delegate to the inner tracker and update the cache.
type CachedBugTracker struct {
	inner   port.BugTracker
	cache   port.BugCache
	project string
	logger  *slog.Logger
}

const incrementalModifiedOverlap = 24 * time.Hour
const defaultBugFetchConcurrency = 4

// NewCachedBugTracker wraps a BugTracker with caching support.
func NewCachedBugTracker(inner port.BugTracker, cache port.BugCache, project string, logger *slog.Logger) *CachedBugTracker {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &CachedBugTracker{inner: inner, cache: cache, project: project, logger: logger}
}

func (c *CachedBugTracker) Type() forge.ForgeType {
	return c.inner.Type()
}

func (c *CachedBugTracker) GetBug(ctx context.Context, id string) (*forge.Bug, error) {
	if c.isSynced(ctx) {
		b, err := c.cache.GetBug(ctx, c.inner.Type(), id)
		if err == nil {
			c.logger.Debug("bug served from cache", "id", id)
			return b, nil
		}
		c.logger.Debug("bug not in cache, falling back to live", "id", id, "error", err)
	}
	return c.inner.GetBug(ctx, id)
}

func (c *CachedBugTracker) ListBugTasks(ctx context.Context, project string, opts forge.ListBugTasksOpts) ([]forge.BugTask, error) {
	if c.isSynced(ctx) {
		tasks, err := c.cache.ListBugTasks(ctx, c.inner.Type(), project, opts)
		if err == nil {
			c.logger.Debug("bug tasks served from cache", "project", project, "count", len(tasks))
			return tasks, nil
		}
		c.logger.Debug("cache read failed, falling back to live", "project", project, "error", err)
	}
	return c.inner.ListBugTasks(ctx, project, opts)
}

func (c *CachedBugTracker) UpdateBugTaskStatus(ctx context.Context, taskSelfLink, status string) error {
	if err := c.inner.UpdateBugTaskStatus(ctx, taskSelfLink, status); err != nil {
		return err
	}
	// Write-through: update cached task if the cache is populated.
	if c.isSynced(ctx) {
		c.updateCachedTaskStatus(ctx, taskSelfLink, status)
	}
	return nil
}

func (c *CachedBugTracker) AddBugTask(ctx context.Context, bugID int, seriesSelfLink string) error {
	return c.inner.AddBugTask(ctx, bugID, seriesSelfLink)
}

func (c *CachedBugTracker) GetProjectSeries(ctx context.Context, projectName string) ([]forge.ProjectSeries, error) {
	return c.inner.GetProjectSeries(ctx, projectName)
}

func (c *CachedBugTracker) GetProject(ctx context.Context, projectName string) (*forge.Project, error) {
	return c.inner.GetProject(ctx, projectName)
}

// Sync fetches bugs from the inner tracker and stores them in the cache.
// If the cache was previously synced, only tasks modified since the last sync
// are fetched (incremental sync).
func (c *CachedBugTracker) Sync(ctx context.Context) (synced int, err error) {
	forgeType := c.inner.Type()

	opts := forge.ListBugTasksOpts{}
	lastSync, lsErr := c.cache.LastSync(ctx, forgeType, c.project)
	if lsErr == nil && !lastSync.IsZero() {
		opts.CreatedSince = lastSync.UTC().Format(time.RFC3339)
		modifiedSince := lastSync.Add(-incrementalModifiedOverlap)
		opts.ModifiedSince = modifiedSince.UTC().Format(time.RFC3339)
		c.logger.Debug(
			"incremental bug cache sync",
			"project", c.project,
			"created_since", opts.CreatedSince,
			"modified_since", opts.ModifiedSince,
			"modified_overlap", incrementalModifiedOverlap.String(),
		)
	} else {
		c.logger.Debug("full bug cache sync", "project", c.project)
	}

	incoming, err := c.inner.ListBugTasks(ctx, c.project, opts)
	if err != nil {
		return 0, fmt.Errorf("fetching bug tasks for %s: %w", c.project, err)
	}

	// Only fetch bug details for newly returned tasks, not the entire cache.
	bugIDs := uniqueBugIDs(incoming)

	// For incremental sync, merge new tasks with existing cached tasks.
	tasks := incoming
	if opts.ModifiedSince != "" || opts.CreatedSince != "" {
		existing, _ := c.cache.ListBugTasks(ctx, forgeType, c.project, forge.ListBugTasksOpts{})
		tasks = mergeTasks(existing, incoming)
	}

	if err := c.cache.StoreBugTasks(ctx, forgeType, c.project, tasks); err != nil {
		return 0, fmt.Errorf("storing tasks for %s: %w", c.project, err)
	}
	bugs := c.fetchBugs(ctx, bugIDs)

	if len(bugs) > 0 {
		if err := c.cache.StoreBugs(ctx, bugs); err != nil {
			return 0, fmt.Errorf("storing bugs for %s: %w", c.project, err)
		}
	}

	if err := c.cache.SetLastSync(ctx, forgeType, c.project, time.Now()); err != nil {
		return 0, fmt.Errorf("recording last sync for %s: %w", c.project, err)
	}

	c.logger.Debug("bug cache sync complete", "project", c.project, "tasks", len(tasks), "bugs", len(bugs))
	return len(tasks), nil
}

// Project returns the project ID this cached tracker operates on.
func (c *CachedBugTracker) Project() string {
	return c.project
}

func (c *CachedBugTracker) isSynced(ctx context.Context) bool {
	t, err := c.cache.LastSync(ctx, c.inner.Type(), c.project)
	return err == nil && !t.IsZero()
}

// updateCachedTaskStatus finds a task by self link in the cache and updates its status.
func (c *CachedBugTracker) updateCachedTaskStatus(ctx context.Context, selfLink, newStatus string) {
	forgeType := c.inner.Type()
	tasks, err := c.cache.ListBugTasks(ctx, forgeType, c.project, forge.ListBugTasksOpts{})
	if err != nil {
		return
	}
	for i := range tasks {
		if tasks[i].SelfLink == selfLink {
			tasks[i].Status = newStatus
			cacheImpl, ok := c.cache.(*Cache)
			if ok {
				if uErr := cacheImpl.UpdateTask(ctx, forgeType, &tasks[i]); uErr != nil {
					c.logger.Warn("failed to update cached task status", "selfLink", selfLink, "error", uErr)
				}
			}
			return
		}
	}
}

// uniqueBugIDs returns deduplicated bug IDs from a set of tasks.
func uniqueBugIDs(tasks []forge.BugTask) []string {
	seen := make(map[string]struct{}, len(tasks))
	var ids []string
	for _, t := range tasks {
		if _, ok := seen[t.BugID]; !ok {
			seen[t.BugID] = struct{}{}
			ids = append(ids, t.BugID)
		}
	}
	return ids
}

// mergeTasks merges newly fetched tasks into the existing cached task list.
// New tasks replace existing tasks with the same key (BugID:TargetName).
func mergeTasks(existing, incoming []forge.BugTask) []forge.BugTask {
	merged := make(map[string]forge.BugTask, len(existing)+len(incoming))
	for _, t := range existing {
		merged[t.BugID+":"+t.TargetName] = t
	}
	for _, t := range incoming {
		merged[t.BugID+":"+t.TargetName] = t
	}
	result := make([]forge.BugTask, 0, len(merged))
	for _, t := range merged {
		result = append(result, t)
	}
	return result
}

func (c *CachedBugTracker) fetchBugs(ctx context.Context, bugIDs []string) []*forge.Bug {
	if len(bugIDs) == 0 {
		return nil
	}

	type fetchResult struct {
		index int
		bug   *forge.Bug
	}

	workerCount := min(defaultBugFetchConcurrency, len(bugIDs))
	jobs := make(chan int)
	results := make(chan fetchResult, len(bugIDs))

	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					return
				}
				id := bugIDs[idx]
				b, err := c.inner.GetBug(ctx, id)
				if err != nil {
					c.logger.Warn("failed to fetch bug details", "id", id, "error", err)
					continue
				}
				results <- fetchResult{index: idx, bug: b}
			}
		}()
	}

	for idx := range bugIDs {
		if ctx.Err() != nil {
			break
		}
		jobs <- idx
	}
	close(jobs)
	wg.Wait()
	close(results)

	ordered := make([]*forge.Bug, len(bugIDs))
	for result := range results {
		ordered[result.index] = result.bug
	}

	bugs := make([]*forge.Bug, 0, len(bugIDs))
	for _, bug := range ordered {
		if bug != nil {
			bugs = append(bugs, bug)
		}
	}
	return bugs
}
