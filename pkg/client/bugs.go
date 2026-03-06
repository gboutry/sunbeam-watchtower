// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/url"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// BugsListOptions holds query parameters for listing bug tasks.
type BugsListOptions struct {
	Projects   []string
	Status     []string
	Importance []string
	Assignee   string
	Tags       []string
	Since      string // ISO 8601 date — return bugs created/modified since this date
}

// BugsListResult is the response returned by BugsList.
type BugsListResult struct {
	Tasks    []forge.BugTask `json:"tasks"`
	Warnings []string        `json:"warnings,omitempty"`
}

// BugsList lists bug tasks across all configured bug trackers.
func (c *Client) BugsList(ctx context.Context, opts BugsListOptions) (*BugsListResult, error) {
	q := url.Values{}
	for _, v := range opts.Projects {
		q.Add("project", v)
	}
	for _, v := range opts.Status {
		q.Add("status", v)
	}
	for _, v := range opts.Importance {
		q.Add("importance", v)
	}
	if opts.Assignee != "" {
		q.Set("assignee", opts.Assignee)
	}
	for _, v := range opts.Tags {
		q.Add("tag", v)
	}
	if opts.Since != "" {
		q.Set("since", opts.Since)
	}

	var result BugsListResult
	err := c.get(ctx, "/api/v1/bugs", q, &result)
	return &result, err
}

// BugsGet retrieves a single bug by ID.
func (c *Client) BugsGet(ctx context.Context, id string) (*forge.Bug, error) {
	var result forge.Bug
	err := c.get(ctx, "/api/v1/bugs/"+url.PathEscape(id), nil, &result)
	return &result, err
}

// BugsSyncOptions holds the request body for the bug sync endpoint.
type BugsSyncOptions struct {
	Projects []string `json:"projects,omitempty"`
	DryRun   bool     `json:"dry_run"`
	Since    string   `json:"since,omitempty"` // RFC 3339 timestamp
}

// BugsSyncResult is the response returned by BugsSync.
type BugsSyncResult struct {
	Actions []dto.BugSyncAction `json:"actions"`
	Skipped int                 `json:"skipped"`
	Errors  []string            `json:"errors,omitempty"`
}

// BugsSync triggers a bug status sync from commits.
func (c *Client) BugsSync(ctx context.Context, opts BugsSyncOptions) (*BugsSyncResult, error) {
	var result BugsSyncResult
	err := c.post(ctx, "/api/v1/bugs/sync", opts, &result)
	return &result, err
}
