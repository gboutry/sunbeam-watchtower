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

func TestReleasesCommandsRenderListAndShow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/releases":
			_ = json.NewEncoder(w).Encode(map[string]any{"releases": []dto.ReleaseListEntry{{Project: "sunbeam", ArtifactType: dto.ArtifactSnap, Name: "snap-openstack", Track: "2024.1", Risk: dto.ReleaseRiskStable, Branch: "risc-v"}}})
		case "/api/v1/releases/snap-openstack":
			_ = json.NewEncoder(w).Encode(dto.ReleaseShowResult{Project: "sunbeam", ArtifactType: dto.ArtifactSnap, Name: "snap-openstack", Tracks: []string{"2024.1"}, Channels: []dto.ReleaseChannelSnapshot{{Track: "2024.1", Risk: dto.ReleaseRiskStable, Branch: "risc-v"}}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}, Output: "table", Client: client.NewClient(server.URL), Logger: discardTestLogger()}
	cmd := newReleasesCmd(opts)
	cmd.SetArgs([]string{"list"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("list Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "snap-openstack") || !strings.Contains(out.String(), "risc-v") {
		t.Fatalf("unexpected list output: %q", out.String())
	}

	out.Reset()
	cmd = newReleasesCmd(opts)
	cmd.SetArgs([]string{"show", "snap-openstack"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("show Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Project:") || !strings.Contains(out.String(), "snap-openstack") {
		t.Fatalf("unexpected show output: %q", out.String())
	}
}
