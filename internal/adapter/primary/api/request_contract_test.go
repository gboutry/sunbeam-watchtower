package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
)

func TestCommitsList_OmittedOptionalQueryParamsReturns200(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterCommitsAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/commits")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Commits []any `json:"commits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Commits) != 0 {
		t.Fatalf("expected no commits, got %d", len(body.Commits))
	}
}

func TestReviewsList_OmittedOptionalQueryParamsReturns200(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterReviewsAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/reviews")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		MergeRequests []any `json:"merge_requests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.MergeRequests) != 0 {
		t.Fatalf("expected no merge requests, got %d", len(body.MergeRequests))
	}
}

func TestBugsList_OmittedOptionalQueryParamsReturns200(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterBugsAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/bugs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Tasks []any `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Tasks) != 0 {
		t.Fatalf("expected no bug tasks, got %d", len(body.Tasks))
	}
}

func TestBuildsList_OmittedOptionalQueryParamsReturns200(t *testing.T) {
	srv, base := startTestServer(t)
	defer srv.Shutdown(context.Background())

	application := app.NewApp(&config.Config{}, discardLogger())
	RegisterBuildsAPI(srv.API(), application)

	resp, err := http.Get(base + "/api/v1/builds")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Builds []any `json:"builds"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Builds) != 0 {
		t.Fatalf("expected no builds, got %d", len(body.Builds))
	}
}
