package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/commit"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// --- List commits ---

// CommitsListInput holds query parameters for listing commits.
type CommitsListInput struct {
	Projects   []string `query:"project" required:"false" doc:"Filter by project name"`
	Forges     []string `query:"forge" required:"false" doc:"Filter by forge type: github, launchpad, gerrit"`
	Branch     string   `query:"branch" doc:"Branch to list commits from"`
	Author     string   `query:"author" doc:"Filter by author"`
	IncludeMRs bool     `query:"include_mrs" required:"false" doc:"Include commits from merge request refs"`
}

// CommitsListOutput is the response for listing commits.
type CommitsListOutput struct {
	Body struct {
		Commits  []forge.Commit `json:"commits" doc:"Commits matching filters"`
		Warnings []string       `json:"warnings,omitempty" doc:"Non-fatal warnings"`
	}
}

// --- Track commits ---

// CommitsTrackInput holds query parameters for tracking commits by bug ID.
type CommitsTrackInput struct {
	BugID      string   `query:"bug_id" required:"true" doc:"Bug ID to track"`
	Projects   []string `query:"project" required:"false" doc:"Filter by project name"`
	Forges     []string `query:"forge" required:"false" doc:"Filter by forge type: github, launchpad, gerrit"`
	Branch     string   `query:"branch" doc:"Branch to list commits from"`
	IncludeMRs bool     `query:"include_mrs" required:"false" doc:"Include commits from merge request refs"`
}

// CommitsTrackOutput is the response for tracking commits by bug ID.
type CommitsTrackOutput struct {
	Body struct {
		Commits  []forge.Commit `json:"commits" doc:"Commits referencing the bug"`
		Warnings []string       `json:"warnings,omitempty" doc:"Non-fatal warnings"`
	}
}

// RegisterCommitsAPI registers the /api/v1/commits endpoints on the given huma API.
func RegisterCommitsAPI(api huma.API, application *app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "list-commits",
		Method:      http.MethodGet,
		Path:        "/api/v1/commits",
		Summary:     "List commits",
		Description: "List commits across all configured forges, with optional filters.",
		Tags:        []string{"commits"},
	}, func(ctx context.Context, input *CommitsListInput) (*CommitsListOutput, error) {
		return listCommits(ctx, application, commit.ListOptions{
			Projects:   input.Projects,
			Branch:     input.Branch,
			Author:     input.Author,
			IncludeMRs: input.IncludeMRs,
		}, input.Forges)
	})

	huma.Register(api, huma.Operation{
		OperationID: "track-commits",
		Method:      http.MethodGet,
		Path:        "/api/v1/commits/track",
		Summary:     "Track commits by bug ID",
		Description: "Find commits referencing a specific bug ID across all configured forges.",
		Tags:        []string{"commits"},
	}, func(ctx context.Context, input *CommitsTrackInput) (*CommitsTrackOutput, error) {
		out, err := listCommits(ctx, application, commit.ListOptions{
			Projects:   input.Projects,
			Branch:     input.Branch,
			BugID:      input.BugID,
			IncludeMRs: input.IncludeMRs,
		}, input.Forges)
		if err != nil {
			return nil, err
		}
		// Repackage into CommitsTrackOutput (identical structure).
		trackOut := &CommitsTrackOutput{}
		trackOut.Body.Commits = out.Body.Commits
		trackOut.Body.Warnings = out.Body.Warnings
		return trackOut, nil
	})
}

// listCommits is the shared implementation for list and track endpoints.
func listCommits(ctx context.Context, application *app.App, opts commit.ListOptions, forges []string) (*CommitsListOutput, error) {
	sources, err := application.BuildCommitSources()
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to build commit sources: %v", err))
	}

	for _, f := range forges {
		ft, err := parseAPIForgeType(f)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		opts.Forges = append(opts.Forges, ft)
	}

	svc := commit.NewService(sources, application.Logger)

	commits, results, err := svc.List(ctx, opts)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to list commits: %v", err))
	}

	out := &CommitsListOutput{}
	out.Body.Commits = commits
	for _, r := range results {
		if r.Err != nil {
			out.Body.Warnings = append(out.Body.Warnings, r.Err.Error())
		}
	}
	return out, nil
}
