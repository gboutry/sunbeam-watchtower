// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
)

func TestCacheSyncUpstream_WithoutUpstreamConfigIsSkipped(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterCacheAPI(srv.API(), application)

	resp, err := http.Post(base+"/api/v1/cache/sync/upstream", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "skipped: upstream not configured" {
		t.Fatalf("unexpected status %q", body.Status)
	}
}

func TestCacheDelete_InvalidTypeReturns400(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterCacheAPI(srv.API(), application)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, base+"/api/v1/cache/not-valid", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCacheSyncGit_EmptyConfigReturns400(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterCacheAPI(srv.API(), application)

	resp, err := http.Post(base+"/api/v1/cache/sync/git", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCacheSyncBugs_EmptyConfigReturnsZeroSynced(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterCacheAPI(srv.API(), application)

	resp, err := http.Post(base+"/api/v1/cache/sync/bugs", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCacheSyncReleases_WithConfiguredPublicationReturns200(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterCacheAPI(srv.API(), application)

	resp, err := http.Post(base+"/api/v1/cache/sync/releases", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
