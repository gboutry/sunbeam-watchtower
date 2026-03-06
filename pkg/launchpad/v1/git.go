// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"fmt"
	"net/url"
)

// CreateGitRepository creates a new Git repository on Launchpad.
func (c *Client) CreateGitRepository(ctx context.Context, owner, project, name string) (GitRepository, error) {
	form := url.Values{
		"ws.op":  {"new"},
		"owner":  {c.resolveURL("/~" + owner)},
		"target": {c.resolveURL("/" + project)},
		"name":   {name},
	}
	// LP returns 201 with an empty body and a Location header, so we
	// cannot use PostJSON.  Instead we POST, then GET the new resource.
	if _, err := c.Post(ctx, "/+git", form); err != nil {
		return GitRepository{}, fmt.Errorf("creating git repo ~%s/%s/+git/%s: %w", owner, project, name, err)
	}
	return c.GetGitRepository(ctx, owner, project, name)
}

// GetGitRepository fetches a Git repository by owner, project, and repo name.
// Path: /~<owner>/<project>/+git/<name>
func (c *Client) GetGitRepository(ctx context.Context, owner, project, name string) (GitRepository, error) {
	var r GitRepository
	path := fmt.Sprintf("/~%s/%s/+git/%s", owner, project, name)
	if err := c.GetJSON(ctx, path, &r); err != nil {
		return GitRepository{}, fmt.Errorf("fetching git repo %s/%s/+git/%s: %w", owner, project, name, err)
	}
	return r, nil
}

// GetGitRepositoryByLink fetches a Git repository using its self_link.
func (c *Client) GetGitRepositoryByLink(ctx context.Context, selfLink string) (GitRepository, error) {
	var r GitRepository
	if err := c.GetJSON(ctx, selfLink, &r); err != nil {
		return GitRepository{}, fmt.Errorf("fetching git repo: %w", err)
	}
	return r, nil
}

// GetGitRef fetches a specific ref from a Git repository.
// refPath should be a full ref path like "refs/heads/main".
func (c *Client) GetGitRef(ctx context.Context, repoSelfLink, refPath string) (GitRef, error) {
	u := wsOpURL(repoSelfLink, "getRefByPath", url.Values{
		"path": {refPath},
	})
	var ref GitRef
	if err := c.GetJSON(ctx, u, &ref); err != nil {
		return GitRef{}, fmt.Errorf("fetching git ref %q: %w", refPath, err)
	}
	return ref, nil
}

// GetGitBranches returns branch refs from a Git repository.
func (c *Client) GetGitBranches(ctx context.Context, repoSelfLink string) ([]GitRef, error) {
	return GetAllPages[GitRef](ctx, c, repoSelfLink+"/branches")
}

// GetGitRefs returns all refs from a Git repository.
func (c *Client) GetGitRefs(ctx context.Context, repoSelfLink string) ([]GitRef, error) {
	return GetAllPages[GitRef](ctx, c, repoSelfLink+"/refs")
}

// GetGitRepoMergeProposals returns merge proposals for a Git repository.
func (c *Client) GetGitRepoMergeProposals(ctx context.Context, repoSelfLink string, status ...string) ([]MergeProposal, error) {
	params := url.Values{}
	for _, s := range status {
		params.Add("status", s)
	}
	u := wsOpURL(repoSelfLink, "getMergeProposals", params)
	return GetAllPages[MergeProposal](ctx, c, u)
}

// GetGitRefMergeProposals returns merge proposals where a ref is the source.
func (c *Client) GetGitRefMergeProposals(ctx context.Context, refSelfLink string, status ...string) ([]MergeProposal, error) {
	params := url.Values{}
	for _, s := range status {
		params.Add("status", s)
	}
	u := wsOpURL(refSelfLink, "getMergeProposals", params)
	return GetAllPages[MergeProposal](ctx, c, u)
}

// GetMergeProposal fetches a merge proposal by its self_link.
func (c *Client) GetMergeProposal(ctx context.Context, selfLink string) (MergeProposal, error) {
	var mp MergeProposal
	if err := c.GetJSON(ctx, selfLink, &mp); err != nil {
		return MergeProposal{}, fmt.Errorf("fetching merge proposal: %w", err)
	}
	return mp, nil
}

// GetDefaultRepository returns the default git repository for a Launchpad project.
// It calls the getDefaultRepository operation on the /+git endpoint.
func (c *Client) GetDefaultRepository(ctx context.Context, projectSelfLink string) (GitRepository, error) {
	u := wsOpURL(c.resolveURL("/+git"), "getDefaultRepository", url.Values{
		"target": {projectSelfLink},
	})
	var repo GitRepository
	if err := c.GetJSON(ctx, u, &repo); err != nil {
		return GitRepository{}, fmt.Errorf("fetching default repository for project: %w", err)
	}
	return repo, nil
}

// GetDefaultRepositoryForProject returns the default git repository for a project by name.
func (c *Client) GetDefaultRepositoryForProject(ctx context.Context, projectName string) (GitRepository, error) {
	return c.GetDefaultRepository(ctx, c.resolveURL("/"+projectName))
}

// SetMergeProposalStatus changes the status of a merge proposal.
func (c *Client) SetMergeProposalStatus(ctx context.Context, mpSelfLink, status string) error {
	form := url.Values{
		"ws.op":  {"setStatus"},
		"status": {status},
	}
	_, err := c.Post(ctx, mpSelfLink, form)
	if err != nil {
		return fmt.Errorf("setting merge proposal status: %w", err)
	}
	return nil
}
