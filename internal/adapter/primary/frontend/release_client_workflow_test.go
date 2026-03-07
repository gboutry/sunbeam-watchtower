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
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestReleaseClientWorkflowListAndShow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/releases":
			if got := r.URL.Query().Get("track"); got != "2024.1" {
				t.Fatalf("track query = %q, want 2024.1", got)
			}
			if got := r.URL.Query().Get("branch"); got != "risc-v" {
				t.Fatalf("branch query = %q, want risc-v", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"releases": []dto.ReleaseListEntry{{Name: "snap-openstack", Track: "2024.1", Risk: dto.ReleaseRiskStable, Branch: "risc-v"}}})
		case "/api/v1/releases/snap-openstack":
			if got := r.URL.Query().Get("branch"); got != "risc-v" {
				t.Fatalf("show branch query = %q, want risc-v", got)
			}
			_ = json.NewEncoder(w).Encode(dto.ReleaseShowResult{Name: "snap-openstack", ArtifactType: dto.ArtifactSnap})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	workflow := NewReleaseClientWorkflow(NewClientTransport(client.NewClient(server.URL)))
	list, err := workflow.List(context.Background(), ReleasesListRequest{Tracks: []string{"2024.1"}, Branches: []string{"risc-v"}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Name != "snap-openstack" {
		t.Fatalf("List() = %+v, want one row", list)
	}
	show, err := workflow.Show(context.Background(), ReleasesShowRequest{Name: "snap-openstack", Branch: "risc-v"})
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if show.Name != "snap-openstack" || show.ArtifactType != dto.ArtifactSnap {
		t.Fatalf("Show() = %+v, want snap-openstack", show)
	}
}
