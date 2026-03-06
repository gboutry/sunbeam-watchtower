// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package appclient

import (
	"context"
	"net/url"

	dto "github.com/gboutry/sunbeam-watchtower/internal/dto/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// BuildsTriggerOptions holds the request body for triggering builds.
type BuildsTriggerOptions struct {
	Project   string   `json:"project"`
	Recipes   []string `json:"recipes,omitempty"`
	Source    string   `json:"source,omitempty"`
	Wait      bool     `json:"wait,omitempty"`
	Timeout   string   `json:"timeout,omitempty"`
	Owner     string   `json:"owner,omitempty"`
	Prefix    string   `json:"prefix,omitempty"`
	LocalPath string   `json:"local_path,omitempty"`
}

// BuildsTrigger triggers builds for a project.
func (c *Client) BuildsTrigger(ctx context.Context, opts BuildsTriggerOptions) (*dto.BuildTriggerResult, error) {
	var result dto.BuildTriggerResult
	err := c.post(ctx, "/api/v1/builds/trigger", opts, &result)
	return &result, err
}

// BuildsListOptions holds query parameters for listing builds.
type BuildsListOptions struct {
	Projects []string
	All      bool
	State    string
}

// BuildsListResult is the response returned by BuildsList.
type BuildsListResult struct {
	Builds []port.Build `json:"builds"`
}

// BuildsList lists builds across projects.
func (c *Client) BuildsList(ctx context.Context, opts BuildsListOptions) ([]port.Build, error) {
	q := url.Values{}
	for _, v := range opts.Projects {
		q.Add("project", v)
	}
	if opts.All {
		q.Set("all", "true")
	}
	if opts.State != "" {
		q.Set("state", opts.State)
	}

	var result BuildsListResult
	err := c.get(ctx, "/api/v1/builds", q, &result)
	return result.Builds, err
}

// BuildsDownloadOptions holds the request body for downloading build artifacts.
type BuildsDownloadOptions struct {
	Project      string   `json:"project"`
	Recipes      []string `json:"recipes,omitempty"`
	ArtifactsDir string   `json:"artifacts_dir,omitempty"`
}

// BuildsDownload downloads build artifacts.
func (c *Client) BuildsDownload(ctx context.Context, opts BuildsDownloadOptions) error {
	return c.post(ctx, "/api/v1/builds/download", opts, nil)
}

// BuildsCleanupOptions holds the request body for cleaning up temporary recipes.
type BuildsCleanupOptions struct {
	Project string `json:"project,omitempty"`
	Owner   string `json:"owner,omitempty"`
	Prefix  string `json:"prefix,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

// BuildsCleanupResult is the response returned by BuildsCleanup.
type BuildsCleanupResult struct {
	Deleted []string `json:"deleted"`
}

// BuildsCleanup deletes temporary build recipes.
func (c *Client) BuildsCleanup(ctx context.Context, opts BuildsCleanupOptions) ([]string, error) {
	var result BuildsCleanupResult
	err := c.post(ctx, "/api/v1/builds/cleanup", opts, &result)
	return result.Deleted, err
}
