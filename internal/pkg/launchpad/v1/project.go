// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"fmt"
	"net/url"
)

// CreateProject registers a new project on Launchpad.
// LP returns 201 with an empty body, so we POST then GET the new resource.
func (c *Client) CreateProject(ctx context.Context, name, displayName, summary, description string) (Project, error) {
	form := url.Values{
		"ws.op":        {"new_project"},
		"name":         {name},
		"display_name": {displayName},
		"title":        {displayName},
		"summary":      {summary},
		"licenses":     {"Apache Licence"},
	}
	if description != "" {
		form.Set("description", description)
	}
	if _, err := c.Post(ctx, "/projects", form); err != nil {
		return Project{}, fmt.Errorf("creating project %q: %w", name, err)
	}
	return c.GetProject(ctx, name)
}

// GetProject fetches a project by name.
func (c *Client) GetProject(ctx context.Context, name string) (Project, error) {
	var p Project
	if err := c.GetJSON(ctx, "/"+name, &p); err != nil {
		return Project{}, fmt.Errorf("fetching project %q: %w", name, err)
	}
	return p, nil
}

// SearchProjects searches projects by text in title/summary/description.
func (c *Client) SearchProjects(ctx context.Context, text string) ([]Project, error) {
	u := wsOpURL(c.resolveURL("/projects"), "search", url.Values{
		"text": {text},
	})
	return GetAllPages[Project](ctx, c, u)
}

// GetProjectMergeProposals returns merge proposals for a project.
func (c *Client) GetProjectMergeProposals(ctx context.Context, projectName string, status ...string) ([]MergeProposal, error) {
	params := url.Values{}
	for _, s := range status {
		params.Add("status", s)
	}
	u := wsOpURL(c.resolveURL("/"+projectName), "getMergeProposals", params)
	return GetAllPages[MergeProposal](ctx, c, u)
}

// SearchBugTasks searches bug tasks on a project.
func (c *Client) SearchBugTasks(ctx context.Context, projectName string, opts BugTaskSearchOpts) ([]BugTask, error) {
	params := opts.values()
	u := wsOpURL(c.resolveURL("/"+projectName), "searchTasks", params)
	return GetAllPages[BugTask](ctx, c, u)
}

// BugTaskSearchOpts holds optional filters for searchTasks.
type BugTaskSearchOpts struct {
	Status         []string
	Importance     []string
	Assignee       string
	Tags           []string
	TagsCombinator string // "Any" or "All"
	SearchText     string
	Milestone      string
	OrderBy        []string
	CreatedSince   string
	ModifiedSince  string
	OmitDuplicates bool
}

func (o BugTaskSearchOpts) values() url.Values {
	v := url.Values{}
	for _, s := range o.Status {
		v.Add("status", s)
	}
	for _, s := range o.Importance {
		v.Add("importance", s)
	}
	if o.Assignee != "" {
		v.Set("assignee", o.Assignee)
	}
	for _, t := range o.Tags {
		v.Add("tags", t)
	}
	if o.TagsCombinator != "" {
		v.Set("tags_combinator", o.TagsCombinator)
	}
	if o.SearchText != "" {
		v.Set("search_text", o.SearchText)
	}
	if o.Milestone != "" {
		v.Set("milestone", o.Milestone)
	}
	for _, s := range o.OrderBy {
		v.Add("order_by", s)
	}
	if o.CreatedSince != "" {
		v.Set("created_since", o.CreatedSince)
	}
	if o.ModifiedSince != "" {
		v.Set("modified_since", o.ModifiedSince)
	}
	if o.OmitDuplicates {
		v.Set("omit_duplicates", "true")
	}
	return v
}

// GetProjectSeries returns the series for a project.
func (c *Client) GetProjectSeries(ctx context.Context, projectName string) (*Collection[ProjectSeries], error) {
	path := fmt.Sprintf("/%s/series", projectName)
	return GetCollection[ProjectSeries](ctx, c, path)
}

// ProjectSeries represents a project series (e.g. "trunk", "2.0").
type ProjectSeries struct {
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	Active      bool   `json:"active"`
	SelfLink    string `json:"self_link"`
	WebLink     string `json:"web_link"`
	ProjectLink string `json:"project_link"`
	OwnerLink   string `json:"owner_link"`
	DriverLink  string `json:"driver_link"`
	DateCreated *Time  `json:"date_created,omitempty"`
}
