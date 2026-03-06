// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"fmt"
	"net/url"
)

// GetPerson fetches a person or team by username.
func (c *Client) GetPerson(ctx context.Context, name string) (Person, error) {
	var p Person
	if err := c.GetJSON(ctx, "/~"+name, &p); err != nil {
		return Person{}, fmt.Errorf("fetching person %q: %w", name, err)
	}
	return p, nil
}

// GetPPAs returns the PPAs owned by a person or team.
func (c *Client) GetPPAs(ctx context.Context, personName string) ([]Archive, error) {
	path := fmt.Sprintf("/~%s/ppas", personName)
	return GetAllPages[Archive](ctx, c, path)
}

// GetPPAByName returns a specific PPA by name for a person.
func (c *Client) GetPPAByName(ctx context.Context, personName, ppaName string) (Archive, error) {
	u := wsOpURL(c.resolveURL("/~"+personName), "getPPAByName", url.Values{
		"name": {ppaName},
	})
	var a Archive
	if err := c.GetJSON(ctx, u, &a); err != nil {
		return Archive{}, fmt.Errorf("fetching PPA %q for %q: %w", ppaName, personName, err)
	}
	return a, nil
}

// GetPersonMergeProposals returns merge proposals for a person.
// status is optional; valid values: "Work in progress", "Needs review", "Approved", "Rejected", "Merged", "Superseded".
func (c *Client) GetPersonMergeProposals(ctx context.Context, personName string, status ...string) ([]MergeProposal, error) {
	params := url.Values{}
	for _, s := range status {
		params.Add("status", s)
	}
	u := wsOpURL(c.resolveURL("/~"+personName), "getMergeProposals", params)
	return GetAllPages[MergeProposal](ctx, c, u)
}

// GetPersonReviewRequests returns merge proposals where the person was asked to review.
func (c *Client) GetPersonReviewRequests(ctx context.Context, personName string, status ...string) ([]MergeProposal, error) {
	params := url.Values{}
	for _, s := range status {
		params.Add("status", s)
	}
	u := wsOpURL(c.resolveURL("/~"+personName), "getRequestedReviews", params)
	return GetAllPages[MergeProposal](ctx, c, u)
}

// GetOwnedProjects returns projects owned by a person or their teams.
func (c *Client) GetOwnedProjects(ctx context.Context, personName string) ([]Project, error) {
	u := wsOpURL(c.resolveURL("/~"+personName), "getOwnedProjects", nil)
	return GetAllPages[Project](ctx, c, u)
}

// GetTeamMembers returns direct members of a team.
func (c *Client) GetTeamMembers(ctx context.Context, teamName string) ([]Person, error) {
	path := fmt.Sprintf("/~%s/members", teamName)
	return GetAllPages[Person](ctx, c, path)
}
