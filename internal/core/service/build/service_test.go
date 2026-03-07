// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// mockRecipeBuilder implements port.RecipeBuilder for testing.
type mockRecipeBuilder struct {
	artifactType dto.ArtifactType
	recipes      map[string]*dto.Recipe       // name → recipe
	builds       map[string][]dto.Build       // recipe SelfLink → builds
	buildReqs    map[string]*dto.BuildRequest // recipe SelfLink → request
	fileURLs     map[string][]string          // build SelfLink → file URLs
	ownerRecipes []*dto.Recipe                // ListRecipesByOwner result
	createErr    error
	requestErr   error
	listErr      error
	retryErr     error
	ownerListErr error
	retried      []string // tracks retried build self links
}

func (m *mockRecipeBuilder) ArtifactType() dto.ArtifactType { return m.artifactType }

func (m *mockRecipeBuilder) GetRecipe(_ context.Context, _, _, name string) (*dto.Recipe, error) {
	r, ok := m.recipes[name]
	if !ok {
		return nil, fmt.Errorf("recipe %q not found", name)
	}
	return r, nil
}

func (m *mockRecipeBuilder) CreateRecipe(_ context.Context, opts dto.CreateRecipeOpts) (*dto.Recipe, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	r := &dto.Recipe{
		Name:     opts.Name,
		SelfLink: "/~" + opts.Owner + "/" + opts.Project + "/+recipe/" + opts.Name,
	}
	if m.recipes == nil {
		m.recipes = make(map[string]*dto.Recipe)
	}
	m.recipes[opts.Name] = r
	return r, nil
}

func (m *mockRecipeBuilder) DeleteRecipe(_ context.Context, _ string) error { return nil }

func (m *mockRecipeBuilder) RequestBuilds(_ context.Context, recipe *dto.Recipe, _ dto.RequestBuildsOpts) (*dto.BuildRequest, error) {
	if m.requestErr != nil {
		return nil, m.requestErr
	}
	if br, ok := m.buildReqs[recipe.SelfLink]; ok {
		return br, nil
	}
	return &dto.BuildRequest{SelfLink: recipe.SelfLink + "/+request/1"}, nil
}

func (m *mockRecipeBuilder) ListBuilds(_ context.Context, recipe *dto.Recipe) ([]dto.Build, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.builds[recipe.SelfLink], nil
}

func (m *mockRecipeBuilder) RetryBuild(_ context.Context, buildSelfLink string) error {
	m.retried = append(m.retried, buildSelfLink)
	return m.retryErr
}

func (m *mockRecipeBuilder) CancelBuild(_ context.Context, _ string) error { return nil }

func (m *mockRecipeBuilder) GetBuildFileURLs(_ context.Context, buildSelfLink string) ([]string, error) {
	return m.fileURLs[buildSelfLink], nil
}

func (m *mockRecipeBuilder) ListRecipesByOwner(_ context.Context, _ string) ([]*dto.Recipe, error) {
	if m.ownerListErr != nil {
		return nil, m.ownerListErr
	}
	return m.ownerRecipes, nil
}

// mockRepoManager implements port.RepoManager for testing.
type mockRepoManager struct {
	currentUser   string
	project       string
	repoSelfLink  string
	gitSSHURL     string
	refSelfLink   string
	defaultBranch string
	createErr     error
	repoErr       error
	refErr        error
	defaultErr    error
}

func (m *mockRepoManager) GetCurrentUser(_ context.Context) (string, error) {
	if m.currentUser != "" {
		return m.currentUser, nil
	}
	return "test-user", nil
}

func (m *mockRepoManager) GetDefaultRepo(_ context.Context, projectName string) (string, string, error) {
	if m.defaultErr != nil {
		return "", "", m.defaultErr
	}
	link := m.repoSelfLink
	if link == "" {
		link = "https://api.launchpad.net/devel/~owner/" + projectName + "/+git/" + projectName
	}
	branch := m.defaultBranch
	if branch == "" {
		branch = "main"
	}
	return link, branch, nil
}

func (m *mockRepoManager) GetOrCreateProject(_ context.Context, _ string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return m.project, nil
}

func (m *mockRepoManager) GetOrCreateRepo(_ context.Context, _, _, _ string) (string, string, error) {
	if m.repoErr != nil {
		return "", "", m.repoErr
	}
	return m.repoSelfLink, m.gitSSHURL, nil
}

func (m *mockRepoManager) GetGitRef(_ context.Context, _, _ string) (string, error) {
	if m.refErr != nil {
		return "", m.refErr
	}
	return m.refSelfLink, nil
}

func (m *mockRepoManager) WaitForGitRef(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	if m.refErr != nil {
		return "", m.refErr
	}
	return m.refSelfLink, nil
}

// mockStrategy implements ArtifactStrategy for testing.
type mockStrategy struct {
	artifactType dto.ArtifactType
}

func (m *mockStrategy) ArtifactType() dto.ArtifactType            { return m.artifactType }
func (m *mockStrategy) MetadataFileName() string                  { return "rockcraft.yaml" }
func (m *mockStrategy) BuildPath(name string) string              { return "rocks/" + name }
func (m *mockStrategy) ParsePlatforms(_ []byte) ([]string, error) { return []string{"amd64"}, nil }
func (m *mockStrategy) TempRecipeName(name, sha, prefix string) string {
	return prefix + "-" + sha[:8] + "-" + name
}
func (m *mockStrategy) DiscoverRecipes(_ string) ([]string, error) {
	return []string{"discovered-recipe"}, nil
}
func (m *mockStrategy) OfficialRecipeName(name, series, devFocus string) string {
	if series == devFocus {
		return name
	}
	return name + "-" + series
}
func (m *mockStrategy) BranchForSeries(series, devFocus, defaultBranch string) string {
	if series == devFocus {
		return defaultBranch
	}
	return "stable/" + series
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Trigger tests
// ---------------------------------------------------------------------------

func TestTrigger_RemoteMode_RequestBuilds(t *testing.T) {
	recipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": recipe},
		builds:  map[string][]dto.Build{"/recipe/keystone": {}},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	if len(result.RecipeResults) != 1 {
		t.Fatalf("expected 1 recipe result, got %d", len(result.RecipeResults))
	}
	rr := result.RecipeResults[0]
	if rr.Action != ActionRequestBuilds {
		t.Errorf("Action = %d, want ActionRequestBuilds (%d)", rr.Action, ActionRequestBuilds)
	}
	if rr.BuildRequest == nil {
		t.Error("expected BuildRequest to be set")
	}
}

func TestTrigger_RemoteMode_AllSucceeded(t *testing.T) {
	recipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": recipe},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: dto.BuildSucceeded, Arch: "amd64", SelfLink: "/build/1"},
				{Recipe: "keystone", State: dto.BuildSucceeded, Arch: "arm64", SelfLink: "/build/2"},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	rr := result.RecipeResults[0]
	if rr.Action != ActionDownload {
		t.Errorf("Action = %d, want ActionDownload (%d)", rr.Action, ActionDownload)
	}
	if len(rr.Builds) != 2 {
		t.Errorf("expected 2 builds, got %d", len(rr.Builds))
	}
}

func TestTrigger_RemoteMode_RetryFailed(t *testing.T) {
	recipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": recipe},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: dto.BuildSucceeded, Arch: "amd64", SelfLink: "/build/1"},
				{Recipe: "keystone", State: dto.BuildFailed, Arch: "arm64", SelfLink: "/build/2", CanRetry: true},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	rr := result.RecipeResults[0]
	// executeAction converts ActionRetryFailed → ActionMonitor after retrying
	if rr.Action != ActionMonitor {
		t.Errorf("Action = %d, want ActionMonitor (%d)", rr.Action, ActionMonitor)
	}
	if len(builder.retried) != 1 || builder.retried[0] != "/build/2" {
		t.Errorf("expected retry of /build/2, got %v", builder.retried)
	}
}

func TestTrigger_RemoteMode_MonitorActive(t *testing.T) {
	recipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": recipe},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: dto.BuildBuilding, Arch: "amd64", SelfLink: "/build/1"},
				{Recipe: "keystone", State: dto.BuildPending, Arch: "arm64", SelfLink: "/build/2"},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	rr := result.RecipeResults[0]
	if rr.Action != ActionMonitor {
		t.Errorf("Action = %d, want ActionMonitor (%d)", rr.Action, ActionMonitor)
	}
	if len(rr.Builds) != 2 {
		t.Errorf("expected 2 builds, got %d", len(rr.Builds))
	}
}

func TestTrigger_RemoteMode_CreateRecipe(t *testing.T) {
	// Recipe doesn't exist → in remote mode, should error (create only in local mode).
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	rr := result.RecipeResults[0]
	if rr.Error == nil {
		t.Fatal("expected error for create recipe in remote mode")
	}
}

func TestTrigger_PreResolvedRefs_FullPipeline(t *testing.T) {
	// Simulates the CLI passing pre-resolved LP resources (as in local mode).
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	tempName := "tmp-abc12345-keystone"
	result, err := svc.Trigger(context.Background(), "sunbeam", []string{tempName}, TriggerOpts{
		Owner: "test-user",
		Prepared: &dto.PreparedBuildSource{
			Backend:       dto.PreparedBuildBackendLaunchpad,
			TargetProject: "test-project",
			Repository:    "/repo/sunbeam",
			Recipes: map[string]dto.PreparedBuildRecipe{
				tempName: {SourceRef: "/ref/abc12345", BuildPath: "rocks/keystone"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	if len(result.RecipeResults) != 1 {
		t.Fatalf("expected 1 recipe result, got %d", len(result.RecipeResults))
	}

	rr := result.RecipeResults[0]
	if rr.Error != nil {
		t.Errorf("unexpected error: %v", rr.Error)
	}
	if rr.BuildRequest == nil {
		t.Error("expected BuildRequest to be set after create+request")
	}

	if _, ok := builder.recipes[tempName]; !ok {
		t.Errorf("expected recipe %q to be created, got keys: %v", tempName, recipeKeys(builder.recipes))
	}
}

func TestTrigger_MultipleRecipes(t *testing.T) {
	rKeystone := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	rNova := &dto.Recipe{Name: "nova", SelfLink: "/recipe/nova"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": rKeystone, "nova": rNova},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {},
			"/recipe/nova":     {{Recipe: "nova", State: dto.BuildSucceeded, SelfLink: "/build/n1"}},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone", "nova"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	if len(result.RecipeResults) != 2 {
		t.Fatalf("expected 2 recipe results, got %d", len(result.RecipeResults))
	}

	actions := map[string]RecipeAction{}
	for _, rr := range result.RecipeResults {
		actions[rr.Name] = rr.Action
	}
	if actions["keystone"] != ActionRequestBuilds {
		t.Errorf("keystone action = %d, want ActionRequestBuilds", actions["keystone"])
	}
	if actions["nova"] != ActionDownload {
		t.Errorf("nova action = %d, want ActionDownload", actions["nova"])
	}
}

func TestTrigger_UnknownProject(t *testing.T) {
	svc := NewService(map[string]ProjectBuilder{}, nil, testLogger())

	_, err := svc.Trigger(context.Background(), "nonexistent", nil, TriggerOpts{})
	if err == nil {
		t.Fatal("expected error for unknown project")
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestList_ActiveOnly(t *testing.T) {
	now := time.Now()
	recipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": recipe},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: dto.BuildBuilding, SelfLink: "/b/1", CreatedAt: now},
				{Recipe: "keystone", State: dto.BuildSucceeded, SelfLink: "/b/2", CreatedAt: now.Add(-time.Hour)},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	builds, _, err := svc.List(context.Background(), ListOpts{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Default (All=false) should return only active builds.
	if len(builds) != 1 {
		t.Fatalf("expected 1 active build, got %d", len(builds))
	}
	if builds[0].State != dto.BuildBuilding {
		t.Errorf("State = %v, want BuildBuilding", builds[0].State)
	}
}

func TestList_AllBuilds(t *testing.T) {
	now := time.Now()
	recipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": recipe},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: dto.BuildBuilding, SelfLink: "/b/1", CreatedAt: now},
				{Recipe: "keystone", State: dto.BuildSucceeded, SelfLink: "/b/2", CreatedAt: now.Add(-time.Hour)},
				{Recipe: "keystone", State: dto.BuildFailed, SelfLink: "/b/3", CreatedAt: now.Add(-2 * time.Hour)},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	builds, _, err := svc.List(context.Background(), ListOpts{All: true})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(builds) != 3 {
		t.Errorf("expected 3 builds, got %d", len(builds))
	}
}

func TestList_ProjectFilter(t *testing.T) {
	rA := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	rB := &dto.Recipe{Name: "nova", SelfLink: "/recipe/nova"}
	builderA := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": rA},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {{Recipe: "keystone", State: dto.BuildBuilding, SelfLink: "/b/a1"}},
		},
	}
	builderB := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"nova": rB},
		builds: map[string][]dto.Build{
			"/recipe/nova": {{Recipe: "nova", State: dto.BuildBuilding, SelfLink: "/b/b1"}},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"projA": {Builder: builderA, Owner: "team", Project: "projA", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
			"projB": {Builder: builderB, Owner: "team", Project: "projB", Artifacts: []string{"nova"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	builds, _, err := svc.List(context.Background(), ListOpts{Projects: []string{"projA"}})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(builds) != 1 {
		t.Fatalf("expected 1 build from projA, got %d", len(builds))
	}
	if builds[0].Project != "projA" {
		t.Errorf("Project = %q, want projA", builds[0].Project)
	}
}

func TestList_GracefulDegradation(t *testing.T) {
	goodRecipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	goodBuilder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": goodRecipe},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {{Recipe: "keystone", State: dto.BuildBuilding, SelfLink: "/b/1"}},
		},
	}
	badBuilder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{}, // GetRecipe will fail
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"good": {Builder: goodBuilder, Owner: "team", Project: "good", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
			"bad":  {Builder: badBuilder, Owner: "team", Project: "bad", Artifacts: []string{"missing"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	builds, results, err := svc.List(context.Background(), ListOpts{})
	if err != nil {
		t.Fatalf("List() should not return top-level error: %v", err)
	}

	if len(builds) != 1 {
		t.Errorf("expected 1 build from good project, got %d", len(builds))
	}

	// Both projects should have results (no top-level error for bad project).
	if len(results) != 2 {
		t.Errorf("expected 2 project results, got %d", len(results))
	}
}

func TestList_Sorting(t *testing.T) {
	now := time.Now()
	recipe := &dto.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{"keystone": recipe},
		builds: map[string][]dto.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: dto.BuildBuilding, SelfLink: "/b/old", CreatedAt: now.Add(-2 * time.Hour)},
				{Recipe: "keystone", State: dto.BuildBuilding, SelfLink: "/b/new", CreatedAt: now},
				{Recipe: "keystone", State: dto.BuildBuilding, SelfLink: "/b/mid", CreatedAt: now.Add(-1 * time.Hour)},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	builds, _, err := svc.List(context.Background(), ListOpts{All: true})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(builds) != 3 {
		t.Fatalf("expected 3 builds, got %d", len(builds))
	}

	// Should be sorted by CreatedAt descending.
	for i := 1; i < len(builds); i++ {
		if builds[i].CreatedAt.After(builds[i-1].CreatedAt) {
			t.Errorf("builds[%d].CreatedAt (%v) > builds[%d].CreatedAt (%v) — not sorted descending",
				i, builds[i].CreatedAt, i-1, builds[i-1].CreatedAt)
		}
	}
}

// ---------------------------------------------------------------------------
// Trigger: local mode resolves owner from Me()
// ---------------------------------------------------------------------------

func TestTrigger_PreResolved_OwnerOverride(t *testing.T) {
	// Project has NO owner configured; pre-resolved opts provide owner.
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "", Project: "sunbeam", Artifacts: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, testLogger(),
	)

	tempName := "tmp-abc12345-keystone"
	result, err := svc.Trigger(context.Background(), "sunbeam", []string{tempName}, TriggerOpts{
		Owner: "test-user",
		Prepared: &dto.PreparedBuildSource{
			Backend:       dto.PreparedBuildBackendLaunchpad,
			TargetProject: "test-project",
			Repository:    "/repo/sunbeam",
			Recipes: map[string]dto.PreparedBuildRecipe{
				tempName: {SourceRef: "/ref/abc12345", BuildPath: "rocks/keystone"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	if len(result.RecipeResults) != 1 {
		t.Fatalf("expected 1 recipe result, got %d", len(result.RecipeResults))
	}
	rr := result.RecipeResults[0]
	if rr.Error != nil {
		t.Errorf("unexpected error: %v", rr.Error)
	}
}

// ---------------------------------------------------------------------------
// Trigger: remote mode with official codehosting expands series
// ---------------------------------------------------------------------------

func TestTrigger_RemoteMode_UsesOfficialRepo(t *testing.T) {
	// All recipes start as not-found so the service tries to create them.
	// The mock CreateRecipe fills them in.
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{},
	}
	repoMgr := &mockRepoManager{
		repoSelfLink:  "/repo/rocks",
		defaultBranch: "main",
		refSelfLink:   "/ref/heads/main",
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {
				Builder:             builder,
				Owner:               "team",
				Project:             "ubuntu-openstack-rocks",
				Artifacts:           []string{"nova-consolidated"},
				Series:              []string{"2024.1", "2025.1"},
				DevFocus:            "2025.1",
				OfficialCodehosting: true,
				Strategy:            &mockStrategy{},
			},
		},
		repoMgr, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", []string{"nova-consolidated"}, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	// Should expand into 2 recipes: nova-consolidated-2024.1 and nova-consolidated
	if len(result.RecipeResults) != 2 {
		t.Fatalf("expected 2 recipe results, got %d", len(result.RecipeResults))
	}

	names := map[string]bool{}
	for _, rr := range result.RecipeResults {
		names[rr.Name] = true
	}
	if !names["nova-consolidated"] {
		t.Error("expected recipe 'nova-consolidated' (dev focus)")
	}
	if !names["nova-consolidated-2024.1"] {
		t.Error("expected recipe 'nova-consolidated-2024.1' (non-dev series)")
	}
}

// ---------------------------------------------------------------------------
// Trigger: remote mode without official_codehosting creates recipe error
// ---------------------------------------------------------------------------

func TestTrigger_RemoteMode_FailsWithoutOfficialCodehosting(t *testing.T) {
	// Without OfficialCodehosting=true, remote mode has no git info.
	// Recipes that don't exist will fail to create (no repoSelfLink).
	builder := &mockRecipeBuilder{
		recipes: map[string]*dto.Recipe{},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {
				Builder:             builder,
				Owner:               "team",
				Project:             "sunbeam",
				Artifacts:           []string{"keystone"},
				OfficialCodehosting: false,
				Strategy:            &mockStrategy{},
			},
		},
		nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	if len(result.RecipeResults) != 1 {
		t.Fatalf("expected 1 recipe result, got %d", len(result.RecipeResults))
	}
	rr := result.RecipeResults[0]
	if rr.Error == nil {
		t.Fatal("expected error when recipe not found and OfficialCodehosting is false")
	}
}

// ---------------------------------------------------------------------------
// ProjectBuilder tests
// ---------------------------------------------------------------------------

func TestProjectBuilder_RecipeProject(t *testing.T) {
	pb := ProjectBuilder{Project: "ubuntu-openstack-rocks"}
	if got := pb.RecipeProject(); got != "ubuntu-openstack-rocks" {
		t.Errorf("RecipeProject() = %q, want %q", got, "ubuntu-openstack-rocks")
	}

	pb.LPProject = "custom-lp-project"
	if got := pb.RecipeProject(); got != "custom-lp-project" {
		t.Errorf("RecipeProject() = %q, want %q", got, "custom-lp-project")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func recipeKeys(m map[string]*dto.Recipe) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
