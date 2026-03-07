// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestBuildWorkflowTriggerAsync(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/builds/trigger/async" {
			t.Fatalf("path = %q, want async trigger path", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(dto.OperationJob{
			ID:    "op-build-1",
			Kind:  dto.OperationKindBuildTrigger,
			State: dto.OperationStateQueued,
		})
	}))
	defer ts.Close()

	workflow := NewBuildWorkflow(NewClientTransport(client.NewClient(ts.URL)), nil)
	got, err := workflow.Trigger(context.Background(), BuildTriggerRequest{
		Async:   true,
		Project: "demo",
	})
	if err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}
	if got.Job == nil || got.Job.ID != "op-build-1" {
		t.Fatalf("Job = %+v, want op-build-1", got.Job)
	}
}

func TestBuildWorkflowTriggerLocalDownload(t *testing.T) {
	var triggerBody client.BuildsTriggerOptions
	var downloadBody client.BuildsDownloadOptions

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/builds/trigger":
			if err := json.NewDecoder(r.Body).Decode(&triggerBody); err != nil {
				t.Fatalf("Decode(trigger) error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(dto.BuildTriggerResult{
				Project: "demo",
				RecipeResults: []dto.BuildRecipeResult{{
					Name: "tmp-build-01234567-keystone",
					Builds: []dto.Build{{
						Project: "demo",
						Recipe:  "tmp-build-01234567-keystone",
						State:   dto.BuildSucceeded,
					}},
				}},
			})
		case "/api/v1/builds/download":
			if err := json.NewDecoder(r.Body).Decode(&downloadBody); err != nil {
				t.Fatalf("Decode(download) error = %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

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

	workflow := NewBuildWorkflow(NewClientTransport(client.NewClient(ts.URL)), preparer)
	got, err := workflow.Trigger(context.Background(), BuildTriggerRequest{
		Source:       "local",
		LocalPath:    "/tmp/demo",
		Download:     true,
		ArtifactsDir: "/tmp/artifacts",
		Project:      "demo",
		Prefix:       "tmp-build",
	})
	if err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}
	if got.Result == nil {
		t.Fatal("Result = nil, want response")
	}
	if triggerBody.Prepared == nil || triggerBody.Prepared.TargetProject != "lp-project" {
		t.Fatalf("Prepared trigger body = %+v", triggerBody.Prepared)
	}
	if len(downloadBody.Artifacts) != 1 || !strings.HasPrefix(downloadBody.Artifacts[0], "tmp-build-01234567-") {
		t.Fatalf("download artifacts = %+v", downloadBody.Artifacts)
	}
	if downloadBody.ArtifactsDir != "/tmp/artifacts" {
		t.Fatalf("ArtifactsDir = %q, want /tmp/artifacts", downloadBody.ArtifactsDir)
	}
}

func TestBuildWorkflowListLocal(t *testing.T) {
	var query string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"builds": []dto.Build{}})
	}))
	defer ts.Close()

	preparer := NewLocalBuildPreparer(
		nil,
		&fakeRepoManager{currentUser: "lp-user", project: "lp-project"},
		nil,
	)

	workflow := NewBuildWorkflow(NewClientTransport(client.NewClient(ts.URL)), preparer)
	_, err := workflow.List(context.Background(), BuildListRequest{
		Source:     "local",
		SHA:        "01234567",
		Prefix:     "tmp-build-",
		DefaultAll: true,
		Projects:   []string{"demo"},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if !strings.Contains(query, "all=true") || !strings.Contains(query, "recipe_prefix=tmp-build-01234567-") || !strings.Contains(query, "target_project=lp-project") {
		t.Fatalf("unexpected query: %q", query)
	}
}

func TestBuildWorkflowCleanup(t *testing.T) {
	var gotBody client.BuildsCleanupOptions

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/builds/cleanup" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(client.BuildsCleanupResult{
			Deleted: []string{"tmp-build-keystone", "tmp-build-glance"},
		})
	}))
	defer ts.Close()

	workflow := NewBuildWorkflow(NewClientTransport(client.NewClient(ts.URL)), nil)
	got, err := workflow.Cleanup(context.Background(), BuildCleanupRequest{
		Project: "keystone",
		Owner:   "team-a",
		Prefix:  "tmp-build",
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if gotBody.Project != "keystone" || gotBody.Owner != "team-a" || gotBody.Prefix != "tmp-build" || !gotBody.DryRun {
		t.Fatalf("cleanup body = %+v, want keystone/team-a/tmp-build dry-run", gotBody)
	}
	if len(got) != 2 || got[0] != "tmp-build-keystone" {
		t.Fatalf("Cleanup() = %+v, want deleted recipes", got)
	}
}
