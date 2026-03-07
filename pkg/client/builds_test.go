// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildsTriggerUsesTargetProject(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_, _ = w.Write([]byte(`{"project":"demo","recipe_results":[]}`))
	}))
	defer ts.Close()

	_, err := NewClient(ts.URL).BuildsTrigger(context.Background(), BuildsTriggerOptions{
		Project:       "demo",
		TargetProject: "target-project",
	})
	if err != nil {
		t.Fatalf("BuildsTrigger() error = %v", err)
	}
	if body["target_project"] != "target-project" {
		t.Fatalf("target_project = %v, want target-project", body["target_project"])
	}
}

func TestBuildsListUsesTargetProjectQuery(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"builds":[]}`))
	}))
	defer ts.Close()

	_, err := NewClient(ts.URL).BuildsList(context.Background(), BuildsListOptions{
		TargetProject: "target-project",
	})
	if err != nil {
		t.Fatalf("BuildsList() error = %v", err)
	}
	if !strings.Contains(rawQuery, "target_project=target-project") {
		t.Fatalf("query = %q, want target_project", rawQuery)
	}
}

func TestBuildsDownloadUsesTargetProject(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	err := NewClient(ts.URL).BuildsDownload(context.Background(), BuildsDownloadOptions{
		Project:       "demo",
		TargetProject: "target-project",
	})
	if err != nil {
		t.Fatalf("BuildsDownload() error = %v", err)
	}
	if body["target_project"] != "target-project" {
		t.Fatalf("target_project = %v, want target-project", body["target_project"])
	}
}

func TestBuildsTriggerAsyncUsesTargetProject(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"op-1","kind":"build.trigger","state":"queued"}`))
	}))
	defer ts.Close()

	got, err := NewClient(ts.URL).BuildsTriggerAsync(context.Background(), BuildsTriggerOptions{
		Project:       "demo",
		TargetProject: "target-project",
	})
	if err != nil {
		t.Fatalf("BuildsTriggerAsync() error = %v", err)
	}
	if got.ID != "op-1" {
		t.Fatalf("job = %+v, want op-1", got)
	}
	if body["target_project"] != "target-project" {
		t.Fatalf("target_project = %v, want target-project", body["target_project"])
	}
}
