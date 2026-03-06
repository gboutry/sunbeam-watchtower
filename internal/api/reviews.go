package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/service/review"
)

// --- List reviews ---

// ReviewsListInput holds query parameters for listing merge requests.
type ReviewsListInput struct {
	Projects []string `query:"project" doc:"Filter by project name"`
	Forges   []string `query:"forge" doc:"Filter by forge type: github, launchpad, gerrit"`
	State    string   `query:"state" doc:"Filter by state: open, merged, closed, wip, abandoned"`
	Author   string   `query:"author" doc:"Filter by author"`
}

// ReviewsListOutput is the response for listing merge requests.
type ReviewsListOutput struct {
	Body struct {
		MergeRequests []forge.MergeRequest `json:"merge_requests" doc:"Merge requests matching filters"`
		Warnings      []string             `json:"warnings,omitempty" doc:"Non-fatal warnings"`
	}
}

// --- Get review ---

// ReviewGetInput holds path parameters for getting a single merge request.
type ReviewGetInput struct {
	Project string `path:"project" doc:"Project name"`
	ID      string `path:"id" doc:"Merge request ID"`
}

// ReviewGetOutput is the response for getting a single merge request.
type ReviewGetOutput struct {
	Body *forge.MergeRequest
}

// RegisterReviewsAPI registers the /api/v1/reviews endpoints on the given huma API.
func RegisterReviewsAPI(api huma.API, application *app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "list-reviews",
		Method:      http.MethodGet,
		Path:        "/api/v1/reviews",
		Summary:     "List merge requests",
		Description: "List merge requests across all configured forges, with optional filters.",
		Tags:        []string{"reviews"},
	}, func(ctx context.Context, input *ReviewsListInput) (*ReviewsListOutput, error) {
		clients, err := application.BuildForgeClients()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build forge clients", err)
		}

		svc := review.NewService(clients, application.Logger)

		listOpts := review.ListOptions{
			Projects: input.Projects,
			Author:   input.Author,
		}

		if input.State != "" {
			s, err := parseAPIMergeState(input.State)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			listOpts.State = s
		}

		for _, f := range input.Forges {
			ft, err := parseAPIForgeType(f)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			listOpts.Forges = append(listOpts.Forges, ft)
		}

		mrs, results, err := svc.List(ctx, listOpts)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list reviews", err)
		}

		out := &ReviewsListOutput{}
		out.Body.MergeRequests = mrs
		for _, r := range results {
			if r.Err != nil {
				out.Body.Warnings = append(out.Body.Warnings, r.Err.Error())
			}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-review",
		Method:      http.MethodGet,
		Path:        "/api/v1/reviews/{project}/{id}",
		Summary:     "Get a merge request",
		Description: "Retrieve a single merge request by project name and ID.",
		Tags:        []string{"reviews"},
	}, func(ctx context.Context, input *ReviewGetInput) (*ReviewGetOutput, error) {
		clients, err := application.BuildForgeClients()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build forge clients", err)
		}

		svc := review.NewService(clients, application.Logger)

		mr, err := svc.Get(ctx, input.Project, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("review %s/%s not found", input.Project, input.ID), err)
		}

		out := &ReviewGetOutput{}
		out.Body = mr
		return out, nil
	})
}

func parseAPIMergeState(s string) (forge.MergeState, error) {
	switch strings.ToLower(s) {
	case "open":
		return forge.MergeStateOpen, nil
	case "merged":
		return forge.MergeStateMerged, nil
	case "closed":
		return forge.MergeStateClosed, nil
	case "wip":
		return forge.MergeStateWIP, nil
	case "abandoned":
		return forge.MergeStateAbandoned, nil
	default:
		return 0, fmt.Errorf("invalid state %q (valid: open, merged, closed, wip, abandoned)", s)
	}
}

func parseAPIForgeType(s string) (forge.ForgeType, error) {
	switch strings.ToLower(s) {
	case "github":
		return forge.ForgeGitHub, nil
	case "launchpad":
		return forge.ForgeLaunchpad, nil
	case "gerrit":
		return forge.ForgeGerrit, nil
	default:
		return 0, fmt.Errorf("invalid forge %q (valid: github, launchpad, gerrit)", s)
	}
}
