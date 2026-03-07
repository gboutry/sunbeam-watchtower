// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"fmt"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	opsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/operation"
	projectsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/project"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// Facade exposes frontend-oriented wrappers on top of the generic operation service.
type Facade struct {
	application *app.App
}

// BuildTriggerOptions holds the inputs for starting an async build trigger operation.
type BuildTriggerOptions struct {
	Project   string
	Artifacts []string
	Trigger   build.TriggerOpts
}

// ProjectSyncOptions holds the inputs for starting an async project sync operation.
type ProjectSyncOptions struct {
	Projects []string
	DryRun   bool
}

// NewFacade creates a frontend-oriented facade on top of the application wiring.
func NewFacade(application *app.App) *Facade {
	return &Facade{application: application}
}

// StartBuildTrigger starts an async build-trigger workflow.
func (f *Facade) StartBuildTrigger(ctx context.Context, opts BuildTriggerOptions) (*dto.OperationJob, error) {
	buildService, err := f.application.BuildService()
	if err != nil {
		return nil, err
	}
	operationService, err := f.application.OperationService()
	if err != nil {
		return nil, err
	}

	attributes := map[string]string{
		"project": opts.Project,
	}
	if len(opts.Artifacts) > 0 {
		attributes["artifacts"] = fmt.Sprintf("%d", len(opts.Artifacts))
	}
	if opts.Trigger.Owner != "" {
		attributes["owner"] = opts.Trigger.Owner
	}
	if opts.Trigger.LPProject != "" {
		attributes["lp_project"] = opts.Trigger.LPProject
	}
	if opts.Trigger.Wait {
		attributes["wait"] = "true"
	}

	return operationService.Start(ctx, dto.OperationKindBuildTrigger, attributes, func(runCtx context.Context, reporter *opsvc.Reporter) (string, error) {
		reporter.Event(fmt.Sprintf("starting build trigger for project %q", opts.Project))
		reporter.Progress(dto.OperationProgress{
			Phase:         "triggering",
			Message:       "submitting build requests",
			Indeterminate: true,
		})

		result, err := buildService.Trigger(runCtx, opts.Project, opts.Artifacts, opts.Trigger)
		if err != nil {
			return "", err
		}

		for _, recipe := range result.RecipeResults {
			switch {
			case recipe.ErrorMessage != "":
				reporter.Event(fmt.Sprintf("recipe %q failed: %s", recipe.Name, recipe.ErrorMessage))
			case recipe.BuildRequest != nil:
				reporter.Event(fmt.Sprintf("recipe %q submitted build request %q", recipe.Name, recipe.BuildRequest.Status))
			case len(recipe.Builds) > 0:
				reporter.Event(fmt.Sprintf("recipe %q returned %d builds", recipe.Name, len(recipe.Builds)))
			default:
				reporter.Event(fmt.Sprintf("recipe %q processed with no build records", recipe.Name))
			}
		}

		summary := buildTriggerSummary(result)
		reporter.Progress(dto.OperationProgress{
			Phase:   "completed",
			Message: summary,
			Current: len(result.RecipeResults),
			Total:   len(result.RecipeResults),
		})
		return summary, nil
	})
}

// StartProjectSync starts an async project-sync workflow.
func (f *Facade) StartProjectSync(ctx context.Context, opts ProjectSyncOptions) (*dto.OperationJob, error) {
	projectService, err := f.application.ProjectService()
	if err != nil {
		return nil, err
	}
	operationService, err := f.application.OperationService()
	if err != nil {
		return nil, err
	}

	attributes := map[string]string{}
	if opts.DryRun {
		attributes["dry_run"] = "true"
	}
	if len(opts.Projects) > 0 {
		attributes["projects"] = fmt.Sprintf("%d", len(opts.Projects))
	}

	return operationService.Start(ctx, dto.OperationKindProjectSync, attributes, func(runCtx context.Context, reporter *opsvc.Reporter) (string, error) {
		reporter.Event("starting Launchpad project sync")
		reporter.Progress(dto.OperationProgress{
			Phase:         "syncing",
			Message:       "checking Launchpad project metadata",
			Indeterminate: true,
		})

		result, err := projectService.Sync(runCtx, projectsvc.SyncOptions{
			Projects: opts.Projects,
			DryRun:   opts.DryRun,
		})
		if err != nil {
			return "", err
		}

		for _, action := range result.Actions {
			reporter.Event(projectSyncActionMessage(action))
		}
		for _, syncErr := range result.Errors {
			reporter.Event(fmt.Sprintf("project sync error: %s", syncErr.Error()))
		}

		summary := projectSyncSummary(result, opts.DryRun)
		reporter.Progress(dto.OperationProgress{
			Phase:   "completed",
			Message: summary,
			Current: len(result.Actions),
			Total:   len(result.Actions) + len(result.Errors),
		})
		return summary, nil
	})
}

// ListOperations returns all known operations.
func (f *Facade) ListOperations(ctx context.Context) ([]dto.OperationJob, error) {
	operationService, err := f.application.OperationService()
	if err != nil {
		return nil, err
	}
	return operationService.List(ctx)
}

// GetOperation returns one operation snapshot.
func (f *Facade) GetOperation(ctx context.Context, id string) (*dto.OperationJob, error) {
	operationService, err := f.application.OperationService()
	if err != nil {
		return nil, err
	}
	return operationService.Get(ctx, id)
}

// OperationEvents returns the event history for one operation.
func (f *Facade) OperationEvents(ctx context.Context, id string) ([]dto.OperationEvent, error) {
	operationService, err := f.application.OperationService()
	if err != nil {
		return nil, err
	}
	return operationService.Events(ctx, id)
}

// CancelOperation requests cancellation for a running operation.
func (f *Facade) CancelOperation(ctx context.Context, id string) error {
	operationService, err := f.application.OperationService()
	if err != nil {
		return err
	}
	return operationService.Cancel(ctx, id)
}

func buildTriggerSummary(result *dto.BuildTriggerResult) string {
	requestCount := 0
	buildCount := 0
	errorCount := 0
	for _, recipe := range result.RecipeResults {
		if recipe.BuildRequest != nil {
			requestCount++
		}
		buildCount += len(recipe.Builds)
		if recipe.ErrorMessage != "" {
			errorCount++
		}
	}

	parts := []string{fmt.Sprintf("%d recipes", len(result.RecipeResults))}
	if requestCount > 0 {
		parts = append(parts, fmt.Sprintf("%d requests", requestCount))
	}
	if buildCount > 0 {
		parts = append(parts, fmt.Sprintf("%d builds", buildCount))
	}
	if errorCount > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", errorCount))
	}

	summary := fmt.Sprintf("processed %s", strings.Join(parts, ", "))
	if result.Project != "" {
		summary = fmt.Sprintf("%s for %s", summary, result.Project)
	}
	return summary
}

func projectSyncSummary(result *projectsvc.SyncResult, dryRun bool) string {
	mode := "applied"
	if dryRun {
		mode = "planned"
	}

	summary := fmt.Sprintf("%s %d project sync actions", mode, len(result.Actions))
	if len(result.Errors) > 0 {
		summary = fmt.Sprintf("%s with %d non-fatal errors", summary, len(result.Errors))
	}
	return summary
}

func projectSyncActionMessage(action dto.ProjectSyncAction) string {
	switch action.ActionType {
	case dto.ProjectSyncActionCreateSeries:
		return fmt.Sprintf("project %q: create series %q", action.Project, action.Series)
	case dto.ProjectSyncActionSetDevFocus:
		return fmt.Sprintf("project %q: set development focus to %q", action.Project, action.Series)
	case dto.ProjectSyncActionDevFocusUnchanged:
		return fmt.Sprintf("project %q: development focus already %q", action.Project, action.Series)
	default:
		return fmt.Sprintf("project %q: completed %q", action.Project, action.ActionType)
	}
}
