// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestCacheSyncReleasesRendersCountsAndWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cache/sync/releases" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(dto.ReleaseSyncResult{
			Status:     "ok",
			Discovered: 4,
			Synced:     3,
			Skipped:    1,
			Warnings:   []string{"sunbeam: skipped (no series, release.tracks, or release.branches configured)"},
		})
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	opts := &Options{
		Out:    &out,
		ErrOut: &errOut,
		Output: "table",
		Client: client.NewClient(server.URL),
		Logger: discardTestLogger(),
	}

	cmd := newCacheCmd(opts)
	cmd.SetArgs([]string{"sync", "releases"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "discovered 4, synced 3, skipped 1") {
		t.Fatalf("stdout = %q, want counted release sync summary", got)
	}
	if got := errOut.String(); !strings.Contains(got, "warning: sunbeam: skipped") {
		t.Fatalf("stderr = %q, want release skip warning", got)
	}
}
