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
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	distro "github.com/gboutry/sunbeam-watchtower/pkg/distro/v1"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
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
		for _, want := range []string{"watchtower-tui", "Dashboard", "Builds", "Releases", "Packages", "Bugs", "Reviews", "Commits", "Projects"} {
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

func TestNewRootModelDefaultsBugViewToMerge(t *testing.T) {
	model := newRootModel(nil, true)
	if !model.bugs.filters.merge {
		t.Fatal("model.bugs.filters.merge = false, want true")
	}
}

func TestSummarizeReleaseArtifactsDeduplicatesByArtifact(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	artifacts := summarizeReleaseArtifacts([]dto.ReleaseListEntry{
		{
			Project:      "demo",
			Name:         "artifact-a",
			ArtifactType: dto.ArtifactSnap,
			Channel:      "latest/edge",
			ReleasedAt:   now.Add(-time.Hour),
			Targets: []dto.ReleaseTargetSnapshot{{
				Architecture: "amd64",
				Base:         dto.ReleaseBase{Name: "ubuntu", Channel: "22.04"},
				Revision:     40,
				ReleasedAt:   now.Add(-time.Hour),
			}},
		},
		{
			Project:      "demo",
			Name:         "artifact-a",
			ArtifactType: dto.ArtifactSnap,
			Channel:      "latest/stable",
			ReleasedAt:   now,
			Targets: []dto.ReleaseTargetSnapshot{{
				Architecture: "amd64",
				Base:         dto.ReleaseBase{Name: "ubuntu", Channel: "24.04"},
				Revision:     41,
				ReleasedAt:   now,
			}},
		},
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
	if got := artifacts[0].LatestVisibleTarget; got != "amd64@ubuntu/24.04:r41" {
		t.Fatalf("artifacts[0].LatestVisibleTarget = %q, want amd64@ubuntu/24.04:r41", got)
	}
	if got := artifacts[0].VisibleTargetCount; got != 2 {
		t.Fatalf("artifacts[0].VisibleTargetCount = %d, want 2", got)
	}
}

func TestSummarizeReleaseArtifactsKeepsSameNameAcrossTypes(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	artifacts := summarizeReleaseArtifacts([]dto.ReleaseListEntry{
		{Project: "demo", Name: "keystone", ArtifactType: dto.ArtifactSnap, Channel: "latest/stable", ReleasedAt: now},
		{Project: "demo", Name: "keystone", ArtifactType: dto.ArtifactCharm, Channel: "2024.1/stable", ReleasedAt: now.Add(-time.Hour)},
	})
	if got := len(artifacts); got != 2 {
		t.Fatalf("len(artifacts) = %d, want 2", got)
	}
	if artifacts[0].ArtifactType != dto.ArtifactSnap || artifacts[1].ArtifactType != dto.ArtifactCharm {
		t.Fatalf("artifacts = %+v, want separate snap and charm summaries", artifacts)
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
				Targets: []dto.ReleaseTargetSnapshot{{
					Architecture: "amd64",
					Base:         dto.ReleaseBase{Name: "ubuntu", Channel: "24.04"},
					Revision:     3,
					Version:      "1.2.3",
					ReleasedAt:   now,
				}},
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
	if !strings.Contains(rendered, "targets: amd64@ubuntu/24.04:r3/1.2.3") {
		t.Fatalf("detail missing target-aware release formatting:\n%s", rendered)
	}
	if !strings.Contains(rendered, "resources: postgresql-image:r12") {
		t.Fatalf("detail missing resources:\n%s", rendered)
	}
}

func TestReleaseFilterFormAutocompleteUsesReleaseData(t *testing.T) {
	session := newEmbeddedTestSession(t)
	defer session.Close()
	session.Config.Releases.TargetProfiles = map[string]config.ReleaseTargetProfileConfig{
		"noble-and-newer": {
			Include: []config.ReleaseTargetMatcherConfig{{BaseNames: []string{"ubuntu"}, MinBaseChannel: "24.04"}},
		},
	}

	form := newReleaseFilterForm(session, releasesModel{
		filters: releasesFilters{project: "demo", targetProfile: "noble-and-newer"},
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
	assertSuggestionsContain(t, form.fields[5].AvailableSuggestions(), "noble-and-newer")
	assertSuggestionsContain(t, form.fields[6].AvailableSuggestions(), "true")
	assertSuggestionsContain(t, form.fields[6].AvailableSuggestions(), "false")
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
		Project:             "demo",
		Name:                "artifact-a",
		ArtifactType:        dto.ArtifactCharm,
		ReleasedAt:          now,
		LatestVisibleTarget: "amd64@ubuntu/24.04:r3",
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
	model.packageFilterForm = newPackageFilterForm(session, model.packages)
	model.bugFilterForm = newBugFilterForm(session, model.bugs)
	model.reviewFilterForm = newReviewFilterForm(session, model.reviews)
	model.commitFilterForm = newCommitFilterForm(session, model.commits)
	model.projectFilterForm = newProjectFilterForm(model.projects)

	model.activeView = viewBuilds
	if rendered := model.renderBuilds(); !strings.Contains(rendered, "demo build") {
		t.Fatalf("renderBuilds missing build detail:\n%s", rendered)
	}

	model.activeView = viewReleases
	if rendered := model.renderReleases(); !strings.Contains(rendered, "artifact-a") {
		t.Fatalf("renderReleases missing artifact:\n%s", rendered)
	}
	if rendered := model.renderReleases(); !strings.Contains(rendered, "amd64@ubuntu/24.04:r3") {
		t.Fatalf("renderReleases missing latest visible target:\n%s", rendered)
	}

	model.packages.inventoryRows = []distro.SourcePackage{{
		Package:   "nova",
		Version:   "2:1.0-0ubuntu1",
		Suite:     "noble",
		Component: "main",
	}}
	model.packages.inventoryDetail = &distro.SourcePackageInfo{
		SourcePackage: distro.SourcePackage{
			Package:   "nova",
			Version:   "2:1.0-0ubuntu1",
			Suite:     "noble",
			Component: "main",
		},
		Fields: []distro.FieldEntry{{Key: "Maintainer", Value: "Canonical"}},
	}
	model.activeView = viewPackages
	if rendered := model.renderPackages(); !strings.Contains(rendered, "nova") {
		t.Fatalf("renderPackages missing package detail:\n%s", rendered)
	}

	model.bugs.rows = []forge.BugTask{{Project: "snap-openstack", BugID: "12345", Status: "Fix Released", Title: "nova bug"}}
	model.bugs.detail = &forge.Bug{ID: "12345", Title: "nova bug", Tasks: []forge.BugTask{{Project: "snap-openstack", Status: "Fix Released"}}}
	model.activeView = viewBugs
	if rendered := model.renderBugs(); !strings.Contains(rendered, "12345") {
		t.Fatalf("renderBugs missing bug detail:\n%s", rendered)
	}

	model.reviews.rows = []forge.MergeRequest{{Repo: "snap-openstack", Forge: forge.ForgeGitHub, ID: "#42", Title: "Improve test coverage", Author: "alice", State: forge.MergeStateOpen}}
	model.reviews.detail = &forge.MergeRequest{Repo: "snap-openstack", Forge: forge.ForgeGitHub, ID: "#42", Title: "Improve test coverage", Author: "alice", State: forge.MergeStateOpen}
	model.activeView = viewReviews
	if rendered := model.renderReviews(); !strings.Contains(rendered, "Improve test coverage") {
		t.Fatalf("renderReviews missing MR detail:\n%s", rendered)
	}

	model.commits.rows = []forge.Commit{{Repo: "snap-openstack", Forge: forge.ForgeGitHub, SHA: "0123456789abcdef", Message: "Fix bug\n\nmore", Author: "alice", Date: now}}
	model.activeView = viewCommits
	if rendered := model.renderCommits(); !strings.Contains(rendered, "0123456789") {
		t.Fatalf("renderCommits missing commit detail:\n%s", rendered)
	}

	model.projects.rows = []projectSummary{{Name: "snap-openstack", CodeForge: "github", CodeProject: "snap-openstack", Series: []string{"2025.1"}}}
	model.activeView = viewProjects
	if rendered := model.renderProjects(); !strings.Contains(rendered, "snap-openstack") {
		t.Fatalf("renderProjects missing project detail:\n%s", rendered)
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
		{name: "package-filters", overlay: overlayPackageFilters, want: "Package Filters"},
		{name: "bug-filters", overlay: overlayBugFilters, want: "Bug Filters"},
		{name: "review-filters", overlay: overlayReviewFilters, want: "Review Filters"},
		{name: "commit-filters", overlay: overlayCommitFilters, want: "Commit Filters"},
		{name: "project-filters", overlay: overlayProjectFilters, want: "Project Filters"},
	} {
		model.overlay = tc.overlay
		rendered := model.renderOverlay("base")
		if !strings.Contains(rendered, tc.want) {
			t.Fatalf("%s overlay missing %q:\n%s", tc.name, tc.want, rendered)
		}
	}
}

func TestRenderDashboardReleasesIncludesArtifactTypeForSameNameEntries(t *testing.T) {
	model := newRootModel(nil, true)
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	model.dashboard.cache = &frontend.CacheStatusResponse{
		Releases: struct {
			Directory string
			Entries   []dto.ReleaseCacheStatus
			Error     string
		}{
			Entries: []dto.ReleaseCacheStatus{
				{Project: "demo", Name: "keystone", ArtifactType: dto.ArtifactSnap, LastUpdated: now},
				{Project: "demo", Name: "keystone", ArtifactType: dto.ArtifactCharm, LastUpdated: now.Add(-time.Minute)},
			},
		},
	}

	rendered := model.renderDashboardReleases()
	if !strings.Contains(rendered, "snap  keystone") || !strings.Contains(rendered, "charm  keystone") {
		t.Fatalf("renderDashboardReleases() = %q, want type-qualified duplicate names", rendered)
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
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(rootModel)
	if model.overlay != overlayPackageFilters {
		t.Fatalf("overlay = %v, want package filters", model.overlay)
	}

	model.overlay = overlayNone
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(rootModel)
	if model.overlay != overlayBugFilters {
		t.Fatalf("overlay = %v, want bug filters", model.overlay)
	}

	model.overlay = overlayNone
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(rootModel)
	if model.overlay != overlayReviewFilters {
		t.Fatalf("overlay = %v, want review filters", model.overlay)
	}

	model.overlay = overlayNone
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(rootModel)
	if model.overlay != overlayCommitFilters {
		t.Fatalf("overlay = %v, want commit filters", model.overlay)
	}

	model.overlay = overlayNone
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")})
	model = next.(rootModel)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = next.(rootModel)
	if model.overlay != overlayProjectFilters {
		t.Fatalf("overlay = %v, want project filters", model.overlay)
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

func TestPackagesAndCommitsSubmodeNavigation(t *testing.T) {
	model := newRootModel(nil, true)
	model.activeView = viewPackages

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	model = next.(rootModel)
	if model.packages.filters.mode != packageModeDiff {
		t.Fatalf("package mode after ] = %v, want diff", model.packages.filters.mode)
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	model = next.(rootModel)
	if model.packages.filters.mode != packageModeInventory {
		t.Fatalf("package mode after [ = %v, want inventory", model.packages.filters.mode)
	}

	model.activeView = viewCommits
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	model = next.(rootModel)
	if model.commits.filters.mode != commitModeTrack {
		t.Fatalf("commit mode after ] = %v, want track", model.commits.filters.mode)
	}
}

func TestPromptRenderingForPackagesDiffAndCommitTrack(t *testing.T) {
	model := newRootModel(nil, true)
	model.width = 120
	model.height = 40

	model.packages.filters.mode = packageModeDiff
	model.packages.prompt = "Select a package set in filters to load diff results."
	if rendered := model.renderPackages(); !strings.Contains(rendered, "Select a package set") {
		t.Fatalf("renderPackages missing diff prompt:\n%s", rendered)
	}

	model.commits.filters.mode = commitModeTrack
	model.commits.prompt = "Enter a bug ID in filters to track matching commits."
	if rendered := model.renderCommits(); !strings.Contains(rendered, "Enter a bug ID") {
		t.Fatalf("renderCommits missing track prompt:\n%s", rendered)
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
