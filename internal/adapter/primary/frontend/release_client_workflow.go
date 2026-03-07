// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ReleasesListRequest describes one release-list workflow.
type ReleasesListRequest struct {
	Names        []string
	Projects     []string
	ArtifactType string
	Tracks       []string
	Risks        []string
}

// ReleasesShowRequest describes one release-show workflow.
type ReleasesShowRequest struct {
	Name         string
	ArtifactType string
	Track        string
}

// ReleaseClientWorkflow exposes reusable client-side release workflows.
type ReleaseClientWorkflow struct {
	client *ClientTransport
}

// NewReleaseClientWorkflow creates a client-side release workflow.
func NewReleaseClientWorkflow(apiClient *ClientTransport) *ReleaseClientWorkflow {
	return &ReleaseClientWorkflow{client: apiClient}
}

// List lists cached published artifact release rows.
func (w *ReleaseClientWorkflow) List(ctx context.Context, req ReleasesListRequest) ([]dto.ReleaseListEntry, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.ReleasesList(ctx, client.ReleasesListOptions{
		Names:        req.Names,
		Projects:     req.Projects,
		ArtifactType: req.ArtifactType,
		Tracks:       req.Tracks,
		Risks:        req.Risks,
	})
}

// Show returns the cached full matrix for one published artifact.
func (w *ReleaseClientWorkflow) Show(ctx context.Context, req ReleasesShowRequest) (*dto.ReleaseShowResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.ReleasesShow(ctx, req.Name, client.ReleasesShowOptions{
		ArtifactType: req.ArtifactType,
		Track:        req.Track,
	})
}

func (w *ReleaseClientWorkflow) resolveClient() (*ClientTransport, error) {
	if w.client == nil {
		return nil, errors.New("release client workflow requires an API client")
	}
	return w.client, nil
}
