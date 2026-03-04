// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v68/github"
)

func newGitHubTestServer(t *testing.T, mux *http.ServeMux) (*GitHubForge, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(mux)
	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
	return NewGitHubForge(client), server
}

func ptr[T any](v T) *T { return &v }

func TestGitHubForge_Type(t *testing.T) {
	forge := NewGitHubForge(github.NewClient(nil))
	if forge.Type() != ForgeGitHub {
		t.Errorf("Type() = %v, want ForgeGitHub", forge.Type())
	}
}

func TestGitHubForge_ListMergeRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/canonical/sunbeam/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "open" {
			t.Errorf("expected state=open, got %q", r.URL.Query().Get("state"))
		}
		prs := []*github.PullRequest{
			{
				Number:  ptr(42),
				Title:   ptr("Fix the thing"),
				Body:    ptr("A detailed description"),
				State:   ptr("open"),
				Draft:   ptr(false),
				HTMLURL: ptr("https://github.com/canonical/sunbeam/pull/42"),
				User:    &github.User{Login: ptr("alice")},
				Head:    &github.PullRequestBranch{Ref: ptr("fix-thing")},
				Base:    &github.PullRequestBranch{Ref: ptr("main")},
				CreatedAt: &github.Timestamp{Time: time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)},
				UpdatedAt: &github.Timestamp{Time: time.Date(2024, 3, 2, 12, 0, 0, 0, time.UTC)},
			},
			{
				Number:  ptr(43),
				Title:   ptr("WIP: Another thing"),
				State:   ptr("open"),
				Draft:   ptr(true),
				HTMLURL: ptr("https://github.com/canonical/sunbeam/pull/43"),
				User:    &github.User{Login: ptr("bob")},
				Head:    &github.PullRequestBranch{Ref: ptr("wip-branch")},
				Base:    &github.PullRequestBranch{Ref: ptr("main")},
			},
		}
		json.NewEncoder(w).Encode(prs)
	})

	forge, server := newGitHubTestServer(t, mux)
	defer server.Close()

	mrs, err := forge.ListMergeRequests(context.Background(), "canonical/sunbeam", ListMergeRequestsOpts{
		State: MergeStateOpen,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(mrs) != 2 {
		t.Fatalf("len = %d, want 2", len(mrs))
	}

	// First PR: regular open.
	if mrs[0].ID != "#42" {
		t.Errorf("ID = %q, want #42", mrs[0].ID)
	}
	if mrs[0].Title != "Fix the thing" {
		t.Errorf("Title = %q", mrs[0].Title)
	}
	if mrs[0].Author != "alice" {
		t.Errorf("Author = %q", mrs[0].Author)
	}
	if mrs[0].State != MergeStateOpen {
		t.Errorf("State = %v, want Open", mrs[0].State)
	}
	if mrs[0].SourceBranch != "fix-thing" {
		t.Errorf("SourceBranch = %q", mrs[0].SourceBranch)
	}
	if mrs[0].TargetBranch != "main" {
		t.Errorf("TargetBranch = %q", mrs[0].TargetBranch)
	}
	if mrs[0].Description != "A detailed description" {
		t.Errorf("Description = %q", mrs[0].Description)
	}
	if mrs[0].Forge != ForgeGitHub {
		t.Errorf("Forge = %v", mrs[0].Forge)
	}

	// Second PR: draft = WIP.
	if mrs[1].State != MergeStateWIP {
		t.Errorf("State = %v, want WIP", mrs[1].State)
	}
}

func TestGitHubForge_ListMergeRequests_AuthorFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/canonical/sunbeam/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []*github.PullRequest{
			{Number: ptr(1), State: ptr("open"), User: &github.User{Login: ptr("alice")}},
			{Number: ptr(2), State: ptr("open"), User: &github.User{Login: ptr("bob")}},
		}
		json.NewEncoder(w).Encode(prs)
	})

	forge, server := newGitHubTestServer(t, mux)
	defer server.Close()

	mrs, err := forge.ListMergeRequests(context.Background(), "canonical/sunbeam", ListMergeRequestsOpts{
		Author: "alice",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(mrs) != 1 {
		t.Fatalf("len = %d, want 1", len(mrs))
	}
	if mrs[0].Author != "alice" {
		t.Errorf("Author = %q, want alice", mrs[0].Author)
	}
}

func TestGitHubForge_GetMergeRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/canonical/sunbeam/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		pr := &github.PullRequest{
			Number:  ptr(42),
			Title:   ptr("Fix the thing"),
			State:   ptr("open"),
			Merged:  ptr(false),
			HTMLURL: ptr("https://github.com/canonical/sunbeam/pull/42"),
			User:    &github.User{Login: ptr("alice")},
			Head:    &github.PullRequestBranch{Ref: ptr("fix-thing")},
			Base:    &github.PullRequestBranch{Ref: ptr("main")},
		}
		json.NewEncoder(w).Encode(pr)
	})
	mux.HandleFunc("/repos/canonical/sunbeam/pulls/42/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*github.PullRequestReview{
			{User: &github.User{Login: ptr("reviewer1")}, State: ptr("APPROVED")},
		}
		json.NewEncoder(w).Encode(reviews)
	})

	forge, server := newGitHubTestServer(t, mux)
	defer server.Close()

	mr, err := forge.GetMergeRequest(context.Background(), "canonical/sunbeam", "#42")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mr.ID != "#42" {
		t.Errorf("ID = %q", mr.ID)
	}
	if mr.ReviewState != ReviewStateApproved {
		t.Errorf("ReviewState = %v, want Approved", mr.ReviewState)
	}
}

func TestGitHubForge_GetMergeRequest_ChangesRequested(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/canonical/sunbeam/pulls/10", func(w http.ResponseWriter, r *http.Request) {
		pr := &github.PullRequest{
			Number: ptr(10),
			Title:  ptr("Test"),
			State:  ptr("open"),
			User:   &github.User{Login: ptr("alice")},
		}
		json.NewEncoder(w).Encode(pr)
	})
	mux.HandleFunc("/repos/canonical/sunbeam/pulls/10/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*github.PullRequestReview{
			{User: &github.User{Login: ptr("r1")}, State: ptr("APPROVED")},
			{User: &github.User{Login: ptr("r2")}, State: ptr("CHANGES_REQUESTED")},
		}
		json.NewEncoder(w).Encode(reviews)
	})

	forge, server := newGitHubTestServer(t, mux)
	defer server.Close()

	mr, err := forge.GetMergeRequest(context.Background(), "canonical/sunbeam", "10")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mr.ReviewState != ReviewStateChangesRequested {
		t.Errorf("ReviewState = %v, want ChangesRequested", mr.ReviewState)
	}
}

func TestGitHubForge_GetMergeRequest_Merged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/canonical/sunbeam/pulls/99", func(w http.ResponseWriter, r *http.Request) {
		pr := &github.PullRequest{
			Number: ptr(99),
			Title:  ptr("Merged PR"),
			State:  ptr("closed"),
			Merged: ptr(true),
			User:   &github.User{Login: ptr("alice")},
		}
		json.NewEncoder(w).Encode(pr)
	})
	mux.HandleFunc("/repos/canonical/sunbeam/pulls/99/reviews", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*github.PullRequestReview{})
	})

	forge, server := newGitHubTestServer(t, mux)
	defer server.Close()

	mr, err := forge.GetMergeRequest(context.Background(), "canonical/sunbeam", "99")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mr.State != MergeStateMerged {
		t.Errorf("State = %v, want Merged", mr.State)
	}
}

func TestGitHubForge_ListCommits(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/canonical/sunbeam/commits", func(w http.ResponseWriter, r *http.Request) {
		commits := []*github.RepositoryCommit{
			{
				SHA:     ptr("abc123"),
				HTMLURL: ptr("https://github.com/canonical/sunbeam/commit/abc123"),
				Commit: &github.Commit{
					Message: ptr("Fix bug\n\nLP: #12345"),
					Author:  &github.CommitAuthor{Name: ptr("Alice"), Date: &github.Timestamp{Time: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
			{
				SHA: ptr("def456"),
				Commit: &github.Commit{
					Message: ptr("Regular commit"),
					Author:  &github.CommitAuthor{Name: ptr("Bob")},
				},
			},
		}
		json.NewEncoder(w).Encode(commits)
	})

	forge, server := newGitHubTestServer(t, mux)
	defer server.Close()

	commits, err := forge.ListCommits(context.Background(), "canonical/sunbeam", ListCommitsOpts{
		Branch: "main",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("len = %d, want 2", len(commits))
	}

	if commits[0].SHA != "abc123" {
		t.Errorf("SHA = %q", commits[0].SHA)
	}
	if commits[0].Author != "Alice" {
		t.Errorf("Author = %q", commits[0].Author)
	}
	if len(commits[0].BugRefs) != 1 || commits[0].BugRefs[0] != "12345" {
		t.Errorf("BugRefs = %v", commits[0].BugRefs)
	}
	if commits[0].Forge != ForgeGitHub {
		t.Errorf("Forge = %v", commits[0].Forge)
	}

	// Second commit has no bug refs.
	if len(commits[1].BugRefs) != 0 {
		t.Errorf("BugRefs = %v, want empty", commits[1].BugRefs)
	}
}

func TestGitHubForge_InvalidRepo(t *testing.T) {
	forge := NewGitHubForge(github.NewClient(nil))

	_, err := forge.ListMergeRequests(context.Background(), "invalid-repo", ListMergeRequestsOpts{})
	if err == nil {
		t.Error("expected error for invalid repo format")
	}

	_, err = forge.GetMergeRequest(context.Background(), "invalid-repo", "#1")
	if err == nil {
		t.Error("expected error for invalid repo format")
	}

	_, err = forge.ListCommits(context.Background(), "invalid-repo", ListCommitsOpts{})
	if err == nil {
		t.Error("expected error for invalid repo format")
	}
}

func TestGitHubForge_InvalidPRNumber(t *testing.T) {
	forge := NewGitHubForge(github.NewClient(nil))

	_, err := forge.GetMergeRequest(context.Background(), "owner/repo", "not-a-number")
	if err == nil {
		t.Error("expected error for invalid PR number")
	}
}

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"canonical/sunbeam", "canonical", "sunbeam", false},
		{"org/repo-name", "org", "repo-name", false},
		{"invalid", "", "", true},
		{"", "", "", true},
		{"/repo", "", "", true},
		{"owner/", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, err := parseOwnerRepo(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("got %q/%q, want %q/%q", owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestGhMergeState(t *testing.T) {
	tests := []struct {
		name  string
		pr    *github.PullRequest
		want  MergeState
	}{
		{"open", &github.PullRequest{State: ptr("open")}, MergeStateOpen},
		{"closed", &github.PullRequest{State: ptr("closed")}, MergeStateClosed},
		{"merged", &github.PullRequest{State: ptr("closed"), Merged: ptr(true)}, MergeStateMerged},
		{"draft", &github.PullRequest{State: ptr("open"), Draft: ptr(true)}, MergeStateWIP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ghMergeState(tt.pr); got != tt.want {
				t.Errorf("ghMergeState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGhReviewState(t *testing.T) {
	tests := []struct {
		name    string
		reviews []*github.PullRequestReview
		want    ReviewState
	}{
		{
			"no reviews",
			nil,
			ReviewStatePending,
		},
		{
			"approved",
			[]*github.PullRequestReview{
				{User: &github.User{Login: ptr("r1")}, State: ptr("APPROVED")},
			},
			ReviewStateApproved,
		},
		{
			"changes requested overrides approval",
			[]*github.PullRequestReview{
				{User: &github.User{Login: ptr("r1")}, State: ptr("APPROVED")},
				{User: &github.User{Login: ptr("r2")}, State: ptr("CHANGES_REQUESTED")},
			},
			ReviewStateChangesRequested,
		},
		{
			"later approval overrides earlier request from same user",
			[]*github.PullRequestReview{
				{User: &github.User{Login: ptr("r1")}, State: ptr("CHANGES_REQUESTED")},
				{User: &github.User{Login: ptr("r1")}, State: ptr("APPROVED")},
			},
			ReviewStateApproved,
		},
		{
			"comment only is pending",
			[]*github.PullRequestReview{
				{User: &github.User{Login: ptr("r1")}, State: ptr("COMMENTED")},
			},
			ReviewStatePending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ghReviewState(tt.reviews); got != tt.want {
				t.Errorf("ghReviewState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGhPRState(t *testing.T) {
	tests := []struct {
		state MergeState
		want  string
	}{
		{MergeStateOpen, "open"},
		{MergeStateMerged, "closed"},
		{MergeStateClosed, "closed"},
		{MergeStateWIP, "open"},
		{MergeStateAbandoned, "all"},
	}

	for _, tt := range tests {
		if got := ghPRState(tt.state); got != tt.want {
			t.Errorf("ghPRState(%v) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
