package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"github.com/gboutry/sunbeam-watchtower/internal/service/bug"
	"github.com/gboutry/sunbeam-watchtower/internal/service/bugsync"
)

// --- List bugs ---

// BugsListInput holds query parameters for listing bug tasks.
type BugsListInput struct {
	Projects   []string `query:"project" doc:"Filter by project name"`
	Status     []string `query:"status" doc:"Filter by status"`
	Importance []string `query:"importance" doc:"Filter by importance"`
	Assignee   string   `query:"assignee" doc:"Filter by assignee"`
	Tags       []string `query:"tag" doc:"Filter by tag"`
}

// BugsListOutput is the response for listing bug tasks.
type BugsListOutput struct {
	Body struct {
		Tasks    []forge.BugTask `json:"tasks" doc:"Bug tasks matching filters"`
		Warnings []string        `json:"warnings,omitempty" doc:"Non-fatal warnings"`
	}
}

// --- Get bug ---

// BugGetInput holds the path parameter for getting a single bug.
type BugGetInput struct {
	ID string `path:"id" doc:"Bug ID (e.g. Launchpad bug number)"`
}

// BugGetOutput is the response for getting a single bug.
type BugGetOutput struct {
	Body *forge.Bug
}

// --- Sync bugs ---

// BugSyncInput holds the request body for triggering a bug sync.
type BugSyncInput struct {
	Body struct {
		Projects []string `json:"projects,omitempty" doc:"Filter to these project names (empty = all)"`
		DryRun   bool     `json:"dry_run" doc:"If true, show what would change without updating"`
		Days     int      `json:"days,omitempty" doc:"Only consider bugs created in the last N days"`
	}
}

// BugSyncOutput is the response for a bug sync operation.
type BugSyncOutput struct {
	Body struct {
		Actions []bugsync.SyncAction `json:"actions" doc:"Actions taken or planned"`
		Skipped int                  `json:"skipped" doc:"Number of tasks skipped"`
		Errors  []string             `json:"errors,omitempty" doc:"Non-fatal error messages"`
	}
}

// RegisterBugsAPI registers the /api/v1/bugs endpoints on the given huma API.
func RegisterBugsAPI(api huma.API, application *app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "list-bugs",
		Method:      http.MethodGet,
		Path:        "/api/v1/bugs",
		Summary:     "List bug tasks",
		Description: "List bug tasks across all configured bug trackers, with optional filters.",
	}, func(ctx context.Context, input *BugsListInput) (*BugsListOutput, error) {
		trackers, projectMap, err := application.BuildBugTrackers()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build bug trackers", err)
		}

		svc := bug.NewService(trackers, projectMap, application.Logger)

		tasks, results, err := svc.List(ctx, bug.ListOptions{
			Projects:   input.Projects,
			Status:     input.Status,
			Importance: input.Importance,
			Assignee:   input.Assignee,
			Tags:       input.Tags,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list bugs", err)
		}

		out := &BugsListOutput{}
		out.Body.Tasks = tasks
		for _, r := range results {
			if r.Err != nil {
				out.Body.Warnings = append(out.Body.Warnings, r.Err.Error())
			}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-bug",
		Method:      http.MethodGet,
		Path:        "/api/v1/bugs/{id}",
		Summary:     "Get a bug",
		Description: "Retrieve a single bug and its tasks by ID.",
	}, func(ctx context.Context, input *BugGetInput) (*BugGetOutput, error) {
		trackers, projectMap, err := application.BuildBugTrackers()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build bug trackers", err)
		}

		svc := bug.NewService(trackers, projectMap, application.Logger)

		b, err := svc.Get(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("bug %s not found", input.ID), err)
		}

		out := &BugGetOutput{}
		out.Body = b
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "sync-bugs",
		Method:      http.MethodPost,
		Path:        "/api/v1/bugs/sync",
		Summary:     "Sync bug statuses",
		Description: "Scan cached commits for bug references and update bug task statuses. Also assigns bugs to the appropriate series.",
	}, func(ctx context.Context, input *BugSyncInput) (*BugSyncOutput, error) {
		sources, err := application.BuildCommitSources()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build commit sources", err)
		}

		trackers, _, err := application.BuildBugTrackers()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build bug trackers", err)
		}

		// Use the first available bug tracker and collect LP project names.
		var tracker port.BugTracker
		var lpProjects []string
		for _, pt := range trackers {
			if tracker == nil {
				tracker = pt.Tracker
			}
			lpProjects = append(lpProjects, pt.ProjectID)
		}
		if tracker == nil {
			return nil, huma.Error422UnprocessableEntity("no bug tracker configured")
		}

		// Build watchtower project → LP bug project mapping.
		lpProjectMap := make(map[string][]string)
		for _, proj := range application.Config.Projects {
			for _, b := range proj.Bugs {
				if b.Forge == "launchpad" {
					lpProjectMap[proj.Name] = append(lpProjectMap[proj.Name], b.Project)
				}
			}
		}

		svc := bugsync.NewService(sources, tracker, lpProjects, lpProjectMap, application.Logger)
		syncOpts := bugsync.SyncOptions{
			Projects: input.Body.Projects,
			DryRun:   input.Body.DryRun,
		}
		if input.Body.Days > 0 {
			since := time.Now().AddDate(0, 0, -input.Body.Days)
			syncOpts.Since = &since
		}

		result, err := svc.Sync(ctx, syncOpts)
		if err != nil {
			return nil, huma.Error500InternalServerError("sync failed", err)
		}

		out := &BugSyncOutput{}
		out.Body.Actions = result.Actions
		out.Body.Skipped = result.Skipped
		for _, e := range result.Errors {
			out.Body.Errors = append(out.Body.Errors, e.Error())
		}
		return out, nil
	})
}
