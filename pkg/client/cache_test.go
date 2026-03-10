// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheSyncGitSendsProjects(t *testing.T) {
	var got CacheSyncGitOptions
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cache/sync/git" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(CacheSyncGitResult{Synced: 2})
	}))
	defer ts.Close()

	result, err := NewClient(ts.URL).CacheSyncGit(context.Background(), CacheSyncGitOptions{Projects: []string{"a", "b"}})
	if err != nil {
		t.Fatalf("CacheSyncGit() error = %v", err)
	}
	if result.Synced != 2 {
		t.Fatalf("CacheSyncGit() = %+v, want synced result", result)
	}
	if len(got.Projects) != 2 || got.Projects[0] != "a" || got.Projects[1] != "b" {
		t.Fatalf("request body = %+v, want projects a/b", got)
	}
}

func TestCacheDeleteAddsRepeatedProjectQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/cache/git" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query()["project"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Fatalf("project query = %+v, want a/b", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	if err := NewClient(ts.URL).CacheDelete(context.Background(), "git", []string{"a", "b"}); err != nil {
		t.Fatalf("CacheDelete() error = %v", err)
	}
}

func TestCacheSyncReviewsSendsProjectsAndSince(t *testing.T) {
	var got CacheSyncReviewsOptions
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cache/sync/reviews" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(CacheSyncReviewsResult{ProjectsSynced: 1})
	}))
	defer ts.Close()

	result, err := NewClient(ts.URL).CacheSyncReviews(context.Background(), CacheSyncReviewsOptions{
		Projects: []string{"snap-openstack", "sunbeam-charms"},
		Since:    "2026-03-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("CacheSyncReviews() error = %v", err)
	}
	if result.ProjectsSynced != 1 {
		t.Fatalf("CacheSyncReviews() = %+v, want counted result", result)
	}
	if len(got.Projects) != 2 || got.Projects[0] != "snap-openstack" || got.Projects[1] != "sunbeam-charms" || got.Since != "2026-03-01T00:00:00Z" {
		t.Fatalf("request body = %+v, want projects and since", got)
	}
}
