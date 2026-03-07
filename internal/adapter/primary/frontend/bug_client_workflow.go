// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// BugListRequest describes one bug-list workflow.
type BugListRequest struct {
	Projects   []string
	Status     []string
	Importance []string
	Assignee   string
	Tags       []string
	Since      string
}

// BugListResponse contains listed bug tasks plus non-fatal warnings.
type BugListResponse struct {
	Tasks    []forge.BugTask
	Warnings []string
}

// BugSyncRequest describes one bug-sync workflow.
type BugSyncRequest struct {
	Projects []string
	DryRun   bool
	Since    string
}

// BugSyncResponse contains normalized bug-sync results plus non-fatal warnings.
type BugSyncResponse struct {
	Result   *dto.BugSyncResult
	Warnings []string
}

// BugClientWorkflow exposes reusable client-side bug workflows for CLI/TUI/MCP frontends.
type BugClientWorkflow struct {
	client *client.Client
}

// NewBugClientWorkflow creates a client-side bug workflow.
func NewBugClientWorkflow(apiClient *client.Client) *BugClientWorkflow {
	return &BugClientWorkflow{client: apiClient}
}

// Show returns one bug with its associated tasks.
func (w *BugClientWorkflow) Show(ctx context.Context, id string) (*forge.Bug, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.BugsGet(ctx, id)
}

// List returns bug tasks matching the requested filters.
func (w *BugClientWorkflow) List(ctx context.Context, req BugListRequest) (*BugListResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}

	resolvedSince, err := dto.ResolveSince(req.Since)
	if err != nil {
		return nil, err
	}

	result, err := apiClient.BugsList(ctx, client.BugsListOptions{
		Projects:   req.Projects,
		Status:     req.Status,
		Importance: req.Importance,
		Assignee:   req.Assignee,
		Tags:       req.Tags,
		Since:      resolvedSince,
	})
	if err != nil {
		return nil, err
	}

	return &BugListResponse{
		Tasks:    result.Tasks,
		Warnings: result.Warnings,
	}, nil
}

// Sync triggers remote bug correlation/sync work.
func (w *BugClientWorkflow) Sync(ctx context.Context, req BugSyncRequest) (*BugSyncResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}

	resolvedSince, err := dto.ResolveSince(req.Since)
	if err != nil {
		return nil, err
	}

	result, err := apiClient.BugsSync(ctx, client.BugsSyncOptions{
		Projects: req.Projects,
		DryRun:   req.DryRun,
		Since:    resolvedSince,
	})
	if err != nil {
		return nil, err
	}

	return &BugSyncResponse{
		Result: &dto.BugSyncResult{
			Actions: result.Actions,
			Skipped: result.Skipped,
		},
		Warnings: result.Errors,
	}, nil
}

func (w *BugClientWorkflow) resolveClient() (*client.Client, error) {
	if w.client == nil {
		return nil, errors.New("bug client workflow requires an API client")
	}
	return w.client, nil
}
