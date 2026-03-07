package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// --- Trigger builds ---

// BuildsTriggerInput holds the request body for triggering builds.
type BuildsTriggerInput struct {
	Body struct {
		Project      string            `json:"project" doc:"Project name"`
		Artifacts    []string          `json:"artifacts,omitempty" required:"false" doc:"Artifact names to build (empty = all configured)"`
		Wait         bool              `json:"wait,omitempty" required:"false" doc:"Wait for builds to complete"`
		Timeout      string            `json:"timeout,omitempty" required:"false" doc:"Max wait time as Go duration (e.g. 5h)"`
		Owner        string            `json:"owner,omitempty" required:"false" doc:"Override LP owner"`
		Prefix       string            `json:"prefix,omitempty" required:"false" doc:"Temp recipe name prefix"`
		RepoSelfLink string            `json:"repo_self_link,omitempty" required:"false" doc:"LP git repo self_link (pre-resolved)"`
		GitRefLinks  map[string]string `json:"git_ref_links,omitempty" required:"false" doc:"Recipe name → git ref self_link (pre-resolved)"`
		BuildPaths   map[string]string `json:"build_paths,omitempty" required:"false" doc:"Recipe name → build path (pre-resolved)"`
		LPProject    string            `json:"lp_project,omitempty" required:"false" doc:"Override LP project name"`
	}
}

// BuildsTriggerOutput is the response for triggering builds.
type BuildsTriggerOutput struct {
	Body dto.BuildTriggerResult
}

// --- List builds ---

// BuildsListInput holds query parameters for listing builds.
type BuildsListInput struct {
	Projects     []string `query:"project" required:"false" doc:"Filter by project name"`
	All          bool     `query:"all" required:"false" doc:"Show all builds (not just active)"`
	State        string   `query:"state" doc:"Filter by state"`
	Owner        string   `query:"owner" doc:"Override LP owner"`
	LPProject    string   `query:"lp_project" doc:"Override LP project for recipe lookup"`
	RecipeNames  []string `query:"recipe" required:"false" doc:"Explicit recipe names (overrides project config)"`
	RecipePrefix string   `query:"recipe_prefix" doc:"Filter recipes by name prefix"`
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
		Artifacts    []string `json:"artifacts,omitempty" required:"false" doc:"Artifact names to download (empty = all)"`
		RecipePrefix string   `json:"recipe_prefix,omitempty" required:"false" doc:"Filter recipes by name prefix"`
		Owner        string   `json:"owner,omitempty" required:"false" doc:"Override LP owner"`
		LPProject    string   `json:"lp_project,omitempty" required:"false" doc:"Override LP project"`
		ArtifactsDir string   `json:"artifacts_dir,omitempty" required:"false" doc:"Output directory (default from config)"`
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
		DryRun  bool   `json:"dry_run,omitempty" required:"false" doc:"Show what would be deleted"`
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
	operationFacade := frontend.NewFacade(application)

	huma.Register(api, huma.Operation{
		OperationID: "trigger-builds",
		Method:      http.MethodPost,
		Path:        "/api/v1/builds/trigger",
		Summary:     "Trigger builds for a project",
		Description: "Trigger builds for a project's artifacts, optionally from a local git repo.",
		Tags:        []string{"builds"},
	}, func(ctx context.Context, input *BuildsTriggerInput) (*BuildsTriggerOutput, error) {
		svc, err := application.BuildService()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to build build service: %v", err))
		}

		triggerOpts, err := buildTriggerOptionsFromInput(input)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid timeout duration", err)
		}

		result, err := svc.Trigger(ctx, input.Body.Project, input.Body.Artifacts, triggerOpts)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("trigger failed: %v", err))
		}

		out := &BuildsTriggerOutput{}
		out.Body = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "trigger-builds-async",
		Method:      http.MethodPost,
		Path:        "/api/v1/builds/trigger/async",
		Summary:     "Trigger builds asynchronously",
		Description: "Queue build triggering as a long-running operation job.",
		Tags:        []string{"builds", "operations"},
	}, func(ctx context.Context, input *BuildsTriggerInput) (*OperationOutput, error) {
		triggerOpts, err := buildTriggerOptionsFromInput(input)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid timeout duration", err)
		}

		job, err := operationFacade.StartBuildTrigger(ctx, frontend.BuildTriggerOptions{
			Project:   input.Body.Project,
			Artifacts: input.Body.Artifacts,
			Trigger:   triggerOpts,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("async trigger failed: %v", err))
		}

		out := &OperationOutput{}
		out.Body = *job
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
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to build build service: %v", err))
		}

		builds, _, err := svc.List(ctx, build.ListOpts{
			Projects:     input.Projects,
			All:          input.All,
			State:        input.State,
			Owner:        input.Owner,
			LPProject:    input.LPProject,
			RecipeNames:  input.RecipeNames,
			RecipePrefix: input.RecipePrefix,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to list builds: %v", err))
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
		Description: "Download build artifacts for succeeded builds of the given artifacts.",
		Tags:        []string{"builds"},
	}, func(ctx context.Context, input *BuildsDownloadInput) (*BuildsDownloadOutput, error) {
		svc, err := application.BuildService()
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to build build service: %v", err))
		}

		artifactsDir := input.Body.ArtifactsDir
		if artifactsDir == "" {
			artifactsDir = application.Config.Build.ArtifactsDir
		}

		if err := svc.Download(ctx, build.DownloadOpts{
			Projects:      []string{input.Body.Project},
			ArtifactNames: input.Body.Artifacts,
			RecipePrefix:  input.Body.RecipePrefix,
			Owner:         input.Body.Owner,
			LPProject:     input.Body.LPProject,
			OutputDir:     artifactsDir,
		}); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("download failed: %v", err))
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
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to build build service: %v", err))
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
			return nil, huma.Error500InternalServerError(fmt.Sprintf("cleanup failed: %v", err))
		}

		out := &BuildsCleanupOutput{}
		out.Body.Deleted = deleted
		return out, nil
	})
}

func buildTriggerOptionsFromInput(input *BuildsTriggerInput) (build.TriggerOpts, error) {
	var timeout time.Duration
	var err error
	if input.Body.Timeout != "" {
		timeout, err = time.ParseDuration(input.Body.Timeout)
		if err != nil {
			return build.TriggerOpts{}, err
		}
	}

	return build.TriggerOpts{
		Wait:         input.Body.Wait,
		Timeout:      timeout,
		Owner:        input.Body.Owner,
		Prefix:       input.Body.Prefix,
		RepoSelfLink: input.Body.RepoSelfLink,
		GitRefLinks:  input.Body.GitRefLinks,
		BuildPaths:   input.Body.BuildPaths,
		LPProject:    input.Body.LPProject,
	}, nil
}
