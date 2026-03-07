// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	authsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/auth"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// AuthStatusOutput is the response for the auth status endpoint.
type AuthStatusOutput struct {
	Body dto.AuthStatus
}

// AuthLaunchpadBeginOutput is the response for beginning a Launchpad auth flow.
type AuthLaunchpadBeginOutput struct {
	Body dto.LaunchpadAuthBeginResult
}

// AuthLaunchpadFinalizeInput is the request body for finalizing a Launchpad auth flow.
type AuthLaunchpadFinalizeInput struct {
	Body struct {
		FlowID string `json:"flow_id" doc:"Opaque flow ID returned by /api/v1/auth/launchpad/begin"`
	}
}

// AuthLaunchpadFinalizeOutput is the response for completing a Launchpad auth flow.
type AuthLaunchpadFinalizeOutput struct {
	Body dto.LaunchpadAuthFinalizeResult
}

// AuthLaunchpadLogoutOutput is the response for logging out from Launchpad.
type AuthLaunchpadLogoutOutput struct {
	Body dto.LaunchpadAuthLogoutResult
}

// RegisterAuthAPI registers authentication endpoints on the given huma API.
func RegisterAuthAPI(api huma.API, application *app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/status",
		Summary:     "Show authentication status",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, _ *struct{}) (*AuthStatusOutput, error) {
		authWorkflow := frontend.NewAuthWorkflow(application)
		status, err := authWorkflow.Status(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to read auth status: %v", err))
		}

		out := &AuthStatusOutput{}
		out.Body = *status
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-launchpad-begin",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/launchpad/begin",
		Summary:     "Begin Launchpad authentication",
		Description: "Starts a Launchpad OAuth flow and returns an authorization URL plus an opaque flow ID.",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, _ *struct{}) (*AuthLaunchpadBeginOutput, error) {
		authWorkflow := frontend.NewAuthWorkflow(application)
		result, err := authWorkflow.BeginLaunchpad(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to begin Launchpad auth: %v", err))
		}

		out := &AuthLaunchpadBeginOutput{}
		out.Body = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-launchpad-finalize",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/launchpad/finalize",
		Summary:     "Finalize Launchpad authentication",
		Description: "Completes a Launchpad OAuth flow using the opaque flow ID returned by begin.",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, input *AuthLaunchpadFinalizeInput) (*AuthLaunchpadFinalizeOutput, error) {
		authWorkflow := frontend.NewAuthWorkflow(application)
		result, err := authWorkflow.FinalizeLaunchpad(ctx, input.Body.FlowID)
		if err != nil {
			switch {
			case errors.Is(err, authsvc.ErrLaunchpadAuthFlowNotFound):
				return nil, huma.Error404NotFound(err.Error())
			case errors.Is(err, authsvc.ErrLaunchpadAuthFlowExpired):
				return nil, huma.Error400BadRequest(err.Error())
			default:
				return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to finalize Launchpad auth: %v", err))
			}
		}

		out := &AuthLaunchpadFinalizeOutput{}
		out.Body = *result
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-launchpad-logout",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/launchpad/logout",
		Summary:     "Logout from Launchpad",
		Description: "Clears persisted Launchpad credentials when they are file-backed.",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, _ *struct{}) (*AuthLaunchpadLogoutOutput, error) {
		authWorkflow := frontend.NewAuthWorkflow(application)
		result, err := authWorkflow.LogoutLaunchpad(ctx)
		if err != nil {
			if errors.Is(err, authsvc.ErrLaunchpadEnvironmentCredentials) {
				return nil, huma.Error400BadRequest(err.Error())
			}
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to logout from Launchpad: %v", err))
		}

		out := &AuthLaunchpadLogoutOutput{}
		out.Body = *result
		return out, nil
	})
}
