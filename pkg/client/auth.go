// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// AuthStatus returns the current authentication state.
func (c *Client) AuthStatus(ctx context.Context) (*dto.AuthStatus, error) {
	var result dto.AuthStatus
	err := c.get(ctx, "/api/v1/auth/status", nil, &result)
	return &result, err
}

// AuthLaunchpadBegin starts a Launchpad auth flow.
func (c *Client) AuthLaunchpadBegin(ctx context.Context) (*dto.LaunchpadAuthBeginResult, error) {
	var result dto.LaunchpadAuthBeginResult
	err := c.post(ctx, "/api/v1/auth/launchpad/begin", nil, &result)
	return &result, err
}

// AuthLaunchpadFinalize completes a pending Launchpad auth flow.
func (c *Client) AuthLaunchpadFinalize(
	ctx context.Context,
	flowID string,
) (*dto.LaunchpadAuthFinalizeResult, error) {
	var result dto.LaunchpadAuthFinalizeResult
	err := c.post(ctx, "/api/v1/auth/launchpad/finalize", map[string]string{"flow_id": flowID}, &result)
	return &result, err
}

// AuthLaunchpadLogout clears persisted Launchpad credentials.
func (c *Client) AuthLaunchpadLogout(ctx context.Context) (*dto.LaunchpadAuthLogoutResult, error) {
	var result dto.LaunchpadAuthLogoutResult
	err := c.post(ctx, "/api/v1/auth/launchpad/logout", nil, &result)
	return &result, err
}

// AuthGitHubBegin starts a GitHub auth flow.
func (c *Client) AuthGitHubBegin(ctx context.Context) (*dto.GitHubAuthBeginResult, error) {
	var result dto.GitHubAuthBeginResult
	err := c.post(ctx, "/api/v1/auth/github/begin", nil, &result)
	return &result, err
}

// AuthGitHubFinalize completes a pending GitHub auth flow.
func (c *Client) AuthGitHubFinalize(
	ctx context.Context,
	flowID string,
) (*dto.GitHubAuthFinalizeResult, error) {
	var result dto.GitHubAuthFinalizeResult
	err := c.post(ctx, "/api/v1/auth/github/finalize", map[string]string{"flow_id": flowID}, &result)
	return &result, err
}

// AuthGitHubLogout clears persisted GitHub credentials.
func (c *Client) AuthGitHubLogout(ctx context.Context) (*dto.GitHubAuthLogoutResult, error) {
	var result dto.GitHubAuthLogoutResult
	err := c.post(ctx, "/api/v1/auth/github/logout", nil, &result)
	return &result, err
}

// AuthSnapStoreLogin saves a pre-obtained Snap Store macaroon.
func (c *Client) AuthSnapStoreLogin(ctx context.Context, macaroon string) (*dto.SnapStoreAuthLoginResult, error) {
	var result dto.SnapStoreAuthLoginResult
	err := c.post(ctx, "/api/v1/auth/snapstore/login", map[string]string{"macaroon": macaroon}, &result)
	return &result, err
}

// AuthSnapStoreLogout clears persisted Snap Store credentials.
func (c *Client) AuthSnapStoreLogout(ctx context.Context) (*dto.SnapStoreAuthLogoutResult, error) {
	var result dto.SnapStoreAuthLogoutResult
	err := c.post(ctx, "/api/v1/auth/snapstore/logout", nil, &result)
	return &result, err
}

// AuthCharmhubLogin saves a pre-obtained Charmhub macaroon.
func (c *Client) AuthCharmhubLogin(ctx context.Context, macaroon string) (*dto.CharmhubAuthLoginResult, error) {
	var result dto.CharmhubAuthLoginResult
	err := c.post(ctx, "/api/v1/auth/charmhub/login", map[string]string{"macaroon": macaroon}, &result)
	return &result, err
}

// AuthCharmhubLogout clears persisted Charmhub credentials.
func (c *Client) AuthCharmhubLogout(ctx context.Context) (*dto.CharmhubAuthLogoutResult, error) {
	var result dto.CharmhubAuthLogoutResult
	err := c.post(ctx, "/api/v1/auth/charmhub/logout", nil, &result)
	return &result, err
}
