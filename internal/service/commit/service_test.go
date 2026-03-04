// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package commit

import (
	"context"
	"fmt"
	"testing"
	"time"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

// mockForge implements port.Forge for testing.
type mockForge struct {
	forgeType forge.ForgeType
	commits   []forge.Commit
	err       error
}

func (m *mockForge) Type() forge.ForgeType { return m.forgeType }

func (m *mockForge) ListCommits(_ context.Context, _ string, opts forge.ListCommitsOpts) ([]forge.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []forge.Commit
	for _, c := range m.commits {
		if opts.Author != "" && c.Author != opts.Author {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}

func (m *mockForge) ListMergeRequests(_ context.Context, _ string, _ forge.ListMergeRequestsOpts) ([]forge.MergeRequest, error) {
	return nil, nil
}

func (m *mockForge) GetMergeRequest(_ context.Context, _ string, _ string) (*forge.MergeRequest, error) {
	return nil, nil
}

func TestService_List_Aggregation(t *testing.T) {
	now := time.Now()

	ghForge := &mockForge{
		forgeType: forge.ForgeGitHub,
		commits: []forge.Commit{
			{SHA: "aaa", Author: "alice", Date: now.Add(-1 * time.Hour), Message: "fix: thing"},
			{SHA: "bbb", Author: "bob", Date: now.Add(-3 * time.Hour), Message: "feat: stuff"},
		},
	}
	gerritForge := &mockForge{
		forgeType: forge.ForgeGerrit,
		commits: []forge.Commit{
			{SHA: "ccc", Author: "carol", Date: now.Add(-2 * time.Hour), Message: "refactor: code"},
		},
	}

	svc := NewService(map[string]ProjectForge{
		"gh-project":     {Forge: ghForge, ProjectID: "org/repo"},
		"gerrit-project": {Forge: gerritForge, ProjectID: "openstack/nova"},
	})

	commits, results, err := svc.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(commits) != 3 {
		t.Fatalf("len(commits) = %d, want 3", len(commits))
	}

	// Should be sorted by Date descending.
	for i := 1; i < len(commits); i++ {
		if commits[i].Date.After(commits[i-1].Date) {
			t.Errorf("commits[%d].Date (%v) > commits[%d].Date (%v)", i, commits[i].Date, i-1, commits[i-1].Date)
		}
	}

	// Repo field should be set to project name.
	for _, c := range commits {
		if c.Repo == "" {
			t.Errorf("commit %s has empty Repo", c.SHA)
		}
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", r.ProjectName, r.Err)
		}
	}
}

func TestService_List_FilterByProject(t *testing.T) {
	ghForge := &mockForge{
		forgeType: forge.ForgeGitHub,
		commits:   []forge.Commit{{SHA: "aaa", Message: "GH commit"}},
	}
	gerritForge := &mockForge{
		forgeType: forge.ForgeGerrit,
		commits:   []forge.Commit{{SHA: "bbb", Message: "Gerrit commit"}},
	}

	svc := NewService(map[string]ProjectForge{
		"gh-project":     {Forge: ghForge, ProjectID: "org/repo"},
		"gerrit-project": {Forge: gerritForge, ProjectID: "openstack/nova"},
	})

	commits, _, _ := svc.List(context.Background(), ListOptions{Projects: []string{"gh-project"}})
	if len(commits) != 1 || commits[0].SHA != "aaa" {
		t.Errorf("expected only GH commit, got %v", commits)
	}
}

func TestService_List_FilterByForge(t *testing.T) {
	ghForge := &mockForge{
		forgeType: forge.ForgeGitHub,
		commits:   []forge.Commit{{SHA: "aaa"}},
	}
	gerritForge := &mockForge{
		forgeType: forge.ForgeGerrit,
		commits:   []forge.Commit{{SHA: "bbb"}},
	}

	svc := NewService(map[string]ProjectForge{
		"gh-project":     {Forge: ghForge, ProjectID: "org/repo"},
		"gerrit-project": {Forge: gerritForge, ProjectID: "openstack/nova"},
	})

	commits, _, _ := svc.List(context.Background(), ListOptions{
		Forges: []forge.ForgeType{forge.ForgeGerrit},
	})
	if len(commits) != 1 || commits[0].SHA != "bbb" {
		t.Errorf("expected only Gerrit commit, got %v", commits)
	}
}

func TestService_List_FilterByAuthor(t *testing.T) {
	f := &mockForge{
		forgeType: forge.ForgeGitHub,
		commits: []forge.Commit{
			{SHA: "aaa", Author: "alice"},
			{SHA: "bbb", Author: "bob"},
		},
	}

	svc := NewService(map[string]ProjectForge{
		"project": {Forge: f, ProjectID: "org/repo"},
	})

	commits, _, _ := svc.List(context.Background(), ListOptions{Author: "alice"})
	if len(commits) != 1 || commits[0].Author != "alice" {
		t.Errorf("expected only alice's commit, got %v", commits)
	}
}

func TestService_List_FilterByBugID(t *testing.T) {
	f := &mockForge{
		forgeType: forge.ForgeGitHub,
		commits: []forge.Commit{
			{SHA: "aaa", Message: "fix: LP: #12345", BugRefs: []string{"12345"}},
			{SHA: "bbb", Message: "feat: new thing", BugRefs: nil},
			{SHA: "ccc", Message: "fix: LP: #12345 and LP: #99999", BugRefs: []string{"12345", "99999"}},
		},
	}

	svc := NewService(map[string]ProjectForge{
		"project": {Forge: f, ProjectID: "org/repo"},
	})

	commits, _, _ := svc.List(context.Background(), ListOptions{BugID: "12345"})
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits referencing bug 12345, got %d", len(commits))
	}
	for _, c := range commits {
		if c.SHA != "aaa" && c.SHA != "ccc" {
			t.Errorf("unexpected commit %s", c.SHA)
		}
	}
}

func TestService_List_GracefulDegradation(t *testing.T) {
	goodForge := &mockForge{
		forgeType: forge.ForgeGitHub,
		commits:   []forge.Commit{{SHA: "aaa", Message: "good commit"}},
	}
	badForge := &mockForge{
		forgeType: forge.ForgeGerrit,
		err:       fmt.Errorf("connection refused"),
	}

	svc := NewService(map[string]ProjectForge{
		"good-project": {Forge: goodForge, ProjectID: "org/repo"},
		"bad-project":  {Forge: badForge, ProjectID: "openstack/nova"},
	})

	commits, results, err := svc.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() should not return top-level error: %v", err)
	}

	if len(commits) != 1 {
		t.Errorf("expected 1 commit from good project, got %d", len(commits))
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

func TestService_List_Empty(t *testing.T) {
	svc := NewService(map[string]ProjectForge{})

	commits, results, err := svc.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits, got %d", len(commits))
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestService_List_BugIDNoMatch(t *testing.T) {
	f := &mockForge{
		forgeType: forge.ForgeGitHub,
		commits: []forge.Commit{
			{SHA: "aaa", Message: "unrelated fix", BugRefs: []string{"99999"}},
		},
	}

	svc := NewService(map[string]ProjectForge{
		"project": {Forge: f, ProjectID: "org/repo"},
	})

	commits, _, _ := svc.List(context.Background(), ListOptions{BugID: "12345"})
	if len(commits) != 0 {
		t.Errorf("expected 0 commits, got %d", len(commits))
	}
}
