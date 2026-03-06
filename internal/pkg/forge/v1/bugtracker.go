// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"time"
)

// Bug is a forge-agnostic bug with its associated tasks.
type Bug struct {
	Forge       ForgeType `json:"forge"`
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	Tags        []string  `json:"tags,omitempty"`
	URL         string    `json:"url"`
	Tasks       []BugTask `json:"tasks"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BugTask is a forge-agnostic bug task.
type BugTask struct {
	Forge      ForgeType `json:"forge"`
	Project    string    `json:"project"`
	BugID      string    `json:"bug_id"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Importance string    `json:"importance"`
	Assignee   string    `json:"assignee,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	URL        string    `json:"url"`
	SelfLink   string    `json:"self_link,omitempty"`
	TargetName string    `json:"target_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProjectSeries is a forge-agnostic project series.
type ProjectSeries struct {
	Name     string
	SelfLink string
	Active   bool
}

// Project is a forge-agnostic project.
type Project struct {
	Name                 string
	SelfLink             string
	DevelopmentFocusLink string
}

// ListBugTasksOpts holds options for listing bug tasks.
type ListBugTasksOpts struct {
	Status       []string
	Importance   []string
	Assignee     string
	Tags         []string
	CreatedSince string // ISO 8601 date string for filtering by creation date
}

// BugTracker is the interface for querying and updating bug trackers.
type BugTracker interface {
	// Type returns which forge this bug tracker represents.
	Type() ForgeType

	// GetBug returns a bug by ID with its tasks.
	GetBug(ctx context.Context, id string) (*Bug, error)

	// ListBugTasks returns bug tasks for the given project.
	ListBugTasks(ctx context.Context, project string, opts ListBugTasksOpts) ([]BugTask, error)

	// UpdateBugTaskStatus updates the status of a bug task.
	UpdateBugTaskStatus(ctx context.Context, taskSelfLink, status string) error

	// AddBugTask adds a bug task targeting the given series.
	AddBugTask(ctx context.Context, bugID int, seriesSelfLink string) error

	// GetProjectSeries returns the series for a project.
	GetProjectSeries(ctx context.Context, projectName string) ([]ProjectSeries, error)

	// GetProject returns project information.
	GetProject(ctx context.Context, projectName string) (*Project, error)
}
