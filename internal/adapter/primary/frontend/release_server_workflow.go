// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	releasesvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/release"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ReleaseServerWorkflow exposes reusable server-side release workflows.
type ReleaseServerWorkflow struct {
	application *app.App
}

// NewReleaseServerWorkflow creates a server-side release workflow.
func NewReleaseServerWorkflow(application *app.App) *ReleaseServerWorkflow {
	return &ReleaseServerWorkflow{application: application}
}

// List lists cached published artifact release rows.
func (w *ReleaseServerWorkflow) List(ctx context.Context, req dto.ReleaseListQuery) ([]dto.ReleaseListEntry, error) {
	service, err := w.service()
	if err != nil {
		return nil, err
	}
	return service.List(ctx, req)
}

// Show returns the cached full matrix for one artifact.
func (w *ReleaseServerWorkflow) Show(ctx context.Context, name string, artifactType *dto.ArtifactType, track string) (*dto.ReleaseShowResult, error) {
	service, err := w.service()
	if err != nil {
		return nil, err
	}
	return service.Show(ctx, name, artifactType, track)
}

// SyncCache refreshes cached publication snapshots.
func (w *ReleaseServerWorkflow) SyncCache(ctx context.Context) error {
	service, err := w.service()
	if err != nil {
		return err
	}
	publications, err := w.application.TrackedPublications()
	if err != nil {
		return err
	}
	return service.SyncCache(ctx, publications)
}

// CacheStatus returns cached publication metadata.
func (w *ReleaseServerWorkflow) CacheStatus(ctx context.Context) ([]dto.ReleaseCacheStatus, error) {
	service, err := w.service()
	if err != nil {
		return nil, err
	}
	return service.CacheStatus(ctx)
}

func (w *ReleaseServerWorkflow) service() (*releasesvc.Service, error) {
	cache, err := w.application.ReleaseCache()
	if err != nil {
		return nil, fmt.Errorf("failed to open release cache: %w", err)
	}
	return releasesvc.NewService(cache, w.application.BuildReleaseSources(), w.application.Logger), nil
}
