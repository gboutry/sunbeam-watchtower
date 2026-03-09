// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

func TestBuildBugTrackersCreatesLaunchpadTrackerWithoutStoredAuth(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	application := NewApp(&config.Config{
		Projects: []config.ProjectConfig{{
			Name: "openstack",
			Bugs: []config.BugTrackerConfig{{
				Forge:   "launchpad",
				Project: "snap-openstack",
			}},
		}},
	}, slog.Default())

	trackers, projectMap, err := application.BuildBugTrackers()
	if err != nil {
		t.Fatalf("BuildBugTrackers() error = %v", err)
	}
	if len(trackers) != 1 {
		t.Fatalf("len(trackers) = %d, want 1", len(trackers))
	}
	if got := projectMap["launchpad:snap-openstack"]; len(got) != 1 || got[0] != "openstack" {
		t.Fatalf("projectMap = %+v, want openstack mapping", projectMap)
	}
}

func TestBuildBugTrackersFallsBackToCachedLaunchpadProjects(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	application := NewApp(&config.Config{}, slog.Default())
	cache, err := application.BugCache()
	if err != nil {
		t.Fatalf("BugCache() error = %v", err)
	}

	ctx := context.Background()
	project := "snap-openstack"
	task := forge.BugTask{
		Forge:      forge.ForgeLaunchpad,
		BugID:      "12345",
		TargetName: project,
		Status:     "Fix Released",
		CreatedAt:  time.Date(2026, 3, 8, 9, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 3, 9, 9, 0, 0, 0, time.UTC),
	}
	if err := cache.StoreBugTasks(ctx, forge.ForgeLaunchpad, project, []forge.BugTask{task}); err != nil {
		t.Fatalf("StoreBugTasks() error = %v", err)
	}
	if err := cache.SetLastSync(ctx, forge.ForgeLaunchpad, project, time.Date(2026, 3, 9, 9, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetLastSync() error = %v", err)
	}

	trackers, projectMap, err := application.BuildBugTrackers()
	if err != nil {
		t.Fatalf("BuildBugTrackers() error = %v", err)
	}
	if len(trackers) != 1 {
		t.Fatalf("len(trackers) = %d, want 1", len(trackers))
	}

	tracker, ok := trackers["launchpad:"+project]
	if !ok {
		t.Fatalf("trackers = %+v, want cached launchpad tracker for %s", trackers, project)
	}
	tasks, err := tracker.Tracker.ListBugTasks(ctx, project, forge.ListBugTasksOpts{Status: []string{"Fix Released"}})
	if err != nil {
		t.Fatalf("ListBugTasks() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if got := projectMap["launchpad:"+project]; len(got) != 1 || got[0] != project {
		t.Fatalf("projectMap = %+v, want cache-backed project mapping", projectMap)
	}
}
