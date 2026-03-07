// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// LaunchpadAuthorizationHandler handles the user-facing authorization step between begin and finalize.
type LaunchpadAuthorizationHandler func(context.Context, *dto.LaunchpadAuthBeginResult) error

// AuthLoginResult contains both halves of a completed login workflow.
type AuthLoginResult struct {
	Begin     *dto.LaunchpadAuthBeginResult
	Finalized *dto.LaunchpadAuthFinalizeResult
}

// AuthClientWorkflow exposes reusable client-side auth workflows for CLI/TUI/MCP frontends.
type AuthClientWorkflow struct {
	client *client.Client
}

// NewAuthClientWorkflow creates a client-side auth workflow.
func NewAuthClientWorkflow(apiClient *client.Client) *AuthClientWorkflow {
	return &AuthClientWorkflow{client: apiClient}
}

// Status returns the current remote authentication state.
func (w *AuthClientWorkflow) Status(ctx context.Context) (*dto.AuthStatus, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.AuthStatus(ctx)
}

// BeginLaunchpad starts a remote Launchpad auth flow.
func (w *AuthClientWorkflow) BeginLaunchpad(ctx context.Context) (*dto.LaunchpadAuthBeginResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.AuthLaunchpadBegin(ctx)
}

// FinalizeLaunchpad completes one remote Launchpad auth flow.
func (w *AuthClientWorkflow) FinalizeLaunchpad(ctx context.Context, flowID string) (*dto.LaunchpadAuthFinalizeResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.AuthLaunchpadFinalize(ctx, flowID)
}

// LoginLaunchpad runs the full begin -> authorize -> finalize client workflow.
func (w *AuthClientWorkflow) LoginLaunchpad(
	ctx context.Context,
	handler LaunchpadAuthorizationHandler,
) (*AuthLoginResult, error) {
	if handler == nil {
		return nil, errors.New("launchpad authorization handler is required")
	}

	begin, err := w.BeginLaunchpad(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler(ctx, begin); err != nil {
		return nil, err
	}

	finalized, err := w.FinalizeLaunchpad(ctx, begin.FlowID)
	if err != nil {
		return nil, err
	}
	return &AuthLoginResult{
		Begin:     begin,
		Finalized: finalized,
	}, nil
}

// LogoutLaunchpad clears persisted Launchpad credentials through the API.
func (w *AuthClientWorkflow) LogoutLaunchpad(ctx context.Context) (*dto.LaunchpadAuthLogoutResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.AuthLaunchpadLogout(ctx)
}

func (w *AuthClientWorkflow) resolveClient() (*client.Client, error) {
	if w.client == nil {
		return nil, errors.New("auth client workflow requires an API client")
	}
	return w.client, nil
}
