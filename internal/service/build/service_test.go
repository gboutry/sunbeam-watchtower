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

	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// mockRecipeBuilder implements port.RecipeBuilder for testing.
type mockRecipeBuilder struct {
	artifactType port.ArtifactType
	recipes      map[string]*port.Recipe      // name → recipe
	builds       map[string][]port.Build      // recipe SelfLink → builds
	buildReqs    map[string]*port.BuildRequest // recipe SelfLink → request
	fileURLs     map[string][]string           // build SelfLink → file URLs
	createErr    error
	requestErr   error
	listErr      error
	retryErr     error
	retried      []string // tracks retried build self links
}

func (m *mockRecipeBuilder) ArtifactType() port.ArtifactType { return m.artifactType }

func (m *mockRecipeBuilder) GetRecipe(_ context.Context, _, _, name string) (*port.Recipe, error) {
	r, ok := m.recipes[name]
	if !ok {
		return nil, fmt.Errorf("recipe %q not found", name)
	}
	return r, nil
}

func (m *mockRecipeBuilder) CreateRecipe(_ context.Context, opts port.CreateRecipeOpts) (*port.Recipe, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	r := &port.Recipe{
		Name:     opts.Name,
		SelfLink: "/~" + opts.Owner + "/" + opts.Project + "/+recipe/" + opts.Name,
	}
	if m.recipes == nil {
		m.recipes = make(map[string]*port.Recipe)
	}
	m.recipes[opts.Name] = r
	return r, nil
}

func (m *mockRecipeBuilder) DeleteRecipe(_ context.Context, _ string) error { return nil }

func (m *mockRecipeBuilder) RequestBuilds(_ context.Context, recipe *port.Recipe, _ port.RequestBuildsOpts) (*port.BuildRequest, error) {
	if m.requestErr != nil {
		return nil, m.requestErr
	}
	if br, ok := m.buildReqs[recipe.SelfLink]; ok {
		return br, nil
	}
	return &port.BuildRequest{SelfLink: recipe.SelfLink + "/+request/1"}, nil
}

func (m *mockRecipeBuilder) ListBuilds(_ context.Context, recipe *port.Recipe) ([]port.Build, error) {
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

// mockRepoManager implements port.RepoManager for testing.
type mockRepoManager struct {
	project      string
	repoSelfLink string
	gitSSHURL    string
	refSelfLink  string
	createErr    error
	repoErr      error
	refErr       error
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

// mockGitClient implements port.GitClient for testing.
type mockGitClient struct {
	isRepo      bool
	headSHA     string
	uncommitted bool
	pushErr     error
}

func (m *mockGitClient) IsRepo(_ string) bool                        { return m.isRepo }
func (m *mockGitClient) HeadSHA(_ string) (string, error)            { return m.headSHA, nil }
func (m *mockGitClient) HasUncommittedChanges(_ string) (bool, error) { return m.uncommitted, nil }
func (m *mockGitClient) Push(_, _, _, _ string, _ bool) error         { return m.pushErr }
func (m *mockGitClient) AddRemote(_, _, _ string) error               { return nil }
func (m *mockGitClient) RemoveRemote(_, _ string) error               { return nil }

// mockStrategy implements ArtifactStrategy for testing.
type mockStrategy struct {
	artifactType port.ArtifactType
}

func (m *mockStrategy) ArtifactType() port.ArtifactType              { return m.artifactType }
func (m *mockStrategy) MetadataFileName() string                     { return "rockcraft.yaml" }
func (m *mockStrategy) BuildPath(name string) string                 { return "rocks/" + name }
func (m *mockStrategy) ParsePlatforms(_ []byte) ([]string, error)    { return []string{"amd64"}, nil }
func (m *mockStrategy) TempRecipeName(name, sha, prefix string) string {
	return prefix + "-" + sha[:8] + "-" + name
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// Trigger tests
// ---------------------------------------------------------------------------

func TestTrigger_RemoteMode_RequestBuilds(t *testing.T) {
	recipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": recipe},
		builds:  map[string][]port.Build{"/recipe/keystone": {}},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{Source: "remote"})
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
	recipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": recipe},
		builds: map[string][]port.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: port.BuildSucceeded, Arch: "amd64", SelfLink: "/build/1"},
				{Recipe: "keystone", State: port.BuildSucceeded, Arch: "arm64", SelfLink: "/build/2"},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{Source: "remote"})
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
	recipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": recipe},
		builds: map[string][]port.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: port.BuildSucceeded, Arch: "amd64", SelfLink: "/build/1"},
				{Recipe: "keystone", State: port.BuildFailed, Arch: "arm64", SelfLink: "/build/2", CanRetry: true},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{Source: "remote"})
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
	recipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": recipe},
		builds: map[string][]port.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: port.BuildBuilding, Arch: "amd64", SelfLink: "/build/1"},
				{Recipe: "keystone", State: port.BuildPending, Arch: "arm64", SelfLink: "/build/2"},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{Source: "remote"})
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
		recipes: map[string]*port.Recipe{},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{Source: "remote"})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	rr := result.RecipeResults[0]
	if rr.Error == nil {
		t.Fatal("expected error for create recipe in remote mode")
	}
}

func TestTrigger_LocalMode_FullPipeline(t *testing.T) {
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{},
	}
	repoMgr := &mockRepoManager{
		project:      "test-project",
		repoSelfLink: "/repo/sunbeam",
		gitSSHURL:    "git+ssh://lp/repo",
		refSelfLink:  "/ref/abc12345",
	}
	gitCli := &mockGitClient{
		isRepo:  true,
		headSHA: "abc12345deadbeef",
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		repoMgr, gitCli, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{
		Source:    "local",
		LocalPath: "/tmp/repo",
		Prefix:    "tmp",
	})
	if err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}

	if len(result.RecipeResults) != 1 {
		t.Fatalf("expected 1 recipe result, got %d", len(result.RecipeResults))
	}

	rr := result.RecipeResults[0]
	// Local mode creates recipe then requests builds.
	if rr.Error != nil {
		t.Errorf("unexpected error: %v", rr.Error)
	}
	if rr.BuildRequest == nil {
		t.Error("expected BuildRequest to be set after local create+request")
	}

	// The temp recipe name should have been created via strategy.
	expectedTempName := "tmp-abc12345-keystone"
	if _, ok := builder.recipes[expectedTempName]; !ok {
		t.Errorf("expected recipe %q to be created, got keys: %v", expectedTempName, recipeKeys(builder.recipes))
	}
}

func TestTrigger_MultipleRecipes(t *testing.T) {
	rKeystone := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	rNova := &port.Recipe{Name: "nova", SelfLink: "/recipe/nova"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": rKeystone, "nova": rNova},
		builds: map[string][]port.Build{
			"/recipe/keystone": {},
			"/recipe/nova":     {{Recipe: "nova", State: port.BuildSucceeded, SelfLink: "/build/n1"}},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone", "nova"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
	)

	result, err := svc.Trigger(context.Background(), "sunbeam", nil, TriggerOpts{Source: "remote"})
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
	svc := NewService(map[string]ProjectBuilder{}, nil, nil, testLogger())

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
	recipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": recipe},
		builds: map[string][]port.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: port.BuildBuilding, SelfLink: "/b/1", CreatedAt: now},
				{Recipe: "keystone", State: port.BuildSucceeded, SelfLink: "/b/2", CreatedAt: now.Add(-time.Hour)},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
	)

	builds, _, err := svc.List(context.Background(), ListOpts{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Default (All=false) should return only active builds.
	if len(builds) != 1 {
		t.Fatalf("expected 1 active build, got %d", len(builds))
	}
	if builds[0].State != port.BuildBuilding {
		t.Errorf("State = %v, want BuildBuilding", builds[0].State)
	}
}

func TestList_AllBuilds(t *testing.T) {
	now := time.Now()
	recipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": recipe},
		builds: map[string][]port.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: port.BuildBuilding, SelfLink: "/b/1", CreatedAt: now},
				{Recipe: "keystone", State: port.BuildSucceeded, SelfLink: "/b/2", CreatedAt: now.Add(-time.Hour)},
				{Recipe: "keystone", State: port.BuildFailed, SelfLink: "/b/3", CreatedAt: now.Add(-2 * time.Hour)},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
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
	rA := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	rB := &port.Recipe{Name: "nova", SelfLink: "/recipe/nova"}
	builderA := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": rA},
		builds: map[string][]port.Build{
			"/recipe/keystone": {{Recipe: "keystone", State: port.BuildBuilding, SelfLink: "/b/a1"}},
		},
	}
	builderB := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"nova": rB},
		builds: map[string][]port.Build{
			"/recipe/nova": {{Recipe: "nova", State: port.BuildBuilding, SelfLink: "/b/b1"}},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"projA": {Builder: builderA, Owner: "team", Project: "projA", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
			"projB": {Builder: builderB, Owner: "team", Project: "projB", Recipes: []string{"nova"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
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
	goodRecipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	goodBuilder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": goodRecipe},
		builds: map[string][]port.Build{
			"/recipe/keystone": {{Recipe: "keystone", State: port.BuildBuilding, SelfLink: "/b/1"}},
		},
	}
	badBuilder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{}, // GetRecipe will fail
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"good": {Builder: goodBuilder, Owner: "team", Project: "good", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
			"bad":  {Builder: badBuilder, Owner: "team", Project: "bad", Recipes: []string{"missing"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
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
	recipe := &port.Recipe{Name: "keystone", SelfLink: "/recipe/keystone"}
	builder := &mockRecipeBuilder{
		recipes: map[string]*port.Recipe{"keystone": recipe},
		builds: map[string][]port.Build{
			"/recipe/keystone": {
				{Recipe: "keystone", State: port.BuildBuilding, SelfLink: "/b/old", CreatedAt: now.Add(-2 * time.Hour)},
				{Recipe: "keystone", State: port.BuildBuilding, SelfLink: "/b/new", CreatedAt: now},
				{Recipe: "keystone", State: port.BuildBuilding, SelfLink: "/b/mid", CreatedAt: now.Add(-1 * time.Hour)},
			},
		},
	}

	svc := NewService(
		map[string]ProjectBuilder{
			"sunbeam": {Builder: builder, Owner: "team", Project: "sunbeam", Recipes: []string{"keystone"}, Strategy: &mockStrategy{}},
		},
		nil, nil, testLogger(),
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
// helpers
// ---------------------------------------------------------------------------

func recipeKeys(m map[string]*port.Recipe) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
