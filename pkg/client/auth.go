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
