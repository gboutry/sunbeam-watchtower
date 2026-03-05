// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"time"
)

// Bug is a forge-agnostic bug with its associated tasks.
type Bug struct {
	Forge       ForgeType
	ID          string // "12345" for LP, "#123" for GH
	Title       string
	Description string
	Owner       string
	Tags        []string
	URL         string
	Tasks       []BugTask
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// BugTask is a forge-agnostic bug task.
type BugTask struct {
	Forge      ForgeType
	Project    string // watchtower project name
	BugID      string // bug identifier ("12345" for LP, "#123" for GH)
	Title      string
	Status     string // "New", "Confirmed", "In Progress", etc.
	Importance string // "Critical", "High", "Medium", "Low", etc.
	Assignee   string
	Tags       []string
	URL        string
	SelfLink   string // API self link for updates (LP-specific)
	TargetName string // LP project slug (e.g. "snap-openstack")
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
