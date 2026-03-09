// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/bug"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/bugsync"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// BugServerWorkflow exposes reusable server-side bug workflows for the HTTP API.
type BugServerWorkflow struct {
	application *app.App
}

// NewBugServerWorkflow creates a server-side bug workflow.
func NewBugServerWorkflow(application *app.App) *BugServerWorkflow {
	return &BugServerWorkflow{application: application}
}

// Show returns one bug with its associated tasks.
func (w *BugServerWorkflow) Show(ctx context.Context, id string) (*forge.Bug, error) {
	trackers, projectMap, err := w.application.BuildBugTrackers()
	if err != nil {
		return nil, err
	}
	return bug.NewService(trackers, projectMap, w.application.Logger).Get(ctx, id)
}

// List returns bug tasks matching the requested filters.
func (w *BugServerWorkflow) List(ctx context.Context, req BugListRequest) (*BugListResponse, error) {
	trackers, projectMap, err := w.application.BuildBugTrackers()
	if err != nil {
		return nil, err
	}

	tasks, results, err := bug.NewService(trackers, projectMap, w.application.Logger).List(ctx, bug.ListOptions{
		Projects:   req.Projects,
		Status:     req.Status,
		Importance: req.Importance,
		Assignee:   req.Assignee,
		Tags:       req.Tags,
		Since:      req.Since,
		Merge:      req.Merge,
	})
	if err != nil {
		return nil, err
	}

	response := &BugListResponse{Tasks: tasks}
	for _, result := range results {
		if result.Err != nil {
			response.Warnings = append(response.Warnings, result.Err.Error())
		}
	}
	return response, nil
}

// Sync triggers one bug correlation/sync run.
func (w *BugServerWorkflow) Sync(ctx context.Context, req BugSyncRequest) (*BugSyncResponse, error) {
	sources, err := w.application.BuildCommitSources()
	if err != nil {
		return nil, err
	}

	trackers, _, err := w.application.BuildBugTrackers()
	if err != nil {
		return nil, err
	}

	var tracker port.BugTracker
	var lpProjects []string
	for _, pt := range trackers {
		if tracker == nil {
			tracker = pt.Tracker
		}
		lpProjects = append(lpProjects, pt.ProjectID)
	}
	if tracker == nil {
		return nil, ErrNoBugTrackerConfigured
	}

	lpProjectMap := make(map[string][]string)
	for _, proj := range w.application.Config.Projects {
		for _, bugConfig := range proj.Bugs {
			if bugConfig.Forge == "launchpad" {
				lpProjectMap[proj.Name] = append(lpProjectMap[proj.Name], bugConfig.Project)
			}
		}
	}

	opts := bugsync.SyncOptions{
		Projects: req.Projects,
		DryRun:   req.DryRun,
	}
	if req.Since != "" {
		since, err := time.Parse(time.RFC3339, req.Since)
		if err != nil {
			return nil, ErrInvalidBugSyncSince
		}
		opts.Since = &since
	}

	result, err := bugsync.NewService(sources, tracker, lpProjects, lpProjectMap, w.application.Logger).Sync(ctx, opts)
	if err != nil {
		return nil, err
	}

	response := &BugSyncResponse{
		Result: result,
	}
	for _, syncErr := range result.Errors {
		response.Warnings = append(response.Warnings, syncErr.Error())
	}
	return response, nil
}
