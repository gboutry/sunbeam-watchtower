// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantNil bool
	}{
		{"RFC3339", `"2024-01-15T10:30:00Z"`, false, false},
		{"RFC3339 offset", `"2024-01-15T10:30:00+01:00"`, false, false},
		{"no timezone", `"2024-01-15T10:30:00"`, false, false},
		{"null", `null`, false, true},
		{"empty string", `""`, false, true},
		{"invalid", `"not-a-date"`, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts Time
			err := json.Unmarshal([]byte(tt.input), &ts)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil && !ts.IsZero() {
				t.Errorf("expected zero time, got %v", ts.Time)
			}
		})
	}
}

func TestTime_MarshalJSON(t *testing.T) {
	ts := Time{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)}
	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(data) != `"2024-01-15T10:30:00Z"` {
		t.Errorf("got %s", data)
	}
}

func TestTime_MarshalJSON_Zero(t *testing.T) {
	var ts Time
	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("got %s, want null", data)
	}
}

func TestCollection_Unmarshal(t *testing.T) {
	raw := `{
		"total_size": 2,
		"start": 0,
		"next_collection_link": "https://api.launchpad.net/devel/bugs?ws.start=2",
		"entries": [
			{"name": "alice", "self_link": "https://api.launchpad.net/devel/~alice"},
			{"name": "bob", "self_link": "https://api.launchpad.net/devel/~bob"}
		]
	}`

	var col Collection[Person]
	if err := json.Unmarshal([]byte(raw), &col); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if col.TotalSize != 2 {
		t.Errorf("TotalSize = %d, want 2", col.TotalSize)
	}
	if len(col.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(col.Entries))
	}
	if col.Entries[0].Name != "alice" {
		t.Errorf("Entries[0].Name = %q, want alice", col.Entries[0].Name)
	}
	if col.NextCollectionLink == "" {
		t.Error("expected NextCollectionLink to be set")
	}
}

func TestBug_Unmarshal(t *testing.T) {
	raw := `{
		"id": 12345,
		"title": "Something is broken",
		"description": "When I do X, Y happens",
		"self_link": "https://api.launchpad.net/devel/bugs/12345",
		"web_link": "https://bugs.launchpad.net/bugs/12345",
		"tags": ["regression", "ui"],
		"heat": 42,
		"status": "New",
		"importance": "High",
		"date_created": "2024-03-01T12:00:00Z",
		"message_count": 5,
		"private": false
	}`

	var b Bug
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if b.ID != 12345 {
		t.Errorf("ID = %d, want 12345", b.ID)
	}
	if b.Title != "Something is broken" {
		t.Errorf("Title = %q", b.Title)
	}
	if len(b.Tags) != 2 || b.Tags[0] != "regression" {
		t.Errorf("Tags = %v", b.Tags)
	}
	if b.DateCreated == nil || b.DateCreated.IsZero() {
		t.Error("DateCreated should be set")
	}
}

func TestProject_Unmarshal(t *testing.T) {
	raw := `{
		"name": "sunbeam",
		"display_name": "Sunbeam",
		"title": "Sunbeam OpenStack",
		"summary": "A minimal OpenStack deployment",
		"active": true,
		"vcs": "Git",
		"official_bugs": true,
		"official_bug_tags": ["sunbeam", "critical"],
		"owner_link": "https://api.launchpad.net/devel/~canonical-bootstack",
		"self_link": "https://api.launchpad.net/devel/sunbeam"
	}`

	var p Project
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if p.Name != "sunbeam" {
		t.Errorf("Name = %q", p.Name)
	}
	if !p.Active {
		t.Error("expected Active = true")
	}
	if p.VCS != "Git" {
		t.Errorf("VCS = %q", p.VCS)
	}
	if len(p.OfficialBugTags) != 2 {
		t.Errorf("OfficialBugTags = %v", p.OfficialBugTags)
	}
}

func TestGitRepository_Unmarshal(t *testing.T) {
	raw := `{
		"id": 999,
		"name": "sunbeam-charms",
		"display_name": "lp:~canonical-bootstack/sunbeam/+git/sunbeam-charms",
		"git_identity": "lp:~canonical-bootstack/sunbeam/+git/sunbeam-charms",
		"git_https_url": "https://git.launchpad.net/~canonical-bootstack/sunbeam/+git/sunbeam-charms",
		"default_branch": "refs/heads/main",
		"owner_default": false,
		"target_default": true,
		"private": false,
		"repository_type": "Hosted",
		"self_link": "https://api.launchpad.net/devel/~canonical-bootstack/sunbeam/+git/sunbeam-charms"
	}`

	var r GitRepository
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if r.ID != 999 {
		t.Errorf("ID = %d", r.ID)
	}
	if r.DefaultBranch != "refs/heads/main" {
		t.Errorf("DefaultBranch = %q", r.DefaultBranch)
	}
	if !r.TargetDefault {
		t.Error("expected TargetDefault = true")
	}
}
