package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	projectsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/project"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// --- Sync projects ---

// ProjectsSyncInput holds the request body for syncing LP projects.
type ProjectsSyncInput struct {
	Body struct {
		Projects []string `json:"projects,omitempty" required:"false" doc:"Filter to these LP project names (empty = all)"`
		DryRun   bool     `json:"dry_run,omitempty" required:"false" doc:"Show what would change without updating"`
	}
}

// ProjectsSyncOutput is the response for a project sync operation.
type ProjectsSyncOutput struct {
	Body struct {
		Actions []dto.ProjectSyncAction `json:"actions" doc:"Actions taken or planned"`
		Errors  []string                `json:"errors,omitempty" doc:"Non-fatal error messages"`
	}
}

// RegisterProjectsAPI registers the /api/v1/projects endpoints on the given huma API.
func RegisterProjectsAPI(api huma.API, application *app.App) {
	operationFacade := frontend.NewFacade(application)

	huma.Register(api, huma.Operation{
		OperationID: "sync-projects",
		Method:      http.MethodPost,
		Path:        "/api/v1/projects/sync",
		Summary:     "Sync LP project series and development focus",
		Description: "Ensure LP projects have declared series and set the development focus.",
		Tags:        []string{"projects"},
	}, func(ctx context.Context, input *ProjectsSyncInput) (*ProjectsSyncOutput, error) {
		svc, err := application.ProjectService()
		if err != nil {
			if errors.Is(err, app.ErrLaunchpadAuthRequired) {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to build project service: %v", err))
		}

		result, err := svc.Sync(ctx, projectsvc.SyncOptions{
			Projects: input.Body.Projects,
			DryRun:   input.Body.DryRun,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("sync failed: %v", err))
		}

		out := &ProjectsSyncOutput{}
		out.Body.Actions = result.Actions
		for _, e := range result.Errors {
			out.Body.Errors = append(out.Body.Errors, e.Error())
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "sync-projects-async",
		Method:      http.MethodPost,
		Path:        "/api/v1/projects/sync/async",
		Summary:     "Sync LP projects asynchronously",
		Description: "Queue project metadata sync as a long-running operation job.",
		Tags:        []string{"projects", "operations"},
	}, func(ctx context.Context, input *ProjectsSyncInput) (*OperationOutput, error) {
		job, err := operationFacade.StartProjectSync(ctx, frontend.ProjectSyncOptions{
			Projects: input.Body.Projects,
			DryRun:   input.Body.DryRun,
		})
		if err != nil {
			if errors.Is(err, app.ErrLaunchpadAuthRequired) {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			return nil, huma.Error500InternalServerError(fmt.Sprintf("async sync failed: %v", err))
		}

		out := &OperationOutput{}
		out.Body = *job
		return out, nil
	})
}
