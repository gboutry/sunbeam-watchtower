// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// BuildWorkflow coordinates frontend-side build preparation and API calls.
type BuildWorkflow struct {
	client   *ClientTransport
	preparer *LocalBuildPreparer
}

// BuildTriggerRequest describes a frontend build trigger workflow.
type BuildTriggerRequest struct {
	Source       string
	LocalPath    string
	Async        bool
	Download     bool
	ArtifactsDir string
	Project      string
	Artifacts    []string
	Wait         bool
	Timeout      time.Duration
	Owner        string
	Prefix       string
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
	Projects   []string
	All        bool
	State      string
	Owner      string
}

// BuildDownloadRequest describes a frontend build-download workflow.
type BuildDownloadRequest struct {
	Source       string
	SHA          string
	Prefix       string
	Project      string
	Artifacts    []string
	ArtifactsDir string
	Owner        string
}

// BuildCleanupRequest describes a frontend build-cleanup workflow.
type BuildCleanupRequest struct {
	Project string
	Owner   string
	Prefix  string
	DryRun  bool
}

// NewBuildWorkflow creates a reusable frontend build workflow.
func NewBuildWorkflow(apiClient *ClientTransport, preparer *LocalBuildPreparer) *BuildWorkflow {
	return &BuildWorkflow{
		client:   apiClient,
		preparer: preparer,
	}
}

// NewBuildWorkflowFromApp creates a frontend build workflow wired from the application.
func NewBuildWorkflowFromApp(apiClient *ClientTransport, application *app.App) (*BuildWorkflow, error) {
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

	preparedTrigger := PreparedBuildTriggerRequest{
		Project:   req.Project,
		Artifacts: append([]string(nil), req.Artifacts...),
		Wait:      req.Wait,
		Owner:     req.Owner,
		Prefix:    req.Prefix,
	}
	if req.Timeout > 0 {
		preparedTrigger.Timeout = req.Timeout
	}
	requestedArtifacts := append([]string(nil), preparedTrigger.Artifacts...)

	if req.Source == "local" {
		if w.preparer == nil {
			return nil, errors.New("local build preparation is not configured")
		}
		var err error
		preparedTrigger, err = w.preparer.PrepareTrigger(ctx, preparedTrigger, req.LocalPath)
		if err != nil {
			return nil, err
		}
	}
	triggerOpts := client.BuildsTriggerOptions{
		Project:   preparedTrigger.Project,
		Artifacts: preparedTrigger.Artifacts,
		Wait:      preparedTrigger.Wait,
		Owner:     preparedTrigger.Owner,
		Prefix:    preparedTrigger.Prefix,
		Prepared:  preparedTrigger.Prepared,
	}
	if preparedTrigger.Timeout > 0 {
		triggerOpts.Timeout = preparedTrigger.Timeout.String()
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
			Project:      req.Project,
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

	preparedList := PreparedBuildListRequest{
		Projects: append([]string(nil), req.Projects...),
		All:      req.All,
		State:    req.State,
		Owner:    req.Owner,
	}
	if req.Source == "local" {
		if w.preparer == nil {
			return nil, errors.New("local build preparation is not configured")
		}
		if req.DefaultAll && !preparedList.All {
			preparedList.All = true
		}
		listPrefix := req.Prefix
		if req.SHA != "" {
			listPrefix += req.SHA + "-"
		}
		var err error
		preparedList, err = w.preparer.PrepareListByPrefix(ctx, preparedList, listPrefix)
		if err != nil {
			return nil, err
		}
	}
	listOpts := client.BuildsListOptions{
		Projects:     preparedList.Projects,
		All:          preparedList.All,
		State:        preparedList.State,
		Owner:        preparedList.Owner,
		LPProject:    preparedList.LPProject,
		RecipePrefix: preparedList.RecipePrefix,
	}

	return w.client.BuildsList(ctx, listOpts)
}

// Download resolves any local-build prefix state and downloads artifacts.
func (w *BuildWorkflow) Download(ctx context.Context, req BuildDownloadRequest) error {
	if w.client == nil {
		return errors.New("build workflow requires an API client")
	}

	preparedDownload := PreparedBuildDownloadRequest{
		Project:      req.Project,
		Artifacts:    append([]string(nil), req.Artifacts...),
		ArtifactsDir: req.ArtifactsDir,
		Owner:        req.Owner,
	}
	if req.Source == "local" {
		if w.preparer == nil {
			return errors.New("local build preparation is not configured")
		}
		listPrefix := req.Prefix
		if req.SHA != "" {
			listPrefix += req.SHA + "-"
		}
		var err error
		preparedDownload, err = w.preparer.PrepareDownloadByPrefix(ctx, preparedDownload, listPrefix)
		if err != nil {
			return err
		}
	}
	downloadOpts := client.BuildsDownloadOptions{
		Project:      preparedDownload.Project,
		Artifacts:    preparedDownload.Artifacts,
		RecipePrefix: preparedDownload.RecipePrefix,
		Owner:        preparedDownload.Owner,
		LPProject:    preparedDownload.LPProject,
		ArtifactsDir: preparedDownload.ArtifactsDir,
	}

	return w.client.BuildsDownload(ctx, downloadOpts)
}

// Cleanup deletes temporary build recipes matching the requested filters.
func (w *BuildWorkflow) Cleanup(ctx context.Context, req BuildCleanupRequest) ([]string, error) {
	if w.client == nil {
		return nil, errors.New("build workflow requires an API client")
	}
	return w.client.BuildsCleanup(ctx, client.BuildsCleanupOptions{
		Project: req.Project,
		Owner:   req.Owner,
		Prefix:  req.Prefix,
		DryRun:  req.DryRun,
	})
}
