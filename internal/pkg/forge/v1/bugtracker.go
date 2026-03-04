// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"time"
)

// BugTask is a forge-agnostic bug task.
type BugTask struct {
	Forge      ForgeType
	Project    string   // watchtower project name
	BugID      string   // bug identifier ("12345" for LP, "#123" for GH)
	Title      string
	Status     string   // "New", "Confirmed", "In Progress", etc.
	Importance string   // "Critical", "High", "Medium", "Low", etc.
	Assignee   string
	Tags       []string
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ListBugTasksOpts holds options for listing bug tasks.
type ListBugTasksOpts struct {
	Status     []string
	Importance []string
	Assignee   string
	Tags       []string
}

// BugTracker is the interface for querying bug trackers.
type BugTracker interface {
	// Type returns which forge this bug tracker represents.
	Type() ForgeType

	// ListBugTasks returns bug tasks for the given project.
	ListBugTasks(ctx context.Context, project string, opts ListBugTasksOpts) ([]BugTask, error)
}
