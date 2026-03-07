// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// BuildWorkflow coordinates frontend-side build preparation and API calls.
type BuildWorkflow struct {
	client   *client.Client
	preparer *LocalBuildPreparer
}

// BuildTriggerRequest describes a frontend build trigger workflow.
type BuildTriggerRequest struct {
	Source       string
	LocalPath    string
	Async        bool
	Download     bool
	ArtifactsDir string
	Trigger      client.BuildsTriggerOptions
}

// BuildTriggerResponse contains the remote result plus CLI/TUI-friendly slices.
type BuildTriggerResponse struct {
	Result   *dto.BuildTriggerResult
	Job      *dto.OperationJob
	Requests []dto.BuildRequest
	Builds   []dto.Build
	Errors   []error
}

// BuildListRequest describes a frontend list-builds workflow.
type BuildListRequest struct {
	Source     string
	SHA        string
	Prefix     string
	DefaultAll bool
	List       client.BuildsListOptions
}

// BuildDownloadRequest describes a frontend build-download workflow.
type BuildDownloadRequest struct {
	Source   string
	SHA      string
	Prefix   string
	Download client.BuildsDownloadOptions
}

// NewBuildWorkflow creates a reusable frontend build workflow.
func NewBuildWorkflow(apiClient *client.Client, preparer *LocalBuildPreparer) *BuildWorkflow {
	return &BuildWorkflow{
		client:   apiClient,
		preparer: preparer,
	}
}

// NewBuildWorkflowFromApp creates a frontend build workflow wired from the application.
func NewBuildWorkflowFromApp(apiClient *client.Client, application *app.App) (*BuildWorkflow, error) {
	preparer, err := NewLocalBuildPreparerFromApp(application)
	if err != nil {
		return nil, err
	}
	return NewBuildWorkflow(apiClient, preparer), nil
}

// Trigger runs a build trigger workflow and optionally follows up with downloads.
func (w *BuildWorkflow) Trigger(ctx context.Context, req BuildTriggerRequest) (*BuildTriggerResponse, error) {
	if w.client == nil {
		return nil, errors.New("build workflow requires an API client")
	}

	triggerOpts := req.Trigger
	requestedArtifacts := append([]string(nil), triggerOpts.Artifacts...)

	if req.Source == "local" {
		if w.preparer == nil {
			return nil, errors.New("local build preparation is not configured")
		}
		var err error
		triggerOpts, err = w.preparer.PrepareTrigger(ctx, triggerOpts, req.LocalPath)
		if err != nil {
			return nil, err
		}
	}

	response := &BuildTriggerResponse{}
	if req.Async {
		job, err := w.client.BuildsTriggerAsync(ctx, triggerOpts)
		if err != nil {
			return nil, err
		}
		response.Job = job
		return response, nil
	}

	result, err := w.client.BuildsTrigger(ctx, triggerOpts)
	if err != nil {
		return nil, err
	}
	response.Result = result

	for _, recipe := range result.RecipeResults {
		response.Builds = append(response.Builds, recipe.Builds...)
		if recipe.BuildRequest != nil {
			response.Requests = append(response.Requests, *recipe.BuildRequest)
		}
		if recipe.ErrorMessage != "" {
			response.Errors = append(response.Errors, fmt.Errorf("recipe %s: %s", recipe.Name, recipe.ErrorMessage))
		}
	}

	if req.Download && len(response.Builds) > 0 {
		downloadArtifacts := triggerOpts.Artifacts
		if len(downloadArtifacts) == 0 {
			downloadArtifacts = requestedArtifacts
		}
		if err := w.client.BuildsDownload(ctx, client.BuildsDownloadOptions{
			Project:      req.Trigger.Project,
			Artifacts:    downloadArtifacts,
			ArtifactsDir: req.ArtifactsDir,
		}); err != nil {
			response.Errors = append(response.Errors, fmt.Errorf("download: %w", err))
		}
	}

	return response, nil
}

// List resolves any local-build prefix state and lists builds.
func (w *BuildWorkflow) List(ctx context.Context, req BuildListRequest) ([]dto.Build, error) {
	if w.client == nil {
		return nil, errors.New("build workflow requires an API client")
	}

	listOpts := req.List
	if req.Source == "local" {
		if w.preparer == nil {
			return nil, errors.New("local build preparation is not configured")
		}
		if req.DefaultAll && !listOpts.All {
			listOpts.All = true
		}
		listPrefix := req.Prefix
		if req.SHA != "" {
			listPrefix += req.SHA + "-"
		}
		var err error
		listOpts, err = w.preparer.PrepareListByPrefix(ctx, listOpts, listPrefix)
		if err != nil {
			return nil, err
		}
	}

	return w.client.BuildsList(ctx, listOpts)
}

// Download resolves any local-build prefix state and downloads artifacts.
func (w *BuildWorkflow) Download(ctx context.Context, req BuildDownloadRequest) error {
	if w.client == nil {
		return errors.New("build workflow requires an API client")
	}

	downloadOpts := req.Download
	if req.Source == "local" {
		if w.preparer == nil {
			return errors.New("local build preparation is not configured")
		}
		listPrefix := req.Prefix
		if req.SHA != "" {
			listPrefix += req.SHA + "-"
		}
		var err error
		downloadOpts, err = w.preparer.PrepareDownloadByPrefix(ctx, downloadOpts, listPrefix)
		if err != nil {
			return err
		}
	}

	return w.client.BuildsDownload(ctx, downloadOpts)
}
