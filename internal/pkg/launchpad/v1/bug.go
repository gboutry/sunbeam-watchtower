// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// GetBug fetches a bug by ID.
func (c *Client) GetBug(ctx context.Context, id int) (Bug, error) {
	var b Bug
	path := fmt.Sprintf("/bugs/%d", id)
	if err := c.GetJSON(ctx, path, &b); err != nil {
		return Bug{}, fmt.Errorf("fetching bug %d: %w", id, err)
	}
	return b, nil
}

// GetBugTasks returns the bug tasks for a given bug.
func (c *Client) GetBugTasks(ctx context.Context, bugID int) ([]BugTask, error) {
	path := fmt.Sprintf("/bugs/%d/bug_tasks", bugID)
	return GetAllPages[BugTask](ctx, c, path)
}

// SearchGlobalBugTasks searches bug tasks across all projects.
func (c *Client) SearchGlobalBugTasks(ctx context.Context, opts BugTaskSearchOpts) ([]BugTask, error) {
	params := opts.values()
	u := wsOpURL(c.resolveURL("/bugs"), "searchTasks", params)
	return GetAllPages[BugTask](ctx, c, u)
}

// CreateBug creates a new bug on the given target.
// target is the API link to the project or package (e.g. "https://api.launchpad.net/devel/sunbeam").
func (c *Client) CreateBug(ctx context.Context, target, title, description string, tags []string) (Bug, error) {
	form := url.Values{
		"ws.op":       {"createBug"},
		"target":      {target},
		"title":       {title},
		"description": {description},
	}
	for _, t := range tags {
		form.Add("tags", t)
	}
	var b Bug
	if err := c.PostJSON(ctx, "/bugs", form, &b); err != nil {
		return Bug{}, fmt.Errorf("creating bug: %w", err)
	}
	return b, nil
}

// UpdateBugTaskStatus PATCHes a bug task's status via its self_link.
func (c *Client) UpdateBugTaskStatus(ctx context.Context, taskSelfLink, status string) error {
	body, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return fmt.Errorf("marshalling status update: %w", err)
	}
	_, err = c.Patch(ctx, taskSelfLink, body)
	if err != nil {
		return fmt.Errorf("updating bug task status: %w", err)
	}
	return nil
}

// NominateBug nominates a bug for a series target.
func (c *Client) NominateBug(ctx context.Context, bugID int, seriesSelfLink string) error {
	form := url.Values{
		"ws.op":  {"addNomination"},
		"target": {seriesSelfLink},
	}
	_, err := c.Post(ctx, fmt.Sprintf("/bugs/%d", bugID), form)
	if err != nil {
		return fmt.Errorf("nominating bug %d: %w", bugID, err)
	}
	return nil
}
