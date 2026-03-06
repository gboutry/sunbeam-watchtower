package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// --- Trigger builds ---

// BuildsTriggerInput holds the request body for triggering builds.
type BuildsTriggerInput struct {
	Body struct {
		Project   string   `json:"project" doc:"Project name"`
		Recipes   []string `json:"recipes,omitempty" doc:"Recipe names (empty = all configured)"`
		Source    string   `json:"source,omitempty" doc:"Build source (remote|local)" default:"remote"`
		Wait      bool     `json:"wait,omitempty" doc:"Wait for builds to complete"`
		Timeout   string   `json:"timeout,omitempty" doc:"Max wait time as Go duration (e.g. 5h)"`
		Owner     string   `json:"owner,omitempty" doc:"Override LP owner"`
		Prefix    string   `json:"prefix,omitempty" doc:"Temp recipe name prefix (local mode)"`
		LocalPath string   `json:"local_path,omitempty" doc:"Path to local git repo (local mode)"`
	}
}

// BuildsTriggerOutput is the response for triggering builds.
type BuildsTriggerOutput struct {
	Body dto.BuildTriggerResult
}

// --- List builds ---

// BuildsListInput holds query parameters for listing builds.
type BuildsListInput struct {
	Projects []string `query:"project" doc:"Filter by project name"`
	All      bool     `query:"all" doc:"Show all builds (not just active)"`
	State    string   `query:"state" doc:"Filter by state"`
}

// BuildsListOutput is the response for listing builds.
type BuildsListOutput struct {
	Body struct {
		Builds []dto.Build `json:"builds" doc:"Builds matching filters"`
	}
}

// --- Download builds ---

// BuildsDownloadInput holds the request body for downloading build artifacts.
type BuildsDownloadInput struct {
	Body struct {
		Project      string   `json:"project" doc:"Project name"`
		Recipes      []string `json:"recipes,omitempty" doc:"Recipe names (empty = all configured)"`
		ArtifactsDir string   `json:"artifacts_dir,omitempty" doc:"Output directory (default from config)"`
	}
}

// BuildsDownloadOutput is the response for downloading build artifacts.
type BuildsDownloadOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

// --- Cleanup builds ---

// BuildsCleanupInput holds the request body for cleaning up temporary recipes.
type BuildsCleanupInput struct {
	Body struct {
		Project string `json:"project,omitempty" doc:"Filter to specific project"`
		Owner   string `json:"owner,omitempty" doc:"LP owner"`
		Prefix  string `json:"prefix,omitempty" doc:"Recipe name prefix to match"`
		DryRun  bool   `json:"dry_run,omitempty" doc:"Show what would be deleted"`
	}
}

// BuildsCleanupOutput is the response for cleaning up temporary recipes.
type BuildsCleanupOutput struct {
	Body struct {
		Deleted []string `json:"deleted" doc:"Deleted recipe names"`
	}
}

// RegisterBuildsAPI registers the /api/v1/builds endpoints on the given huma API.
func RegisterBuildsAPI(api huma.API, application *app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "trigger-builds",
		Method:      http.MethodPost,
		Path:        "/api/v1/builds/trigger",
		Summary:     "Trigger builds for a project",
		Description: "Trigger builds for a project's recipes, optionally from a local git repo.",
		Tags:        []string{"builds"},
	}, func(ctx context.Context, input *BuildsTriggerInput) (*BuildsTriggerOutput, error) {
		svc, err := application.BuildService()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build build service", err)
		}

		var timeout time.Duration
		if input.Body.Timeout != "" {
			timeout, err = time.ParseDuration(input.Body.Timeout)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity("invalid timeout duration", err)
			}
		}

		triggerOpts := build.TriggerOpts{
			Source:    input.Body.Source,
			Wait:      input.Body.Wait,
			Timeout:   timeout,
			Owner:     input.Body.Owner,
			Prefix:    input.Body.Prefix,
			LocalPath: input.Body.LocalPath,
		}

		result, err := svc.Trigger(ctx, input.Body.Project, input.Body.Recipes, triggerOpts)
		if err != nil {
			return nil, huma.Error500InternalServerError("trigger failed", err)
		}

		out := &BuildsTriggerOutput{}
		out.Body = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-builds",
		Method:      http.MethodGet,
		Path:        "/api/v1/builds",
		Summary:     "List builds across projects",
		Description: "List builds across all configured projects, with optional filters.",
		Tags:        []string{"builds"},
	}, func(ctx context.Context, input *BuildsListInput) (*BuildsListOutput, error) {
		svc, err := application.BuildService()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build build service", err)
		}

		builds, _, err := svc.List(ctx, build.ListOpts{
			Projects: input.Projects,
			All:      input.All,
			State:    input.State,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list builds", err)
		}

		out := &BuildsListOutput{}
		out.Body.Builds = builds
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "download-builds",
		Method:      http.MethodPost,
		Path:        "/api/v1/builds/download",
		Summary:     "Download build artifacts",
		Description: "Download build artifacts for succeeded builds of the given recipes.",
		Tags:        []string{"builds"},
	}, func(ctx context.Context, input *BuildsDownloadInput) (*BuildsDownloadOutput, error) {
		svc, err := application.BuildService()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build build service", err)
		}

		artifactsDir := input.Body.ArtifactsDir
		if artifactsDir == "" {
			artifactsDir = application.Config.Build.ArtifactsDir
		}

		if err := svc.Download(ctx, input.Body.Project, input.Body.Recipes, artifactsDir); err != nil {
			return nil, huma.Error500InternalServerError("download failed", err)
		}

		out := &BuildsDownloadOutput{}
		out.Body.Status = "ok"
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "cleanup-builds",
		Method:      http.MethodPost,
		Path:        "/api/v1/builds/cleanup",
		Summary:     "Delete temporary build recipes",
		Description: "Delete temporary build recipes matching a prefix.",
		Tags:        []string{"builds"},
	}, func(ctx context.Context, input *BuildsCleanupInput) (*BuildsCleanupOutput, error) {
		svc, err := application.BuildService()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build build service", err)
		}

		cleanupOpts := build.CleanupOpts{
			Owner:  input.Body.Owner,
			Prefix: input.Body.Prefix,
			DryRun: input.Body.DryRun,
		}
		if input.Body.Project != "" {
			cleanupOpts.Projects = []string{input.Body.Project}
		}

		deleted, err := svc.Cleanup(ctx, cleanupOpts)
		if err != nil {
			return nil, huma.Error500InternalServerError("cleanup failed", err)
		}

		out := &BuildsCleanupOutput{}
		out.Body.Deleted = deleted
		return out, nil
	})
}
