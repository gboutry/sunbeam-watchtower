// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"slices"

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
func (w *TeamServerWorkflow) Sync(ctx context.Context, req dto.TeamSyncRequest) (*dto.TeamSyncResult, error) {
	service, err := w.application.TeamSyncService()
	if err != nil {
		return nil, err
	}

	cfg := w.application.Config
	if cfg == nil {
		return nil, errors.New("no configuration loaded")
	}
	if cfg.Collaborators == nil {
		return nil, errors.New("collaborators not configured")
	}

	var targets []dto.SyncTarget
	for _, proj := range cfg.Projects {
		if len(req.Projects) > 0 && !slices.Contains(req.Projects, proj.Name) {
			continue
		}
		at, err := dto.ParseArtifactType(proj.ArtifactType)
		if err != nil || (at != dto.ArtifactSnap && at != dto.ArtifactCharm) {
			continue
		}
		targets = append(targets, dto.SyncTarget{
			Project:      proj.Name,
			ArtifactType: at,
			StoreName:    proj.Name, // default: project name = store name
		})
	}

	return service.Sync(ctx, cfg.Collaborators.LaunchpadTeam, targets, req.DryRun)
}

// StartSync queues one asynchronous team collaborator sync.
func (w *TeamServerWorkflow) StartSync(ctx context.Context, req dto.TeamSyncRequest) (*dto.OperationJob, error) {
	return w.async.StartTeamSync(ctx, TeamSyncOptions{
		Projects: req.Projects,
		DryRun:   req.DryRun,
	})
}
