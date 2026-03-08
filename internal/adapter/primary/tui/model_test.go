// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	runtimeadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/runtime"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestViewRendersAcrossWidths(t *testing.T) {
	model := newRootModel(nil, true)
	model.dashboard.loaded = true
	model.dashboard.auth = &dto.AuthStatus{}
	model.dashboard.ops = []dto.OperationJob{{ID: "op-1", Kind: dto.OperationKindBuildTrigger, State: dto.OperationStateRunning}}
	model.dashboard.builds = []dto.Build{{Project: "demo", Title: "demo-build", State: dto.BuildBuilding}}
	model.builds.rows = []dto.Build{{Project: "demo", Title: "demo-build", State: dto.BuildBuilding}}
	model.releases.rows = []dto.ReleaseListEntry{{Project: "demo", Name: "demo-artifact", Channel: "latest/edge", ArtifactType: dto.ArtifactSnap, ReleasedAt: time.Now()}}
	model.releases.artifacts = summarizeReleaseArtifacts(model.releases.rows)
	model.releases.detail = &dto.ReleaseShowResult{
		Project:      "demo",
		Name:         "demo-artifact",
		ArtifactType: dto.ArtifactSnap,
		Channels:     []dto.ReleaseChannelSnapshot{{Channel: "latest/edge", Track: "latest", Risk: dto.ReleaseRiskEdge}},
		UpdatedAt:    time.Now(),
	}

	for _, width := range []int{80, 100, 140} {
		model.width = width
		model.height = 40
		view := model.View()
		for _, want := range []string{"watchtower-tui", "Dashboard", "Builds", "Releases"} {
			if !strings.Contains(view, want) {
				t.Fatalf("width %d view missing %q", width, want)
			}
		}
	}
}

func TestBuildTriggerRequestFromValuesAlwaysAsync(t *testing.T) {
	req, err := buildTriggerRequestFromValues([]string{"demo", "rock,charm", "remote", ""})
	if err != nil {
		t.Fatalf("buildTriggerRequestFromValues() error = %v", err)
	}
	if !req.Async {
		t.Fatal("req.Async = false, want true")
	}
	if got := len(req.Artifacts); got != 2 {
		t.Fatalf("len(req.Artifacts) = %d, want 2", got)
	}
}

func TestSummarizeReleaseArtifactsDeduplicatesByArtifact(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	artifacts := summarizeReleaseArtifacts([]dto.ReleaseListEntry{
		{Project: "demo", Name: "artifact-a", ArtifactType: dto.ArtifactSnap, Channel: "latest/edge", ReleasedAt: now.Add(-time.Hour)},
		{Project: "demo", Name: "artifact-a", ArtifactType: dto.ArtifactSnap, Channel: "latest/stable", ReleasedAt: now},
		{Project: "demo", Name: "artifact-b", ArtifactType: dto.ArtifactCharm, Channel: "2024.1/stable", ReleasedAt: now.Add(-2 * time.Hour)},
	})
	if got := len(artifacts); got != 2 {
		t.Fatalf("len(artifacts) = %d, want 2", got)
	}
	if artifacts[0].Name != "artifact-a" {
		t.Fatalf("artifacts[0].Name = %q, want artifact-a", artifacts[0].Name)
	}
	if !artifacts[0].ReleasedAt.Equal(now) {
		t.Fatalf("artifacts[0].ReleasedAt = %s, want %s", artifacts[0].ReleasedAt, now)
	}
}

func TestRenderReleaseDetailUsesLatestReleaseTime(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	detail := &dto.ReleaseShowResult{
		Project:      "demo",
		Name:         "artifact-a",
		ArtifactType: dto.ArtifactCharm,
		Tracks:       []string{"latest", "2024.1"},
		UpdatedAt:    now.Add(3 * time.Hour),
		Channels: []dto.ReleaseChannelSnapshot{
			{
				Channel: "latest/stable",
				Track:   "latest",
				Risk:    dto.ReleaseRiskStable,
				Targets: []dto.ReleaseTargetSnapshot{{ReleasedAt: now}},
				Resources: []dto.ReleaseResourceSnapshot{
					{Name: "postgresql-image", Revision: 12},
				},
			},
		},
	}
	selected := &releaseArtifactSummary{Name: "artifact-a", ArtifactType: dto.ArtifactCharm}
	rendered := renderReleaseDetail(newTheme(), detail, selected, 120)
	if !strings.Contains(rendered, "Released: 2026-03-08T12:00:00Z") {
		t.Fatalf("detail missing released timestamp:\n%s", rendered)
	}
	if strings.Contains(rendered, "Updated: 2026-03-08T15:00:00Z") {
		t.Fatalf("detail should not use cache updated time:\n%s", rendered)
	}
	if !strings.Contains(rendered, "resources: postgresql-image:r12") {
		t.Fatalf("detail missing resources:\n%s", rendered)
	}
}

func TestEmbeddedAuthLoginPromptsForUpgrade(t *testing.T) {
	session := newEmbeddedTestSession(t)
	defer session.Close()

	model := newRootModel(session, true)
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model = next.(rootModel)
	if model.overlay != overlayAuth {
		t.Fatalf("overlay = %v, want auth", model.overlay)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model = next.(rootModel)
	if model.overlay != overlayPrompt {
		t.Fatalf("overlay = %v, want prompt", model.overlay)
	}
}

func TestEmbeddedOperationCancelPromptsForUpgrade(t *testing.T) {
	session := newEmbeddedTestSession(t)
	defer session.Close()

	model := newRootModel(session, true)
	model.overlay = overlayOperations
	model.ops.rows = []dto.OperationJob{{ID: "op-1", Kind: dto.OperationKindBuildTrigger, Cancellable: true}}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = next.(rootModel)
	if model.overlay != overlayPrompt {
		t.Fatalf("overlay = %v, want prompt", model.overlay)
	}
}

func TestEmbeddedBuildTriggerPromptsForUpgrade(t *testing.T) {
	session := newEmbeddedTestSession(t)
	defer session.Close()

	model := newRootModel(session, true)
	model.overlay = overlayBuildTrigger
	model.buildTriggerForm = newBuildTriggerForm(session)
	model.buildTriggerForm.fields[0].SetValue("demo")
	model.buildTriggerForm.fields[2].SetValue("remote")
	model.buildTriggerForm.fields[3].SetValue(".")
	model.buildTriggerForm.active = len(model.buildTriggerForm.fields) - 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(rootModel)
	if model.overlay != overlayPrompt {
		t.Fatalf("overlay = %v, want prompt", model.overlay)
	}
}

func newEmbeddedTestSession(t *testing.T) *runtimeadapter.Session {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	session, err := runtimeadapter.NewSession(context.Background(), runtimeadapter.Options{
		LogWriter: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return session
}
