// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package charmhub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestSourceFetch(t *testing.T) {
	oldEndpoint := infoEndpoint
	defer func() { infoEndpoint = oldEndpoint }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.RawQuery, "fields=channel-map"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"channel-map": []map[string]any{
					{"channel": map[string]any{"base": map[string]any{"architecture": "amd64", "channel": "22.04", "name": "ubuntu"}, "name": "2024.1/stable", "released-at": "2026-03-01T10:00:00Z", "risk": "stable", "track": "2024.1"}, "revision": map[string]any{"revision": 17, "version": "17"}},
					{"channel": map[string]any{"base": map[string]any{"architecture": "arm64", "channel": "22.04", "name": "ubuntu"}, "name": "2024.1/stable", "released-at": "2026-03-01T11:00:00Z", "risk": "stable", "track": "2024.1"}, "revision": map[string]any{"revision": 18, "version": "18"}},
				},
			})
		case strings.Contains(r.URL.RawQuery, "fields=default-release.resources"):
			if got := r.URL.Query().Get("channel"); got != "2024.1/stable" {
				t.Fatalf("channel query = %q, want 2024.1/stable", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"default-release": map[string]any{
					"resources": []map[string]any{{"name": "keystone-image", "type": "oci-image", "revision": 53, "filename": "", "description": "OCI image"}},
				},
			})
		default:
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()
	infoEndpoint = server.URL + "/"

	source := NewSource(server.Client())
	got, err := source.Fetch(context.Background(), dto.TrackedPublication{
		Project:      "sunbeam-charms",
		Name:         "keystone-k8s",
		ArtifactType: dto.ArtifactCharm,
		Tracks:       []string{"2024.1"},
		Resources:    []string{"keystone-image"},
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got.Name != "keystone-k8s" || len(got.Channels) != 1 || len(got.Channels[0].Resources) != 1 {
		t.Fatalf("Fetch() = %+v, want one channel with one resource", got)
	}
}
