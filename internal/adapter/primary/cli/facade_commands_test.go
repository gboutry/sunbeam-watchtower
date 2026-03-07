// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

func TestConfigShowCmd_RendersYAML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/config" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(dto.Config{
			Build: dto.BuildConfig{DefaultPrefix: "tmp-build"},
		})
	}))
	defer ts.Close()

	var out bytes.Buffer
	opts := &Options{
		Out:    &out,
		ErrOut: &bytes.Buffer{},
		Output: "yaml",
		Client: client.NewClient(ts.URL),
	}

	cmd := newConfigCmd(opts)
	cmd.SetArgs([]string{"show"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "default_prefix: tmp-build") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestCacheStatusCmd_RendersCacheStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/cache/status" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(client.CacheStatusResult{
			Git: struct {
				Directory string              `json:"directory"`
				Repos     []client.CacheEntry `json:"repos"`
			}{
				Directory: "/tmp/git",
				Repos:     []client.CacheEntry{{Name: "keystone", Size: "4.0 MiB"}},
			},
		})
	}))
	defer ts.Close()

	var out bytes.Buffer
	opts := &Options{
		Out:    &out,
		ErrOut: &bytes.Buffer{},
		Output: "table",
		Client: client.NewClient(ts.URL),
		Logger: discardTestLogger(),
	}

	cmd := newCacheCmd(opts)
	cmd.SetArgs([]string{"status"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "/tmp/git") || !strings.Contains(out.String(), "keystone") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestBugListCmd_RendersWarningsAndTasks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/bugs" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tasks": []forge.BugTask{{
				Project: "keystone",
				BugID:   "12345",
				Title:   "Fix auth flow",
				Status:  "Triaged",
			}},
			"warnings": []string{"partial tracker failure"},
		})
	}))
	defer ts.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	opts := &Options{
		Out:    &out,
		ErrOut: &errOut,
		Output: "table",
		Client: client.NewClient(ts.URL),
	}

	cmd := newBugCmd(opts)
	cmd.SetArgs([]string{"list"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "12345") || !strings.Contains(errOut.String(), "partial tracker failure") {
		t.Fatalf("unexpected output: out=%q err=%q", out.String(), errOut.String())
	}
}

func TestReviewListCmd_RendersWarningsAndMergeRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/reviews" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"merge_requests": []forge.MergeRequest{{
				ID:    "42",
				Repo:  "keystone",
				Title: "Refactor auth",
				URL:   "https://example.invalid/42",
			}},
			"warnings": []string{"forge timeout"},
		})
	}))
	defer ts.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	opts := &Options{
		Out:    &out,
		ErrOut: &errOut,
		Output: "table",
		Client: client.NewClient(ts.URL),
		Logger: discardTestLogger(),
	}

	cmd := newReviewCmd(opts)
	cmd.SetArgs([]string{"list"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "Refactor auth") || !strings.Contains(errOut.String(), "forge timeout") {
		t.Fatalf("unexpected output: out=%q err=%q", out.String(), errOut.String())
	}
}

func TestCommitTrackCmd_RendersWarningsAndCommits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/commits/track" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commits": []forge.Commit{{
				SHA:     "deadbeef",
				Repo:    "keystone",
				Message: "LP: #12345 fix auth workflow",
				URL:     "https://example.invalid/deadbeef",
			}},
			"warnings": []string{"partial history"},
		})
	}))
	defer ts.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	opts := &Options{
		Out:    &out,
		ErrOut: &errOut,
		Output: "table",
		Client: client.NewClient(ts.URL),
		Logger: discardTestLogger(),
	}

	cmd := newCommitCmd(opts)
	cmd.SetArgs([]string{"track", "--bug-id", "12345"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(out.String(), "deadbeef") || !strings.Contains(errOut.String(), "partial history") {
		t.Fatalf("unexpected output: out=%q err=%q", out.String(), errOut.String())
	}
}
