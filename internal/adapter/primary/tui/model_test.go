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
	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
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

func TestReleaseFilterFormAutocompleteUsesReleaseData(t *testing.T) {
	session := newEmbeddedTestSession(t)
	defer session.Close()

	form := newReleaseFilterForm(session, releasesModel{
		filters: releasesFilters{project: "demo"},
		rows: []dto.ReleaseListEntry{
			{
				Project:      "demo",
				Name:         "artifact-a",
				ArtifactType: dto.ArtifactCharm,
				Track:        "2024.1",
				Risk:         dto.ReleaseRiskStable,
				Branch:       "risc-v",
			},
		},
	})

	assertSuggestionsContain(t, form.fields[0].AvailableSuggestions(), "demo")
	assertSuggestionsContain(t, form.fields[1].AvailableSuggestions(), "charm")
	assertSuggestionsContain(t, form.fields[2].AvailableSuggestions(), "stable")
	assertSuggestionsContain(t, form.fields[3].AvailableSuggestions(), "2024.1")
	assertSuggestionsContain(t, form.fields[4].AvailableSuggestions(), "risc-v")
}

func TestVimMotionGGAndGJumpReleases(t *testing.T) {
	model := newRootModel(nil, true)
	model.activeView = viewReleases
	model.releases.artifacts = []releaseArtifactSummary{
		{Name: "artifact-a", ArtifactType: dto.ArtifactCharm},
		{Name: "artifact-b", ArtifactType: dto.ArtifactSnap},
		{Name: "artifact-c", ArtifactType: dto.ArtifactRock},
	}
	model.releases.index = 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = next.(rootModel)
	if model.releases.index != 1 {
		t.Fatalf("index after first g = %d, want unchanged", model.releases.index)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = next.(rootModel)
	if model.releases.index != 0 {
		t.Fatalf("index after gg = %d, want 0", model.releases.index)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model = next.(rootModel)
	if model.releases.index != 2 {
		t.Fatalf("index after G = %d, want 2", model.releases.index)
	}
}

func TestRenderMetaPaneListsVimShortcuts(t *testing.T) {
	model := newRootModel(nil, true)
	model.width = 120
	model.height = 30
	rendered := model.renderHelp()
	for _, want := range []string{"Meta", "gg jump to the beginning", "G jump to the end", "autocomplete"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("meta pane missing %q:\n%s", want, rendered)
		}
	}
}

func TestRenderViewsAndOverlays(t *testing.T) {
	session := newEmbeddedTestSession(t)
	defer session.Close()

	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	model := newRootModel(session, true)
	model.width = 120
	model.height = 40
	model.dashboard.auth = &dto.AuthStatus{
		Launchpad: dto.LaunchpadAuthStatus{
			Authenticated:   true,
			DisplayName:     "Tester",
			Source:          "file",
			CredentialsPath: "/tmp/creds",
		},
	}
	model.dashboard.ops = []dto.OperationJob{{
		ID:          "op-1",
		Kind:        dto.OperationKindBuildTrigger,
		State:       dto.OperationStateRunning,
		Summary:     "build artifact-a",
		Cancellable: true,
	}}
	model.dashboard.builds = []dto.Build{{
		Project:     "demo",
		Recipe:      "demo-artifact",
		Title:       "demo build",
		State:       dto.BuildBuilding,
		Arch:        "amd64",
		CreatedAt:   now,
		StartedAt:   now,
		WebLink:     "https://example.test/build",
		BuildLogURL: "https://example.test/log",
		CanRetry:    true,
		CanCancel:   true,
	}}
	model.builds.rows = model.dashboard.builds
	model.releases.artifacts = []releaseArtifactSummary{{
		Project:      "demo",
		Name:         "artifact-a",
		ArtifactType: dto.ArtifactCharm,
		ReleasedAt:   now,
	}}
	model.releases.detail = &dto.ReleaseShowResult{
		Project:      "demo",
		Name:         "artifact-a",
		ArtifactType: dto.ArtifactCharm,
		Tracks:       []string{"2024.1"},
		Channels: []dto.ReleaseChannelSnapshot{{
			Channel: "2024.1/stable",
			Track:   "2024.1",
			Risk:    dto.ReleaseRiskStable,
			Branch:  "risc-v",
			Targets: []dto.ReleaseTargetSnapshot{{Architecture: "amd64", Revision: 3, Version: "1.2.3", ReleasedAt: now}},
			Resources: []dto.ReleaseResourceSnapshot{{
				Name:     "image",
				Revision: 7,
			}},
		}},
	}
	model.ops.rows = model.dashboard.ops
	model.ops.events = []dto.OperationEvent{{Time: now, Type: "info", Message: "queued"}}
	model.auth.status = model.dashboard.auth
	model.auth.begin = &dto.LaunchpadAuthBeginResult{FlowID: "flow-1", AuthorizeURL: "https://example.test/auth"}
	model.cache.status = &frontend.CacheStatusResponse{
		Git: struct {
			Directory string
			Repos     []frontend.CacheEntry
		}{Repos: []frontend.CacheEntry{{Name: "repo"}}},
		Packages: struct {
			Directory string
			Sources   []dto.CacheStatus
			Error     string
		}{Sources: []dto.CacheStatus{{Name: "archive"}}},
		Bugs: struct {
			Directory string
			Entries   []dto.BugCacheStatus
			Error     string
		}{Entries: []dto.BugCacheStatus{{Project: "demo"}}},
		Excuses: struct {
			Directory string
			Entries   []dto.ExcusesCacheStatus
			Error     string
		}{Entries: []dto.ExcusesCacheStatus{{Tracker: "ubuntu", EntryCount: 1}}},
		Releases: struct {
			Directory string
			Entries   []dto.ReleaseCacheStatus
			Error     string
		}{Entries: []dto.ReleaseCacheStatus{{Project: "demo", Name: "artifact-a"}}},
	}
	model.server.local = &runtimeadapter.LocalServerStatus{Running: true, PID: 42, LogFile: "/tmp/watchtower.log"}
	model.prompt = promptModel{title: "Switch?", body: "body", accept: "Yes", reject: "No"}
	model.buildFilterForm = newBuildFilterForm(buildsFilters{project: "demo", state: "building", active: true, source: "remote"})
	model.releaseFilterForm = newReleaseFilterForm(session, releasesModel{
		filters: releasesFilters{project: "demo"},
		rows: []dto.ReleaseListEntry{{
			Project:      "demo",
			Name:         "artifact-a",
			ArtifactType: dto.ArtifactCharm,
			Track:        "2024.1",
			Risk:         dto.ReleaseRiskStable,
		}},
	})
	model.buildTriggerForm = newBuildTriggerForm(session)

	model.activeView = viewBuilds
	if rendered := model.renderBuilds(); !strings.Contains(rendered, "demo build") {
		t.Fatalf("renderBuilds missing build detail:\n%s", rendered)
	}

	model.activeView = viewReleases
	if rendered := model.renderReleases(); !strings.Contains(rendered, "artifact-a") {
		t.Fatalf("renderReleases missing artifact:\n%s", rendered)
	}

	for _, tc := range []struct {
		name    string
		overlay overlayKind
		want    string
	}{
		{name: "auth", overlay: overlayAuth, want: "Authorize URL:"},
		{name: "operations", overlay: overlayOperations, want: "Events"},
		{name: "cache", overlay: overlayCache, want: "Release entries: 1"},
		{name: "server", overlay: overlayServer, want: "PID: 42"},
		{name: "prompt", overlay: overlayPrompt, want: "[Enter] Yes"},
		{name: "build-filters", overlay: overlayBuildFilters, want: "Build Filters"},
		{name: "release-filters", overlay: overlayReleaseFilters, want: "Release Filters"},
		{name: "build-trigger", overlay: overlayBuildTrigger, want: "Trigger Build"},
	} {
		model.overlay = tc.overlay
		rendered := model.renderOverlay("base")
		if !strings.Contains(rendered, tc.want) {
			t.Fatalf("%s overlay missing %q:\n%s", tc.name, tc.want, rendered)
		}
	}
}

func TestUpdateGlobalAndOverlayNavigation(t *testing.T) {
	session := newEmbeddedTestSession(t)
	defer session.Close()

	model := newRootModel(session, true)
	model.width = 120
	model.height = 40
	model.dashboard.ops = []dto.OperationJob{{ID: "op-1", Kind: dto.OperationKindBuildTrigger, State: dto.OperationStateQueued}}
	model.builds.rows = []dto.Build{
		{Project: "demo", Title: "a", State: dto.BuildPending},
		{Project: "demo", Title: "b", State: dto.BuildBuilding},
	}
	model.releases.artifacts = []releaseArtifactSummary{
		{Name: "artifact-a", ArtifactType: dto.ArtifactCharm},
		{Name: "artifact-b", ArtifactType: dto.ArtifactSnap},
	}
	model.ops.rows = []dto.OperationJob{
		{ID: "op-1", Kind: dto.OperationKindBuildTrigger, State: dto.OperationStateRunning, Summary: "one", Cancellable: true},
		{ID: "op-2", Kind: dto.OperationKindBuildTrigger, State: dto.OperationStateSucceeded, Summary: "two"},
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	model = next.(rootModel)
	if model.overlay != overlayHelp {
		t.Fatalf("overlay = %v, want help", model.overlay)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model = next.(rootModel)
	if model.overlayScroll == 0 {
		t.Fatal("overlayScroll = 0, want end offset")
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(rootModel)
	if model.overlay != overlayNone {
		t.Fatalf("overlay = %v, want none", model.overlay)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = next.(rootModel)
	if model.builds.index != 1 {
		t.Fatalf("builds.index = %d, want 1", model.builds.index)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(rootModel)
	if model.overlay != overlayBuildFilters {
		t.Fatalf("overlay = %v, want build filters", model.overlay)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(rootModel)
	if model.overlay != overlayReleaseFilters {
		t.Fatalf("overlay = %v, want release filters", model.overlay)
	}

	model.overlay = overlayNone
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = next.(rootModel)
	if model.overlay != overlayOperations {
		t.Fatalf("overlay = %v, want operations", model.overlay)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = next.(rootModel)
	if model.ops.index != 1 {
		t.Fatalf("ops.index = %d, want 1", model.ops.index)
	}
}

func TestFormHelpersAndUtilityRendering(t *testing.T) {
	form := newFormModal("Release Filters", []fieldDef{
		{placeholder: "project", value: "de", suggestions: []string{"demo", "demo2"}},
		{placeholder: "track", value: "2024.1"},
	})
	if ok := acceptFormSuggestion(&form); !ok {
		t.Fatal("acceptFormSuggestion() = false, want true")
	}
	if got := form.fields[0].Value(); got != "demo" {
		t.Fatalf("form.fields[0].Value() = %q, want demo", got)
	}

	var submitted []string
	cmd := updateFormModal(tea.KeyMsg{Type: tea.KeyTab}, &form, func(values []string) tea.Cmd {
		submitted = append([]string(nil), values...)
		return nil
	}, func() {})
	if cmd != nil {
		t.Fatal("updateFormModal(tab) returned command, want nil")
	}
	if form.active != 1 {
		t.Fatalf("form.active = %d, want 1", form.active)
	}

	_ = updateFormModal(tea.KeyMsg{Type: tea.KeyEnter}, &form, func(values []string) tea.Cmd {
		submitted = append([]string(nil), values...)
		return nil
	}, func() {})
	if len(submitted) != 2 {
		t.Fatalf("submitted len = %d, want 2", len(submitted))
	}

	renderedForm := renderFormModal(newTheme(), form)
	for _, want := range []string{"Release Filters", "suggestions"} {
		if !strings.Contains(renderedForm, want) {
			t.Fatalf("renderFormModal missing %q:\n%s", want, renderedForm)
		}
	}

	if got := renderBuildRows(newTheme(), []dto.Build{{Project: "demo", Title: "build-a", State: dto.BuildPending}}, 0, 80); !strings.Contains(got, "build-a") {
		t.Fatalf("renderBuildRows missing build title:\n%s", got)
	}
	if got := renderBuildDetail(newTheme(), &dto.Build{Project: "demo", Recipe: "recipe", Title: "build-a", State: dto.BuildPending, CreatedAt: time.Unix(0, 0)}, 80); !strings.Contains(got, "Project: demo") {
		t.Fatalf("renderBuildDetail missing project:\n%s", got)
	}
	if got := renderReleaseArtifacts(newTheme(), []releaseArtifactSummary{{Name: "artifact-a", ArtifactType: dto.ArtifactCharm, ReleasedAt: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}}, 0, 40); !strings.Contains(got, "artifact-a") {
		t.Fatalf("renderReleaseArtifacts missing artifact:\n%s", got)
	}
	if got := renderOperationRows(newTheme(), []dto.OperationJob{{State: dto.OperationStateQueued, Kind: dto.OperationKindBuildTrigger, Summary: "build"}}, 0); !strings.Contains(got, "build") {
		t.Fatalf("renderOperationRows missing summary:\n%s", got)
	}
	if got := renderOperationEvents(newTheme(), []dto.OperationEvent{{Time: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC), Message: "queued"}}); !strings.Contains(got, "queued") {
		t.Fatalf("renderOperationEvents missing event:\n%s", got)
	}
	if got := renderViewport("one\ntwo\nthree", 2, 1); !strings.Contains(got, "two") || strings.Contains(got, "one") {
		t.Fatalf("renderViewport unexpected window:\n%s", got)
	}
	if got := truncateToWidth("neutron-baremetal-switch-config-k8s", 12); !strings.HasSuffix(got, "…") {
		t.Fatalf("truncateToWidth() = %q, want ellipsis", got)
	}
	if got := padRight("snap", 6); got != "snap  " {
		t.Fatalf("padRight() = %q, want padded string", got)
	}
	if got := renderToast(newTheme(), toastState{message: "ok", level: "success"}); !strings.Contains(got, "ok") {
		t.Fatalf("renderToast missing message: %q", got)
	}
	if got := displayLaunchpadName(&dto.AuthStatus{Launchpad: dto.LaunchpadAuthStatus{Username: "tester"}}); got != "tester" {
		t.Fatalf("displayLaunchpadName() = %q, want tester", got)
	}
	if got := emptyAsAny(""); got != "any" {
		t.Fatalf("emptyAsAny() = %q, want any", got)
	}
	if got := formatListTime(time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)); got != "2026-03-08 12:00" {
		t.Fatalf("formatListTime() = %q", got)
	}
	if got := firstNonEmptySlice("demo"); len(got) != 1 || got[0] != "demo" {
		t.Fatalf("firstNonEmptySlice() = %v", got)
	}
	if got := errString(context.Canceled); got == "" {
		t.Fatal("errString() = empty, want message")
	}
	if got := errorsJoin(context.Canceled, context.DeadlineExceeded); got == nil {
		t.Fatal("errorsJoin() = nil, want joined error")
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

func TestReadOnlyTriggerBuildIsDenied(t *testing.T) {
	session := newReadOnlyEmbeddedTestSession(t)
	defer session.Close()

	msg := triggerBuildCmd(session, frontend.BuildTriggerRequest{Project: "demo", Async: true})()
	result, ok := msg.(actionDeniedMsg)
	if !ok {
		t.Fatalf("msg = %T, want actionDeniedMsg", msg)
	}
	if result.err == nil {
		t.Fatal("actionDeniedMsg.err = nil, want read-only denial")
	}
}

func TestReadOnlyLogoutIsDenied(t *testing.T) {
	session := newReadOnlyEmbeddedTestSession(t)
	defer session.Close()

	msg := logoutAuthCmd(session)()
	result, ok := msg.(actionDeniedMsg)
	if !ok {
		t.Fatalf("msg = %T, want actionDeniedMsg", msg)
	}
	if result.err == nil {
		t.Fatal("actionDeniedMsg.err = nil, want read-only denial")
	}
}

func TestReadOnlyOperationCancelIsDenied(t *testing.T) {
	session := newReadOnlyEmbeddedTestSession(t)
	defer session.Close()

	msg := cancelOperationCmd(session, "op-1")()
	result, ok := msg.(actionDeniedMsg)
	if !ok {
		t.Fatalf("msg = %T, want actionDeniedMsg", msg)
	}
	if result.err == nil {
		t.Fatal("actionDeniedMsg.err = nil, want read-only denial")
	}
}

func newEmbeddedTestSession(t *testing.T) *runtimeadapter.Session {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	session, err := runtimeadapter.NewSession(context.Background(), runtimeadapter.Options{
		LogWriter:    &bytes.Buffer{},
		TargetPolicy: runtimeadapter.TargetPolicyPreferEmbedded,
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return session
}

func newReadOnlyEmbeddedTestSession(t *testing.T) *runtimeadapter.Session {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	session, err := runtimeadapter.NewSession(context.Background(), runtimeadapter.Options{
		LogWriter:    &bytes.Buffer{},
		TargetPolicy: runtimeadapter.TargetPolicyPreferEmbedded,
		AccessMode:   runtimeadapter.AccessModeReadOnly,
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return session
}

func assertSuggestionsContain(t *testing.T, suggestions []string, want string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion == want {
			return
		}
	}
	t.Fatalf("suggestions %v do not contain %q", suggestions, want)
}
