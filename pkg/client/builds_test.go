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

func TestBuildsTriggerPrefersTargetProject(t *testing.T) {
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
		LPProject:     "legacy-project",
	})
	if err != nil {
		t.Fatalf("BuildsTrigger() error = %v", err)
	}
	if body["target_project"] != "target-project" {
		t.Fatalf("target_project = %v, want target-project", body["target_project"])
	}
	if _, ok := body["lp_project"]; ok {
		t.Fatalf("lp_project should not be sent when target_project is set: %+v", body)
	}
}

func TestBuildsListFallsBackToLegacyLPProjectQuery(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"builds":[]}`))
	}))
	defer ts.Close()

	_, err := NewClient(ts.URL).BuildsList(context.Background(), BuildsListOptions{
		LPProject: "legacy-project",
	})
	if err != nil {
		t.Fatalf("BuildsList() error = %v", err)
	}
	if !strings.Contains(rawQuery, "lp_project=legacy-project") {
		t.Fatalf("query = %q, want legacy lp_project", rawQuery)
	}
}

func TestBuildsDownloadPrefersTargetProject(t *testing.T) {
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
		LPProject:     "legacy-project",
	})
	if err != nil {
		t.Fatalf("BuildsDownload() error = %v", err)
	}
	if body["target_project"] != "target-project" {
		t.Fatalf("target_project = %v, want target-project", body["target_project"])
	}
	if _, ok := body["lp_project"]; ok {
		t.Fatalf("lp_project should not be sent when target_project is set: %+v", body)
	}
}
