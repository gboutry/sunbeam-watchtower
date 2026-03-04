// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"testing"
	"time"

	"github.com/andygrunwald/go-gerrit"
)

func TestGerritForge_Type(t *testing.T) {
	forge := NewGerritForge(nil, "https://review.opendev.org")
	if forge.Type() != ForgeGerrit {
		t.Errorf("Type() = %v, want ForgeGerrit", forge.Type())
	}
}

func TestGerritMergeState(t *testing.T) {
	tests := []struct {
		name   string
		change *gerrit.ChangeInfo
		want   MergeState
	}{
		{"NEW", &gerrit.ChangeInfo{Status: "NEW"}, MergeStateOpen},
		{"MERGED", &gerrit.ChangeInfo{Status: "MERGED"}, MergeStateMerged},
		{"ABANDONED", &gerrit.ChangeInfo{Status: "ABANDONED"}, MergeStateAbandoned},
		{"WIP", &gerrit.ChangeInfo{Status: "NEW", WorkInProgress: true}, MergeStateWIP},
		{"unknown defaults open", &gerrit.ChangeInfo{Status: "SOMETHING"}, MergeStateOpen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gerritMergeState(tt.change); got != tt.want {
				t.Errorf("gerritMergeState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGerritReviewState(t *testing.T) {
	tests := []struct {
		name   string
		change *gerrit.ChangeInfo
		want   ReviewState
	}{
		{
			"no labels",
			&gerrit.ChangeInfo{},
			ReviewStatePending,
		},
		{
			"no Code-Review label",
			&gerrit.ChangeInfo{
				Labels: map[string]gerrit.LabelInfo{
					"Verified": {},
				},
			},
			ReviewStatePending,
		},
		{
			"rejected (-2)",
			&gerrit.ChangeInfo{
				Labels: map[string]gerrit.LabelInfo{
					"Code-Review": {
						Rejected: gerrit.AccountInfo{AccountID: 1},
					},
				},
			},
			ReviewStateRejected,
		},
		{
			"disliked (-1)",
			&gerrit.ChangeInfo{
				Labels: map[string]gerrit.LabelInfo{
					"Code-Review": {
						Disliked: gerrit.AccountInfo{AccountID: 1},
					},
				},
			},
			ReviewStateChangesRequested,
		},
		{
			"approved (+2)",
			&gerrit.ChangeInfo{
				Labels: map[string]gerrit.LabelInfo{
					"Code-Review": {
						Approved: gerrit.AccountInfo{AccountID: 1},
					},
				},
			},
			ReviewStateApproved,
		},
		{
			"recommended (+1)",
			&gerrit.ChangeInfo{
				Labels: map[string]gerrit.LabelInfo{
					"Code-Review": {
						Recommended: gerrit.AccountInfo{AccountID: 1},
					},
				},
			},
			ReviewStateApproved,
		},
		{
			"empty Code-Review label",
			&gerrit.ChangeInfo{
				Labels: map[string]gerrit.LabelInfo{
					"Code-Review": {},
				},
			},
			ReviewStatePending,
		},
		{
			"rejected overrides approved",
			&gerrit.ChangeInfo{
				Labels: map[string]gerrit.LabelInfo{
					"Code-Review": {
						Rejected: gerrit.AccountInfo{AccountID: 1},
						Approved: gerrit.AccountInfo{AccountID: 2},
					},
				},
			},
			ReviewStateRejected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gerritReviewState(tt.change); got != tt.want {
				t.Errorf("gerritReviewState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGerritStatusQuery(t *testing.T) {
	tests := []struct {
		state MergeState
		want  string
	}{
		{MergeStateOpen, "status:open"},
		{MergeStateWIP, "status:open"},
		{MergeStateMerged, "status:merged"},
		{MergeStateClosed, "status:abandoned"},
		{MergeStateAbandoned, "status:abandoned"},
		{MergeState(99), "(status:open OR status:merged)"},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := gerritStatusQuery(tt.state); got != tt.want {
				t.Errorf("gerritStatusQuery(%v) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestGerritOwnerName(t *testing.T) {
	tests := []struct {
		name  string
		owner *gerrit.AccountInfo
		want  string
	}{
		{"nil owner", nil, ""},
		{"name set", &gerrit.AccountInfo{Name: "Alice"}, "Alice"},
		{"username fallback", &gerrit.AccountInfo{Username: "alice"}, "alice"},
		{"email fallback", &gerrit.AccountInfo{Email: "alice@example.com"}, "alice@example.com"},
		{"empty", &gerrit.AccountInfo{}, ""},
		{"name preferred over username", &gerrit.AccountInfo{Name: "Alice", Username: "alice"}, "Alice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gerritOwnerName(tt.owner); got != tt.want {
				t.Errorf("gerritOwnerName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGerritChangeToMergeRequest(t *testing.T) {
	now := time.Now()
	forge := NewGerritForge(nil, "https://review.opendev.org")

	change := &gerrit.ChangeInfo{
		Number:  12345,
		Subject: "Fix neutron agent crash",
		Branch:  "main",
		Status:  "NEW",
		Project: "openstack/neutron",
		Owner: gerrit.AccountInfo{
			Name: "Alice",
		},
		Created: gerrit.Timestamp{Time: now.Add(-time.Hour)},
		Updated: gerrit.Timestamp{Time: now},
		Labels: map[string]gerrit.LabelInfo{
			"Code-Review": {
				Approved: gerrit.AccountInfo{AccountID: 1},
			},
		},
	}

	mr := forge.changeToMergeRequest("openstack/neutron", change)

	if mr.Forge != ForgeGerrit {
		t.Errorf("Forge = %v, want ForgeGerrit", mr.Forge)
	}
	if mr.Repo != "openstack/neutron" {
		t.Errorf("Repo = %q", mr.Repo)
	}
	if mr.ID != "12345" {
		t.Errorf("ID = %q, want 12345", mr.ID)
	}
	if mr.Title != "Fix neutron agent crash" {
		t.Errorf("Title = %q", mr.Title)
	}
	if mr.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want main", mr.TargetBranch)
	}
	if mr.Author != "Alice" {
		t.Errorf("Author = %q, want Alice", mr.Author)
	}
	if mr.State != MergeStateOpen {
		t.Errorf("State = %v, want Open", mr.State)
	}
	if mr.ReviewState != ReviewStateApproved {
		t.Errorf("ReviewState = %v, want Approved", mr.ReviewState)
	}
	if mr.URL != "https://review.opendev.org/c/openstack/neutron/+/12345" {
		t.Errorf("URL = %q", mr.URL)
	}
}

func TestGerritChangeURL(t *testing.T) {
	forge := NewGerritForge(nil, "https://review.opendev.org")

	tests := []struct {
		name   string
		change *gerrit.ChangeInfo
		want   string
	}{
		{
			"uses URL field if set",
			&gerrit.ChangeInfo{URL: "https://custom.example.com/123"},
			"https://custom.example.com/123",
		},
		{
			"constructs URL from base",
			&gerrit.ChangeInfo{Project: "openstack/nova", Number: 42},
			"https://review.opendev.org/c/openstack/nova/+/42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := forge.changeURL(tt.change); got != tt.want {
				t.Errorf("changeURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGerritChangeToCommit(t *testing.T) {
	now := time.Now()
	forge := NewGerritForge(nil, "https://review.opendev.org")

	change := &gerrit.ChangeInfo{
		Number:          100,
		Subject:         "Fix crash LP: #12345",
		Project:         "openstack/nova",
		CurrentRevision: "abc123def456",
		Owner:           gerrit.AccountInfo{Name: "Bob"},
		Updated:         gerrit.Timestamp{Time: now},
	}

	commit := forge.changeToCommit("openstack/nova", change)

	if commit.Forge != ForgeGerrit {
		t.Errorf("Forge = %v, want ForgeGerrit", commit.Forge)
	}
	if commit.SHA != "abc123def456" {
		t.Errorf("SHA = %q", commit.SHA)
	}
	if commit.Message != "Fix crash LP: #12345" {
		t.Errorf("Message = %q", commit.Message)
	}
	if commit.Author != "Bob" {
		t.Errorf("Author = %q, want Bob", commit.Author)
	}
	if len(commit.BugRefs) != 1 || commit.BugRefs[0] != "12345" {
		t.Errorf("BugRefs = %v, want [12345]", commit.BugRefs)
	}
	if commit.URL != "https://review.opendev.org/c/openstack/nova/+/100" {
		t.Errorf("URL = %q", commit.URL)
	}
}
