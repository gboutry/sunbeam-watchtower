// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
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
