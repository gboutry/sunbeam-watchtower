// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
)

func TestCacheClientWorkflowSyncGit(t *testing.T) {
	var gotBody client.CacheSyncGitOptions

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cache/sync/git" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(client.CacheSyncGitResult{
			Synced:   2,
			Warnings: []string{"partial"},
		})
	}))
	defer ts.Close()

	workflow := NewCacheClientWorkflow(client.NewClient(ts.URL))
	got, err := workflow.SyncGit(context.Background(), "keystone")
	if err != nil {
		t.Fatalf("SyncGit() error = %v", err)
	}
	if got.Synced != 2 || len(got.Warnings) != 1 || got.Warnings[0] != "partial" {
		t.Fatalf("SyncGit() = %+v, want synced result", got)
	}
	if gotBody.Project != "keystone" {
		t.Fatalf("request body = %+v, want project keystone", gotBody)
	}
}

func TestCacheClientWorkflowSyncPackagesIndex(t *testing.T) {
	var gotBody client.PackagesCacheSyncOptions

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/packages/cache/sync" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	workflow := NewCacheClientWorkflow(client.NewClient(ts.URL))
	err := workflow.SyncPackagesIndex(context.Background(), []string{"ubuntu"}, []string{"noble"}, []string{"none"})
	if err != nil {
		t.Fatalf("SyncPackagesIndex() error = %v", err)
	}
	if len(gotBody.Distros) != 1 || gotBody.Distros[0] != "ubuntu" {
		t.Fatalf("distros = %+v, want ubuntu", gotBody.Distros)
	}
	if len(gotBody.Releases) != 1 || gotBody.Releases[0] != "noble" {
		t.Fatalf("releases = %+v, want noble", gotBody.Releases)
	}
	if len(gotBody.Backports) != 1 || gotBody.Backports[0] != "none" {
		t.Fatalf("backports = %+v, want none", gotBody.Backports)
	}
}

func TestCacheClientWorkflowClearExcuses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/cache/excuses" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query()["tracker"]; len(got) != 2 || got[0] != "ubuntu-devel" || got[1] != "ubuntu-updates" {
			t.Fatalf("tracker query = %+v, want both trackers", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	workflow := NewCacheClientWorkflow(client.NewClient(ts.URL))
	if err := workflow.ClearExcuses(context.Background(), []string{"ubuntu-devel", "ubuntu-updates"}); err != nil {
		t.Fatalf("ClearExcuses() error = %v", err)
	}
}

func TestCacheClientWorkflowStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/cache/status" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(client.CacheStatusResult{
			Git: struct {
				Directory string              `json:"directory"`
				Repos     []client.CacheEntry `json:"repos"`
			}{
				Directory: "/tmp/git",
				Repos:     []client.CacheEntry{{Name: "keystone", Size: "4.0 MiB"}},
			},
		})
	}))
	defer ts.Close()

	workflow := NewCacheClientWorkflow(client.NewClient(ts.URL))
	got, err := workflow.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if got.Git.Directory != "/tmp/git" || len(got.Git.Repos) != 1 || got.Git.Repos[0].Name != "keystone" {
		t.Fatalf("Status() = %+v, want git cache snapshot", got)
	}
}
