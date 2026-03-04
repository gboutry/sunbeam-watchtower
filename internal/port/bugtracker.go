// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
)

// BugTracker is the interface for querying bug trackers.
type BugTracker interface {
	Type() forge.ForgeType
	GetBug(ctx context.Context, id string) (*forge.Bug, error)
	ListBugTasks(ctx context.Context, project string, opts forge.ListBugTasksOpts) ([]forge.BugTask, error)
}
