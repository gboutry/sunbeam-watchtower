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

func TestReleasesList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/releases" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("track"); got != "2024.1" {
			t.Fatalf("track query = %q, want 2024.1", got)
		}
		if got := r.URL.Query().Get("branch"); got != "risc-v" {
			t.Fatalf("branch query = %q, want risc-v", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"releases": []map[string]any{{"name": "snap-openstack", "track": "2024.1", "risk": "stable", "branch": "risc-v"}}})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	got, err := client.ReleasesList(context.Background(), ReleasesListOptions{Tracks: []string{"2024.1"}, Branches: []string{"risc-v"}})
	if err != nil {
		t.Fatalf("ReleasesList() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "snap-openstack" {
		t.Fatalf("ReleasesList() = %+v, want one row", got)
	}
}

func TestReleasesShowAndCacheSyncReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/releases/snap-openstack":
			if got := r.URL.Query().Get("branch"); got != "risc-v" {
				t.Fatalf("show branch query = %q, want risc-v", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "snap-openstack", "artifact_type": 2})
		case "/api/v1/cache/sync/releases":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "discovered": 4, "synced": 3, "skipped": 1})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	show, err := client.ReleasesShow(context.Background(), "snap-openstack", ReleasesShowOptions{Branch: "risc-v"})
	if err != nil {
		t.Fatalf("ReleasesShow() error = %v", err)
	}
	if show.Name != "snap-openstack" {
		t.Fatalf("ReleasesShow() = %+v, want snap-openstack", show)
	}
	result, err := client.CacheSyncReleases(context.Background())
	if err != nil {
		t.Fatalf("CacheSyncReleases() error = %v", err)
	}
	if result.Status != "ok" || result.Discovered != 4 || result.Synced != 3 || result.Skipped != 1 {
		t.Fatalf("CacheSyncReleases() = %+v, want counted result", result)
	}
}
