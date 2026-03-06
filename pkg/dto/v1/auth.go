// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import "time"

// AuthStatus describes the current authentication state across supported providers.
type AuthStatus struct {
	Launchpad LaunchpadAuthStatus `json:"launchpad"`
}

// SchemaName disambiguates AuthStatus in OpenAPI generation.
func (AuthStatus) SchemaName() string { return "AuthStatus" }

// LaunchpadAuthStatus describes the current Launchpad authentication state.
type LaunchpadAuthStatus struct {
	Authenticated   bool   `json:"authenticated"`
	Username        string `json:"username,omitempty"`
	DisplayName     string `json:"display_name,omitempty"`
	Source          string `json:"source,omitempty"`
	CredentialsPath string `json:"credentials_path,omitempty"`
	Error           string `json:"error,omitempty"`
}

// SchemaName disambiguates LaunchpadAuthStatus in OpenAPI generation.
func (LaunchpadAuthStatus) SchemaName() string { return "LaunchpadAuthStatus" }

// LaunchpadAuthBeginResult contains the public, non-secret state needed to continue auth.
type LaunchpadAuthBeginResult struct {
	FlowID       string    `json:"flow_id"`
	AuthorizeURL string    `json:"authorize_url"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SchemaName disambiguates LaunchpadAuthBeginResult in OpenAPI generation.
func (LaunchpadAuthBeginResult) SchemaName() string { return "LaunchpadAuthBeginResult" }

// LaunchpadAuthFinalizeResult contains the resulting Launchpad auth status after finalize.
type LaunchpadAuthFinalizeResult struct {
	Launchpad LaunchpadAuthStatus `json:"launchpad"`
}

// SchemaName disambiguates LaunchpadAuthFinalizeResult in OpenAPI generation.
func (LaunchpadAuthFinalizeResult) SchemaName() string { return "LaunchpadAuthFinalizeResult" }

// LaunchpadAuthLogoutResult describes the outcome of a logout operation.
type LaunchpadAuthLogoutResult struct {
	Cleared         bool   `json:"cleared"`
	CredentialsPath string `json:"credentials_path,omitempty"`
}

// SchemaName disambiguates LaunchpadAuthLogoutResult in OpenAPI generation.
func (LaunchpadAuthLogoutResult) SchemaName() string { return "LaunchpadAuthLogoutResult" }
