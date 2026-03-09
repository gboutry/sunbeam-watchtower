// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

func TestLPBugTaskToBugTaskUsesLatestTaskActivityForUpdatedAt(t *testing.T) {
	created := lp.Time{Time: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)}
	triaged := lp.Time{Time: created.Add(2 * time.Hour)}
	fixCommitted := lp.Time{Time: created.Add(5 * time.Hour)}

	got := lpBugTaskToBugTask(&lp.BugTask{
		BugLink:          "https://api.launchpad.net/devel/bugs/12345",
		BugTargetName:    "snap-openstack",
		Title:            "Fix auth flow",
		Status:           "Fix Committed",
		Importance:       "High",
		DateCreated:      &created,
		DateTriaged:      &triaged,
		DateFixCommitted: &fixCommitted,
	})

	if !got.CreatedAt.Equal(created.Time) {
		t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, created.Time)
	}
	if !got.UpdatedAt.Equal(fixCommitted.Time) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, fixCommitted.Time)
	}
}

func TestLaunchpadBugTrackerListBugTasksUsesCreatedOrModifiedSince(t *testing.T) {
	createdTask := lp.BugTask{
		SelfLink:      "https://api.launchpad.net/devel/sunbeam-charms/+bug/1",
		BugLink:       "https://api.launchpad.net/devel/bugs/1",
		BugTargetName: "sunbeam-charms",
		Title:         "Created recently",
		Status:        "New",
		DateCreated:   &lp.Time{Time: time.Date(2026, 3, 9, 8, 30, 0, 0, time.UTC)},
	}
	modifiedTask := lp.BugTask{
		SelfLink:         "https://api.launchpad.net/devel/sunbeam-charms/+bug/2",
		BugLink:          "https://api.launchpad.net/devel/bugs/2",
		BugTargetName:    "sunbeam-charms",
		Title:            "Modified recently",
		Status:           "Fix Released",
		DateCreated:      &lp.Time{Time: time.Date(2024, 7, 25, 5, 23, 26, 0, time.UTC)},
		DateFixReleased:  &lp.Time{Time: time.Date(2026, 3, 9, 9, 48, 58, 0, time.UTC)},
		DateClosed:       &lp.Time{Time: time.Date(2026, 3, 9, 9, 48, 58, 0, time.UTC)},
		DateFixCommitted: &lp.Time{Time: time.Date(2024, 7, 26, 1, 58, 21, 0, time.UTC)},
	}

	var seen []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query())
		response := lp.Collection[lp.BugTask]{Start: 0}
		switch {
		case r.URL.Query().Get("created_since") != "" && r.URL.Query().Get("modified_since") == "":
			response.Entries = []lp.BugTask{createdTask}
		case r.URL.Query().Get("modified_since") != "" && r.URL.Query().Get("created_since") == "":
			response.Entries = []lp.BugTask{modifiedTask}
		default:
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	rewriteClient := server.Client()
	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse(server.URL): %v", err)
	}
	rewriteClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		return http.DefaultTransport.RoundTrip(req)
	})

	client := lp.NewClient(nil, slog.Default(), rewriteClient)
	tracker := NewLaunchpadBugTracker(client)

	tasks, err := tracker.ListBugTasks(context.Background(), "sunbeam-charms", ListBugTasksOpts{
		CreatedSince:  "2026-03-09T08:00:00Z",
		ModifiedSince: "2026-03-09T08:00:00Z",
	})
	if err != nil {
		t.Fatalf("ListBugTasks() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %d, want 2", len(seen))
	}
	for _, query := range seen {
		if query.Get("created_since") != "" && query.Get("modified_since") != "" {
			t.Fatalf("query unexpectedly sent both created_since and modified_since: %v", query)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
