// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package bugcache_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/bugcache"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// mockBugTracker is a simple in-memory implementation for testing.
type mockBugTracker struct {
	bugs  map[string]*forge.Bug
	tasks map[string][]forge.BugTask // keyed by project
}

func newMockBugTracker() *mockBugTracker {
	return &mockBugTracker{
		bugs:  make(map[string]*forge.Bug),
		tasks: make(map[string][]forge.BugTask),
	}
}

func (m *mockBugTracker) Type() forge.ForgeType { return forge.ForgeLaunchpad }

func (m *mockBugTracker) GetBug(_ context.Context, id string) (*forge.Bug, error) {
	b, ok := m.bugs[id]
	if !ok {
		return nil, fmt.Errorf("bug %s not found", id)
	}
	return b, nil
}

func (m *mockBugTracker) ListBugTasks(_ context.Context, project string, _ forge.ListBugTasksOpts) ([]forge.BugTask, error) {
	return m.tasks[project], nil
}

func (m *mockBugTracker) UpdateBugTaskStatus(_ context.Context, selfLink, status string) error {
	for proj := range m.tasks {
		for i := range m.tasks[proj] {
			if m.tasks[proj][i].SelfLink == selfLink {
				m.tasks[proj][i].Status = status
				return nil
			}
		}
	}
	return nil
}

func (m *mockBugTracker) AddBugTask(_ context.Context, _ int, _ string) error { return nil }

func (m *mockBugTracker) GetProjectSeries(_ context.Context, _ string) ([]forge.ProjectSeries, error) {
	return nil, nil
}

func (m *mockBugTracker) GetProject(_ context.Context, _ string) (*forge.Project, error) {
	return nil, nil
}

func newTestCachedTracker(t *testing.T, mock *mockBugTracker, project string) *bugcache.CachedBugTracker {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bugs")
	cache, err := bugcache.NewCache(dir, nil)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	return bugcache.NewCachedBugTracker(mock, cache, project, nil)
}

func TestCachedTrackerFallsBackWhenNotSynced(t *testing.T) {
	mock := newMockBugTracker()
	mock.tasks["myproject"] = []forge.BugTask{
		{Forge: forge.ForgeLaunchpad, BugID: "1", TargetName: "myproject", Status: "New", SelfLink: "/task/1"},
	}

	ct := newTestCachedTracker(t, mock, "myproject")
	ctx := context.Background()

	// Before sync, should fall back to mock's live data.
	tasks, err := ct.ListBugTasks(ctx, "myproject", forge.ListBugTasksOpts{})
	if err != nil {
		t.Fatalf("ListBugTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].BugID != "1" {
		t.Errorf("expected mock data before sync, got %+v", tasks)
	}
}

func TestCachedTrackerServesFromCacheAfterSync(t *testing.T) {
	mock := newMockBugTracker()
	mock.bugs["1"] = &forge.Bug{Forge: forge.ForgeLaunchpad, ID: "1", Title: "Bug 1"}
	mock.tasks["proj"] = []forge.BugTask{
		{Forge: forge.ForgeLaunchpad, BugID: "1", TargetName: "proj", Status: "New", SelfLink: "/task/1"},
	}

	ct := newTestCachedTracker(t, mock, "proj")
	ctx := context.Background()

	synced, err := ct.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if synced != 1 {
		t.Errorf("expected 1 task synced, got %d", synced)
	}

	// After sync, should serve from cache.
	tasks, err := ct.ListBugTasks(ctx, "proj", forge.ListBugTasksOpts{})
	if err != nil {
		t.Fatalf("ListBugTasks after sync: %v", err)
	}
	if len(tasks) != 1 || tasks[0].BugID != "1" {
		t.Errorf("expected cached data, got %+v", tasks)
	}

	// Clear mock data; cached tracker should still return cached data.
	mock.tasks["proj"] = nil
	tasks, err = ct.ListBugTasks(ctx, "proj", forge.ListBugTasksOpts{})
	if err != nil {
		t.Fatalf("ListBugTasks after clearing mock: %v", err)
	}
	if len(tasks) != 1 || tasks[0].BugID != "1" {
		t.Errorf("expected cached data after mock cleared, got %+v", tasks)
	}
}

func TestCachedTrackerWriteThrough(t *testing.T) {
	mock := newMockBugTracker()
	mock.bugs["5"] = &forge.Bug{Forge: forge.ForgeLaunchpad, ID: "5", Title: "Bug 5"}
	mock.tasks["proj"] = []forge.BugTask{
		{Forge: forge.ForgeLaunchpad, BugID: "5", TargetName: "proj", Status: "New", SelfLink: "/task/5"},
	}

	ct := newTestCachedTracker(t, mock, "proj")
	ctx := context.Background()

	if _, err := ct.Sync(ctx); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Update task status through the cached tracker.
	if err := ct.UpdateBugTaskStatus(ctx, "/task/5", "Fix Committed"); err != nil {
		t.Fatalf("UpdateBugTaskStatus: %v", err)
	}

	// Verify the cached task reflects the new status.
	tasks, err := ct.ListBugTasks(ctx, "proj", forge.ListBugTasksOpts{})
	if err != nil {
		t.Fatalf("ListBugTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Status != "Fix Committed" {
		t.Errorf("expected status 'Fix Committed', got %q", tasks[0].Status)
	}
}

func TestCachedTrackerType(t *testing.T) {
	mock := newMockBugTracker()
	ct := newTestCachedTracker(t, mock, "proj")
	if ct.Type() != forge.ForgeLaunchpad {
		t.Errorf("Type: got %v, want ForgeLaunchpad", ct.Type())
	}
}

func TestCachedTrackerPassThrough(t *testing.T) {
	mock := newMockBugTracker()
	ct := newTestCachedTracker(t, mock, "proj")
	ctx := context.Background()

	// GetProjectSeries should delegate to inner tracker.
	series, err := ct.GetProjectSeries(ctx, "proj")
	if err != nil {
		t.Fatalf("GetProjectSeries: %v", err)
	}
	if series != nil {
		t.Errorf("expected nil series from mock, got %+v", series)
	}

	// GetProject should delegate to inner tracker.
	proj, err := ct.GetProject(ctx, "proj")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj != nil {
		t.Errorf("expected nil project from mock, got %+v", proj)
	}
}
