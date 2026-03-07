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

func newEmptyPackagesApp(t *testing.T) *app.App {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return app.NewApp(&config.Config{}, discardLogger())
}

func TestPackagesDiff_UnknownSetReturns404(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := newEmptyPackagesApp(t)
	RegisterPackagesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/packages/diff/openstack")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPackagesList_NoConfiguredSourcesReturns400(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := newEmptyPackagesApp(t)
	RegisterPackagesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/packages/list?distro=ubuntu")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPackagesShow_NoConfiguredSourcesReturns200(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := newEmptyPackagesApp(t)
	RegisterPackagesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/packages/show/nova?distro=ubuntu")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPackagesDsc_InvalidPackageFormatReturns400(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := newEmptyPackagesApp(t)
	RegisterPackagesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/packages/dsc?packages=invalid-format")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPackagesExcuses_InvalidTrackerReturns400(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := newEmptyPackagesApp(t)
	RegisterPackagesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/packages/excuses?tracker=not-valid")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPackagesCacheStatus_EmptyConfigReturns200(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := newEmptyPackagesApp(t)
	RegisterPackagesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/packages/cache/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
}

func TestPackagesRdepends_NoConfiguredSourcesReturns400(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := newEmptyPackagesApp(t)
	RegisterPackagesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/packages/rdepends/nova?distro=ubuntu")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
