// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// TeamServerWorkflow exposes reusable server-side team workflows for the HTTP API.
type TeamServerWorkflow struct {
	application *app.App
	async       *Facade
}

// NewTeamServerWorkflow creates a server-side team workflow.
func NewTeamServerWorkflow(application *app.App, async *Facade) *TeamServerWorkflow {
	return &TeamServerWorkflow{
		application: application,
		async:       async,
	}
}

// Sync performs one synchronous team collaborator sync.
func (w *TeamServerWorkflow) Sync(_ context.Context, _ dto.TeamSyncRequest) (*dto.TeamSyncResult, error) {
	return nil, errors.New("team sync service not yet wired")
}

// StartSync queues one asynchronous team collaborator sync.
func (w *TeamServerWorkflow) StartSync(ctx context.Context, req dto.TeamSyncRequest) (*dto.OperationJob, error) {
	return w.async.StartTeamSync(ctx, TeamSyncOptions{
		Projects: req.Projects,
		DryRun:   req.DryRun,
	})
}
