// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestReleasesListAndShowEndpoints(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	application := app.NewApp(&config.Config{}, discardLogger())
	cache, err := application.ReleaseCache()
	if err != nil {
		t.Fatalf("ReleaseCache() error = %v", err)
	}
	defer application.Close()
	if err := cache.Store(context.Background(), releaseFixture("sunbeam", "snap-openstack", "2024.1")); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())
	RegisterReleasesAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/releases?track=2024.1&branch=risc-v")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list struct {
		Releases []map[string]any `json:"releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Releases) != 1 {
		t.Fatalf("releases = %+v, want one row", list.Releases)
	}
	if _, ok := list.Releases[0]["released_at"]; !ok {
		t.Fatalf("releases = %+v, want released_at field", list.Releases)
	}

	resp2, err := http.Get(base + "/api/v1/releases/snap-openstack")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func releaseFixture(project, name, track string) dto.PublishedArtifactSnapshot {
	return dto.PublishedArtifactSnapshot{
		Project:      project,
		Name:         name,
		ArtifactType: dto.ArtifactSnap,
		Tracks:       []string{track},
		Channels: []dto.ReleaseChannelSnapshot{{
			Track:     track,
			Risk:      dto.ReleaseRiskStable,
			Branch:    "risc-v",
			Channel:   track + "/stable/risc-v",
			UpdatedAt: time.Now().UTC(),
			Targets:   []dto.ReleaseTargetSnapshot{{Architecture: "amd64", Revision: 12, Version: "1.2.3"}},
		}},
		UpdatedAt: time.Now().UTC(),
	}
}
