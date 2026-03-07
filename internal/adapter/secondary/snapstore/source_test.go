// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package snapstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestSourceFetch(t *testing.T) {
	oldEndpoint := infoEndpoint
	defer func() { infoEndpoint = oldEndpoint }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Snap-Device-Series"); got != "16" {
			t.Fatalf("Snap-Device-Series = %q, want 16", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"channel-map": []map[string]any{
				{"channel": map[string]any{"architecture": "amd64", "name": "2024.1/stable", "released-at": "2026-03-01T10:00:00Z", "risk": "stable", "track": "2024.1"}, "revision": 41, "version": "1.2.3"},
				{"channel": map[string]any{"architecture": "arm64", "name": "2024.1/stable", "released-at": "2026-03-01T11:00:00Z", "risk": "stable", "track": "2024.1"}, "revision": 42, "version": "1.2.3"},
				{"channel": map[string]any{"architecture": "amd64", "name": "2025.1/edge", "released-at": "2026-03-02T10:00:00Z", "risk": "edge", "track": "2025.1"}, "revision": 50, "version": "1.3.0"},
			},
		})
	}))
	defer server.Close()
	infoEndpoint = server.URL + "/"

	source := NewSource(server.Client())
	got, err := source.Fetch(context.Background(), dto.TrackedPublication{
		Project:      "sunbeam",
		Name:         "snap-openstack",
		ArtifactType: dto.ArtifactSnap,
		Tracks:       []string{"2024.1"},
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got.Name != "snap-openstack" || len(got.Channels) != 1 || len(got.Channels[0].Targets) != 2 {
		t.Fatalf("Fetch() = %+v, want one filtered channel with two targets", got)
	}
}
