// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"fmt"
	"strconv"

	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
)

// LaunchpadBugTracker implements BugTracker for Launchpad projects.
type LaunchpadBugTracker struct {
	client *lp.Client
}

// NewLaunchpadBugTracker creates a new Launchpad bug tracker.
func NewLaunchpadBugTracker(client *lp.Client) *LaunchpadBugTracker {
	return &LaunchpadBugTracker{client: client}
}

func (l *LaunchpadBugTracker) Type() ForgeType {
	return ForgeLaunchpad
}

func (l *LaunchpadBugTracker) GetBug(ctx context.Context, id string) (*Bug, error) {
	bugID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid LP bug ID %q: %w", id, err)
	}

	lpBug, err := l.client.GetBug(ctx, bugID)
	if err != nil {
		return nil, err
	}

	lpTasks, err := l.client.GetBugTasks(ctx, bugID)
	if err != nil {
		return nil, fmt.Errorf("fetching tasks for bug %d: %w", bugID, err)
	}

	b := &Bug{
		Forge:       ForgeLaunchpad,
		ID:          id,
		Title:       lpBug.Title,
		Description: lpBug.Description,
		Owner:       lpExtractName(lpBug.OwnerLink),
		Tags:        lpBug.Tags,
		URL:         lpBug.WebLink,
	}
	if lpBug.DateCreated != nil {
		b.CreatedAt = lpBug.DateCreated.Time
	}
	if lpBug.DateLastUpdated != nil {
		b.UpdatedAt = lpBug.DateLastUpdated.Time
	}

	for _, t := range lpTasks {
		b.Tasks = append(b.Tasks, lpBugTaskToBugTask(&t))
	}

	return b, nil
}

func (l *LaunchpadBugTracker) ListBugTasks(ctx context.Context, project string, opts ListBugTasksOpts) ([]BugTask, error) {
	lpOpts := lp.BugTaskSearchOpts{
		Status:         opts.Status,
		Importance:     opts.Importance,
		Tags:           opts.Tags,
		CreatedSince:   opts.CreatedSince,
		OmitDuplicates: true,
	}
	if opts.Assignee != "" {
		lpOpts.Assignee = "https://api.launchpad.net/devel/~" + opts.Assignee
	}

	tasks, err := l.client.SearchBugTasks(ctx, project, lpOpts)
	if err != nil {
		return nil, fmt.Errorf("searching LP bug tasks for %s: %w", project, err)
	}

	result := make([]BugTask, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, lpBugTaskToBugTask(&t))
	}
	return result, nil
}

func (l *LaunchpadBugTracker) UpdateBugTaskStatus(ctx context.Context, taskSelfLink, status string) error {
	return l.client.UpdateBugTaskStatus(ctx, taskSelfLink, status)
}

func (l *LaunchpadBugTracker) NominateBug(ctx context.Context, bugID int, seriesSelfLink string) error {
	return l.client.NominateBug(ctx, bugID, seriesSelfLink)
}

func (l *LaunchpadBugTracker) GetProjectSeries(ctx context.Context, projectName string) ([]ProjectSeries, error) {
	col, err := l.client.GetProjectSeries(ctx, projectName)
	if err != nil {
		return nil, fmt.Errorf("fetching series for %s: %w", projectName, err)
	}
	result := make([]ProjectSeries, 0, len(col.Entries))
	for _, s := range col.Entries {
		result = append(result, ProjectSeries{
			Name:     s.Name,
			SelfLink: s.SelfLink,
			Active:   s.Active,
		})
	}
	return result, nil
}

func (l *LaunchpadBugTracker) GetProject(ctx context.Context, projectName string) (*Project, error) {
	p, err := l.client.GetProject(ctx, projectName)
	if err != nil {
		return nil, err
	}
	return &Project{
		Name:                 p.Name,
		DevelopmentFocusLink: p.DevelopmentFocusLink,
	}, nil
}

func lpBugTaskToBugTask(t *lp.BugTask) BugTask {
	bt := BugTask{
		Forge:      ForgeLaunchpad,
		BugID:      lpExtractBugID(t.BugLink),
		Title:      t.Title,
		Status:     t.Status,
		Importance: t.Importance,
		Assignee:   lpExtractName(t.AssigneeLink),
		URL:        t.WebLink,
		SelfLink:   t.SelfLink,
		TargetName: t.BugTargetName,
	}
	if t.DateCreated != nil {
		bt.CreatedAt = t.DateCreated.Time
	}
	// LP BugTask doesn't have a direct "updated" field; use DateCreated as fallback.
	// The title field contains bug info but the last-updated comes from the bug itself.
	if t.DateCreated != nil {
		bt.UpdatedAt = t.DateCreated.Time
	}
	return bt
}

// lpExtractBugID extracts the bug ID from a bug link.
// "https://api.launchpad.net/devel/bugs/12345" -> "12345"
func lpExtractBugID(link string) string {
	if link == "" {
		return ""
	}
	for i := len(link) - 1; i >= 0; i-- {
		if link[i] == '/' {
			id := link[i+1:]
			if _, err := strconv.Atoi(id); err == nil {
				return id
			}
			return id
		}
	}
	return link
}
