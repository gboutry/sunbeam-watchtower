// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

type fakeGitClient struct {
	headSHA string
}

func (f *fakeGitClient) IsRepo(string) bool                         { return true }
func (f *fakeGitClient) HeadSHA(string) (string, error)             { return f.headSHA, nil }
func (f *fakeGitClient) HasUncommittedChanges(string) (bool, error) { return false, nil }
func (f *fakeGitClient) Push(string, string, string, string, bool) error {
	return nil
}
func (f *fakeGitClient) AddRemote(string, string, string) error { return nil }
func (f *fakeGitClient) RemoveRemote(string, string) error      { return nil }

type fakeRepoManager struct {
	currentUser  string
	project      string
	repoSelfLink string
	gitSSHURL    string
	refSelfLink  string
}

func (f *fakeRepoManager) GetCurrentUser(context.Context) (string, error) { return f.currentUser, nil }
func (f *fakeRepoManager) GetDefaultRepo(context.Context, string) (string, string, error) {
	return "", "", errors.New("not used")
}
func (f *fakeRepoManager) GetOrCreateProject(context.Context, string) (string, error) {
	return f.project, nil
}
func (f *fakeRepoManager) GetOrCreateRepo(context.Context, string, string, string) (string, string, error) {
	return f.repoSelfLink, f.gitSSHURL, nil
}
func (f *fakeRepoManager) GetGitRef(context.Context, string, string) (string, error) {
	return f.refSelfLink, nil
}
func (f *fakeRepoManager) WaitForGitRef(context.Context, string, string, time.Duration) (string, error) {
	return f.refSelfLink, nil
}

type fakeStrategy struct{}

func (f *fakeStrategy) ArtifactType() dto.ArtifactType                          { return dto.ArtifactRock }
func (f *fakeStrategy) MetadataFileName() string                                { return "rockcraft.yaml" }
func (f *fakeStrategy) BuildPath(name string) string                            { return "rocks/" + name }
func (f *fakeStrategy) ParsePlatforms([]byte) ([]string, error)                 { return []string{"amd64"}, nil }
func (f *fakeStrategy) DiscoverRecipes(string) ([]string, error)                { return []string{"keystone"}, nil }
func (f *fakeStrategy) OfficialRecipeName(name, series, devFocus string) string { return name }
func (f *fakeStrategy) BranchForSeries(series, devFocus, defaultBranch string) string {
	return defaultBranch
}
func (f *fakeStrategy) TempRecipeName(name, sha, prefix string) string {
	return prefix + "-" + sha[:8] + "-" + name
}

func TestLocalBuildPreparerPrepareTrigger(t *testing.T) {
	preparer := NewLocalBuildPreparer(
		&fakeGitClient{headSHA: "0123456789abcdef0123456789abcdef01234567"},
		&fakeRepoManager{
			currentUser:  "lp-user",
			project:      "lp-project",
			repoSelfLink: "https://api.launchpad.net/devel/~lp-user/lp-project/+git/demo",
			gitSSHURL:    "git+ssh://git.launchpad.net/~lp-user/lp-project/+git/demo",
			refSelfLink:  "https://api.launchpad.net/devel/~lp-user/lp-project/+git/demo/+ref/refs/heads/tmp-01234567",
		},
		map[string]build.ProjectBuilder{
			"demo": {
				Project:  "demo",
				Strategy: &fakeStrategy{},
			},
		},
	)

	got, err := preparer.PrepareTrigger(context.Background(), PreparedBuildTriggerRequest{
		Project: "demo",
		Prefix:  "tmp-build",
	}, "/tmp/demo")
	if err != nil {
		t.Fatalf("PrepareTrigger() error = %v", err)
	}

	if got.Owner != "lp-user" || got.Prepared == nil || got.Prepared.TargetProject != "lp-project" || got.Prepared.Repository == "" {
		t.Fatalf("unexpected prepared trigger: %+v", got)
	}
	if len(got.Artifacts) != 1 || got.Artifacts[0] != "tmp-build-01234567-keystone" {
		t.Fatalf("Artifacts = %v", got.Artifacts)
	}
	if got.Prepared.Recipes[got.Artifacts[0]].BuildPath != "rocks/keystone" {
		t.Fatalf("Recipes = %+v", got.Prepared.Recipes)
	}
	if got.Prepared.Recipes[got.Artifacts[0]].SourceRef == "" {
		t.Fatalf("Recipes = %+v", got.Prepared.Recipes)
	}
}

func TestLocalBuildPreparerPrepareTriggerRequiresAuth(t *testing.T) {
	preparer := NewLocalBuildPreparer(&fakeGitClient{headSHA: "0123456789abcdef0123456789abcdef01234567"}, nil, nil)

	_, err := preparer.PrepareTrigger(context.Background(), PreparedBuildTriggerRequest{
		Project: "demo",
		Prefix:  "tmp-build",
	}, "/tmp/demo")
	if !errors.Is(err, app.ErrLaunchpadAuthRequired) {
		t.Fatalf("PrepareTrigger() error = %v, want %v", err, app.ErrLaunchpadAuthRequired)
	}
}

func TestLocalBuildPreparerPrepareListByPrefix(t *testing.T) {
	preparer := NewLocalBuildPreparer(
		nil,
		&fakeRepoManager{currentUser: "lp-user", project: "lp-project"},
		nil,
	)

	got, err := preparer.PrepareListByPrefix(context.Background(), PreparedBuildListRequest{}, "tmp-build-01234567-")
	if err != nil {
		t.Fatalf("PrepareListByPrefix() error = %v", err)
	}
	if got.Owner != "lp-user" || got.TargetProject != "lp-project" || got.RecipePrefix != "tmp-build-01234567-" {
		t.Fatalf("unexpected list opts: %+v", got)
	}
}
