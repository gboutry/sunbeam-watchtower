package bug

import (
	"context"
	"fmt"
	"testing"
	"time"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

type mockBugTracker struct {
	forgeType forge.ForgeType
	tasks     []forge.BugTask
	bug       *forge.Bug
	err       error
}

func (m *mockBugTracker) Type() forge.ForgeType { return m.forgeType }

func (m *mockBugTracker) GetBug(_ context.Context, id string) (*forge.Bug, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.bug != nil {
		return m.bug, nil
	}
	return nil, fmt.Errorf("bug %s not found", id)
}

func (m *mockBugTracker) ListBugTasks(_ context.Context, _ string, _ forge.ListBugTasksOpts) ([]forge.BugTask, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tasks, nil
}

func TestService_List_Aggregation(t *testing.T) {
	now := time.Now()

	tracker := &mockBugTracker{
		forgeType: forge.ForgeLaunchpad,
		tasks: []forge.BugTask{
			{BugID: "1", Title: "Bug 1", UpdatedAt: now.Add(-1 * time.Hour)},
			{BugID: "2", Title: "Bug 2", UpdatedAt: now.Add(-3 * time.Hour)},
		},
	}

	svc := NewService(
		map[string]ProjectBugTracker{
			"launchpad:snap-openstack": {Tracker: tracker, ProjectID: "snap-openstack"},
		},
		map[string][]string{
			"launchpad:snap-openstack": {"sunbeam", "microstack"},
		},
	)

	tasks, results, err := svc.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// 2 bugs × 2 projects = 4 tasks
	if len(tasks) != 4 {
		t.Fatalf("len(tasks) = %d, want 4", len(tasks))
	}

	// Should be sorted by UpdatedAt descending.
	for i := 1; i < len(tasks); i++ {
		if tasks[i].UpdatedAt.After(tasks[i-1].UpdatedAt) {
			t.Errorf("tasks[%d].UpdatedAt (%v) > tasks[%d].UpdatedAt (%v)", i, tasks[i].UpdatedAt, i-1, tasks[i-1].UpdatedAt)
		}
	}

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", r.ProjectName, r.Err)
		}
	}
}

func TestService_List_FilterByProject(t *testing.T) {
	tracker := &mockBugTracker{
		forgeType: forge.ForgeLaunchpad,
		tasks:     []forge.BugTask{{BugID: "1", Title: "Bug 1"}},
	}

	svc := NewService(
		map[string]ProjectBugTracker{
			"launchpad:snap-openstack": {Tracker: tracker, ProjectID: "snap-openstack"},
		},
		map[string][]string{
			"launchpad:snap-openstack": {"sunbeam", "microstack"},
		},
	)

	tasks, _, _ := svc.List(context.Background(), ListOptions{Projects: []string{"sunbeam"}})
	if len(tasks) != 1 {
		t.Errorf("expected 1 task for sunbeam, got %d", len(tasks))
	}
	if len(tasks) > 0 && tasks[0].Project != "sunbeam" {
		t.Errorf("expected project=sunbeam, got %s", tasks[0].Project)
	}
}

func TestService_List_Deduplication(t *testing.T) {
	callCount := 0
	tracker := &mockBugTracker{
		forgeType: forge.ForgeLaunchpad,
		tasks:     []forge.BugTask{{BugID: "1", Title: "Bug"}},
	}
	// Wrap to count calls
	countingTracker := &countingBugTracker{inner: tracker, count: &callCount}

	svc := NewService(
		map[string]ProjectBugTracker{
			"launchpad:snap-openstack": {Tracker: countingTracker, ProjectID: "snap-openstack"},
		},
		map[string][]string{
			"launchpad:snap-openstack": {"sunbeam", "microstack"},
		},
	)

	_, _, _ = svc.List(context.Background(), ListOptions{})
	if callCount != 1 {
		t.Errorf("expected 1 API call (deduplication), got %d", callCount)
	}
}

func TestService_List_GracefulDegradation(t *testing.T) {
	goodTracker := &mockBugTracker{
		forgeType: forge.ForgeLaunchpad,
		tasks:     []forge.BugTask{{BugID: "1", Title: "Good Bug"}},
	}
	badTracker := &mockBugTracker{
		forgeType: forge.ForgeLaunchpad,
		err:       fmt.Errorf("connection refused"),
	}

	svc := NewService(
		map[string]ProjectBugTracker{
			"launchpad:good-project": {Tracker: goodTracker, ProjectID: "good-project"},
			"launchpad:bad-project":  {Tracker: badTracker, ProjectID: "bad-project"},
		},
		map[string][]string{
			"launchpad:good-project": {"good"},
			"launchpad:bad-project":  {"bad"},
		},
	)

	tasks, results, err := svc.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() should not return top-level error: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 task from good project, got %d", len(tasks))
	}

	var hadError bool
	for _, r := range results {
		if r.Err != nil {
			hadError = true
		}
	}
	if !hadError {
		t.Error("expected at least one project result with error")
	}
}

// countingBugTracker wraps a BugTracker and counts ListBugTasks calls.
type countingBugTracker struct {
	inner forge.BugTracker
	count *int
}

func (c *countingBugTracker) Type() forge.ForgeType { return c.inner.Type() }

func (c *countingBugTracker) GetBug(ctx context.Context, id string) (*forge.Bug, error) {
	return c.inner.GetBug(ctx, id)
}

func (c *countingBugTracker) ListBugTasks(ctx context.Context, project string, opts forge.ListBugTasksOpts) ([]forge.BugTask, error) {
	*c.count++
	return c.inner.ListBugTasks(ctx, project, opts)
}

func TestService_Get(t *testing.T) {
	tracker := &mockBugTracker{
		forgeType: forge.ForgeLaunchpad,
		bug: &forge.Bug{
			ID:    "12345",
			Title: "Test bug",
			Tags:  []string{"sunbeam"},
			Tasks: []forge.BugTask{
				{BugID: "12345", Status: "New", Importance: "High"},
				{BugID: "12345", Status: "Confirmed", Importance: "Medium"},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBugTracker{
			"launchpad:snap-openstack": {Tracker: tracker, ProjectID: "snap-openstack"},
		},
		map[string][]string{
			"launchpad:snap-openstack": {"sunbeam"},
		},
	)

	bug, err := svc.Get(context.Background(), "12345")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if bug.ID != "12345" {
		t.Errorf("ID = %q, want 12345", bug.ID)
	}
	if len(bug.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(bug.Tasks))
	}
}

func TestService_Get_NotConfigured(t *testing.T) {
	svc := NewService(
		map[string]ProjectBugTracker{},
		map[string][]string{},
	)

	_, err := svc.Get(context.Background(), "12345")
	if err == nil {
		t.Fatal("expected error when no trackers configured")
	}
}
