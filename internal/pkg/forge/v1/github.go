// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v68/github"
)

// GitHubForge implements Forge for GitHub repositories.
type GitHubForge struct {
	client *github.Client
}

// NewGitHubForge creates a new GitHub forge client.
// The github.Client should already be configured with authentication.
func NewGitHubForge(client *github.Client) *GitHubForge {
	return &GitHubForge{client: client}
}

func (g *GitHubForge) Type() ForgeType {
	return ForgeGitHub
}

// parseOwnerRepo splits "owner/repo" into its parts.
func parseOwnerRepo(repo string) (string, string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid GitHub repo format %q, expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}

func (g *GitHubForge) ListMergeRequests(ctx context.Context, repo string, opts ListMergeRequestsOpts) ([]MergeRequest, error) {
	owner, repoName, err := parseOwnerRepo(repo)
	if err != nil {
		return nil, err
	}

	ghOpts := &github.PullRequestListOptions{
		State:       ghPRState(opts.State),
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var result []MergeRequest
	for {
		prs, resp, err := g.client.PullRequests.List(ctx, owner, repoName, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("listing GitHub PRs for %s: %w", repo, err)
		}

		for _, pr := range prs {
			mr := g.prToMergeRequest(repo, pr)
			if opts.Author != "" && mr.Author != opts.Author {
				continue
			}
			result = append(result, mr)
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return result, nil
}

func (g *GitHubForge) GetMergeRequest(ctx context.Context, repo string, id string) (*MergeRequest, error) {
	owner, repoName, err := parseOwnerRepo(repo)
	if err != nil {
		return nil, err
	}

	num, err := strconv.Atoi(strings.TrimPrefix(id, "#"))
	if err != nil {
		return nil, fmt.Errorf("invalid PR number %q: %w", id, err)
	}

	pr, _, err := g.client.PullRequests.Get(ctx, owner, repoName, num)
	if err != nil {
		return nil, fmt.Errorf("getting GitHub PR %s#%d: %w", repo, num, err)
	}

	mr := g.prToMergeRequest(repo, pr)

	// Fetch reviews to determine review state.
	reviews, _, err := g.client.PullRequests.ListReviews(ctx, owner, repoName, num, &github.ListOptions{PerPage: 100})
	if err == nil {
		mr.ReviewState = ghReviewState(reviews)
	}

	return &mr, nil
}

func (g *GitHubForge) ListCommits(ctx context.Context, repo string, opts ListCommitsOpts) ([]Commit, error) {
	owner, repoName, err := parseOwnerRepo(repo)
	if err != nil {
		return nil, err
	}

	ghOpts := &github.CommitsListOptions{
		SHA:         opts.Branch,
		Author:      opts.Author,
		ListOptions: github.ListOptions{PerPage: 100},
	}
	if opts.Since != nil {
		ghOpts.Since = *opts.Since
	}

	var result []Commit
	for {
		commits, resp, err := g.client.Repositories.ListCommits(ctx, owner, repoName, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("listing GitHub commits for %s: %w", repo, err)
		}

		for _, c := range commits {
			result = append(result, g.repoCommitToCommit(repo, c))
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return result, nil
}

func (g *GitHubForge) prToMergeRequest(repo string, pr *github.PullRequest) MergeRequest {
	mr := MergeRequest{
		Forge: ForgeGitHub,
		Repo:  repo,
		ID:    fmt.Sprintf("#%d", pr.GetNumber()),
		Title: pr.GetTitle(),
		Description: pr.GetBody(),
		URL:   pr.GetHTMLURL(),
	}

	if pr.User != nil {
		mr.Author = pr.User.GetLogin()
	}
	if pr.Head != nil {
		mr.SourceBranch = pr.Head.GetRef()
	}
	if pr.Base != nil {
		mr.TargetBranch = pr.Base.GetRef()
	}
	if pr.CreatedAt != nil {
		mr.CreatedAt = pr.CreatedAt.Time
	}
	if pr.UpdatedAt != nil {
		mr.UpdatedAt = pr.UpdatedAt.Time
	}

	mr.State = ghMergeState(pr)
	mr.ReviewState = ReviewStatePending

	return mr
}

func (g *GitHubForge) repoCommitToCommit(repo string, rc *github.RepositoryCommit) Commit {
	c := Commit{
		Forge: ForgeGitHub,
		Repo:  repo,
		SHA:   rc.GetSHA(),
		URL:   rc.GetHTMLURL(),
	}
	if rc.Commit != nil {
		c.Message = rc.Commit.GetMessage()
		if rc.Commit.Author != nil {
			c.Author = rc.Commit.Author.GetName()
			c.Date = rc.Commit.Author.GetDate().Time
		}
	}
	c.BugRefs = extractBugRefs(c.Message)
	return c
}

// ghPRState converts our MergeState to a GitHub API state string.
func ghPRState(state MergeState) string {
	switch state {
	case MergeStateClosed, MergeStateMerged:
		return "closed"
	case MergeStateOpen, MergeStateWIP:
		return "open"
	default:
		return "all"
	}
}

// ghMergeState converts a GitHub PR to our MergeState.
func ghMergeState(pr *github.PullRequest) MergeState {
	if pr.GetMerged() {
		return MergeStateMerged
	}
	if pr.GetDraft() {
		return MergeStateWIP
	}
	switch pr.GetState() {
	case "open":
		return MergeStateOpen
	case "closed":
		return MergeStateClosed
	default:
		return MergeStateOpen
	}
}

// ghReviewState determines the overall review state from a list of reviews.
func ghReviewState(reviews []*github.PullRequestReview) ReviewState {
	// Take the most recent decisive review per user.
	latest := make(map[string]string)
	for _, r := range reviews {
		if r.User == nil || r.State == nil {
			continue
		}
		user := r.User.GetLogin()
		state := r.GetState()
		// Only track decisive states.
		switch state {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
			latest[user] = state
		}
	}

	hasApproval := false
	for _, state := range latest {
		switch state {
		case "CHANGES_REQUESTED":
			return ReviewStateChangesRequested
		case "APPROVED":
			hasApproval = true
		}
	}
	if hasApproval {
		return ReviewStateApproved
	}
	return ReviewStatePending
}
