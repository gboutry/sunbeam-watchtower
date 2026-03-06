package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	lpadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/launchpad"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	projectsvc "github.com/gboutry/sunbeam-watchtower/internal/service/project"
)

// --- Sync projects ---

// ProjectsSyncInput holds the request body for syncing LP projects.
type ProjectsSyncInput struct {
	Body struct {
		Projects []string `json:"projects,omitempty" doc:"Filter to these LP project names (empty = all)"`
		DryRun   bool     `json:"dry_run,omitempty" doc:"Show what would change without updating"`
	}
}

// ProjectsSyncOutput is the response for a project sync operation.
type ProjectsSyncOutput struct {
	Body struct {
		Actions []projectsvc.SyncAction `json:"actions" doc:"Actions taken or planned"`
		Errors  []string                `json:"errors,omitempty" doc:"Non-fatal error messages"`
	}
}

// RegisterProjectsAPI registers the /api/v1/projects endpoints on the given huma API.
func RegisterProjectsAPI(api huma.API, application *app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "sync-projects",
		Method:      http.MethodPost,
		Path:        "/api/v1/projects/sync",
		Summary:     "Sync LP project series and development focus",
		Description: "Ensure LP projects have declared series and set the development focus.",
		Tags:        []string{"projects"},
	}, func(ctx context.Context, input *ProjectsSyncInput) (*ProjectsSyncOutput, error) {
		cfg := application.Config
		if cfg == nil {
			return nil, huma.Error500InternalServerError("no configuration loaded")
		}

		// Collect unique LP project names from bug tracker entries,
		// resolving per-project series/dev-focus overrides.
		projectConfigs := make(map[string]projectsvc.ProjectSyncConfig)
		for _, proj := range cfg.Projects {
			for _, b := range proj.Bugs {
				if b.Forge != "launchpad" {
					continue
				}
				if _, ok := projectConfigs[b.Project]; ok {
					continue
				}
				psc := projectsvc.ProjectSyncConfig{
					Series:           cfg.Launchpad.Series,
					DevelopmentFocus: cfg.Launchpad.DevelopmentFocus,
				}
				if len(proj.Series) > 0 {
					psc.Series = proj.Series
				}
				if proj.DevelopmentFocus != "" {
					psc.DevelopmentFocus = proj.DevelopmentFocus
				}
				projectConfigs[b.Project] = psc
			}
		}

		if len(projectConfigs) == 0 {
			out := &ProjectsSyncOutput{}
			return out, nil
		}

		lpClient := app.NewLaunchpadClient(cfg.Launchpad, application.Logger)
		if lpClient == nil {
			return nil, huma.Error422UnprocessableEntity("Launchpad authentication required")
		}

		manager := lpadapter.NewProjectManager(lpClient)
		svc := projectsvc.NewService(manager, projectConfigs, application.Logger)

		result, err := svc.Sync(ctx, projectsvc.SyncOptions{
			Projects: input.Body.Projects,
			DryRun:   input.Body.DryRun,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("sync failed", err)
		}

		out := &ProjectsSyncOutput{}
		out.Body.Actions = result.Actions
		for _, e := range result.Errors {
			out.Body.Errors = append(out.Body.Errors, e.Error())
		}
		return out, nil
	})
}
