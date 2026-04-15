// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import "time"

// AuthStatus describes the current authentication state across supported providers.
type AuthStatus struct {
	Launchpad LaunchpadAuthStatus `json:"launchpad"`
	GitHub    GitHubAuthStatus    `json:"github"`
	SnapStore SnapStoreAuthStatus `json:"snap_store" yaml:"snap_store"`
	Charmhub  CharmhubAuthStatus  `json:"charmhub" yaml:"charmhub"`
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

// GitHubAuthStatus describes the current GitHub authentication state.
type GitHubAuthStatus struct {
	Authenticated   bool   `json:"authenticated"`
	Username        string `json:"username,omitempty"`
	DisplayName     string `json:"display_name,omitempty"`
	Source          string `json:"source,omitempty"`
	CredentialsPath string `json:"credentials_path,omitempty"`
	Error           string `json:"error,omitempty"`
}

// SchemaName disambiguates GitHubAuthStatus in OpenAPI generation.
func (GitHubAuthStatus) SchemaName() string { return "GitHubAuthStatus" }

// GitHubAuthBeginResult contains the public state needed to continue the device flow.
type GitHubAuthBeginResult struct {
	FlowID          string    `json:"flow_id"`
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	IntervalSeconds int       `json:"interval_seconds"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// SchemaName disambiguates GitHubAuthBeginResult in OpenAPI generation.
func (GitHubAuthBeginResult) SchemaName() string { return "GitHubAuthBeginResult" }

// GitHubAuthFinalizeResult contains the resulting GitHub auth status after finalize.
type GitHubAuthFinalizeResult struct {
	GitHub GitHubAuthStatus `json:"github"`
}

// SchemaName disambiguates GitHubAuthFinalizeResult in OpenAPI generation.
func (GitHubAuthFinalizeResult) SchemaName() string { return "GitHubAuthFinalizeResult" }

// GitHubAuthLogoutResult describes the outcome of a GitHub logout operation.
type GitHubAuthLogoutResult struct {
	Cleared         bool   `json:"cleared"`
	CredentialsPath string `json:"credentials_path,omitempty"`
}

// SchemaName disambiguates GitHubAuthLogoutResult in OpenAPI generation.
func (GitHubAuthLogoutResult) SchemaName() string { return "GitHubAuthLogoutResult" }

// SnapStoreAuthStatus describes the current Snap Store authentication state.
type SnapStoreAuthStatus struct {
	Authenticated   bool   `json:"authenticated" yaml:"authenticated"`
	Source          string `json:"source,omitempty" yaml:"source,omitempty"`
	CredentialsPath string `json:"credentials_path,omitempty" yaml:"credentials_path,omitempty"`
}

// SchemaName disambiguates SnapStoreAuthStatus in OpenAPI generation.
func (SnapStoreAuthStatus) SchemaName() string { return "SnapStoreAuthStatus" }

// SnapStoreAuthBeginResult contains the root macaroon for client-side discharge.
type SnapStoreAuthBeginResult struct {
	RootMacaroon string `json:"root_macaroon"`
}

// SchemaName disambiguates SnapStoreAuthBeginResult in OpenAPI generation.
func (SnapStoreAuthBeginResult) SchemaName() string { return "SnapStoreAuthBeginResult" }

// SnapStoreAuthSaveResult contains the resulting Snap Store auth status after saving a credential.
type SnapStoreAuthSaveResult struct {
	SnapStore SnapStoreAuthStatus `json:"snap_store"`
}

// SchemaName disambiguates SnapStoreAuthSaveResult in OpenAPI generation.
func (SnapStoreAuthSaveResult) SchemaName() string { return "SnapStoreAuthSaveResult" }

// SnapStoreAuthLogoutResult describes the outcome of a Snap Store logout operation.
type SnapStoreAuthLogoutResult struct {
	Cleared         bool   `json:"cleared"`
	CredentialsPath string `json:"credentials_path,omitempty"`
}

// SchemaName disambiguates SnapStoreAuthLogoutResult in OpenAPI generation.
func (SnapStoreAuthLogoutResult) SchemaName() string { return "SnapStoreAuthLogoutResult" }

// CharmhubAuthStatus describes the current Charmhub authentication state.
type CharmhubAuthStatus struct {
	Authenticated   bool   `json:"authenticated" yaml:"authenticated"`
	Source          string `json:"source,omitempty" yaml:"source,omitempty"`
	CredentialsPath string `json:"credentials_path,omitempty" yaml:"credentials_path,omitempty"`
}

// SchemaName disambiguates CharmhubAuthStatus in OpenAPI generation.
func (CharmhubAuthStatus) SchemaName() string { return "CharmhubAuthStatus" }

// CharmhubAuthBeginResult contains the root macaroon for client-side discharge.
type CharmhubAuthBeginResult struct {
	RootMacaroon string `json:"root_macaroon"`
}

// SchemaName disambiguates CharmhubAuthBeginResult in OpenAPI generation.
func (CharmhubAuthBeginResult) SchemaName() string { return "CharmhubAuthBeginResult" }

// CharmhubAuthSaveResult contains the resulting Charmhub auth status after saving a credential.
type CharmhubAuthSaveResult struct {
	Charmhub CharmhubAuthStatus `json:"charmhub"`
}

// SchemaName disambiguates CharmhubAuthSaveResult in OpenAPI generation.
func (CharmhubAuthSaveResult) SchemaName() string { return "CharmhubAuthSaveResult" }

// CharmhubAuthLogoutResult describes the outcome of a Charmhub logout operation.
type CharmhubAuthLogoutResult struct {
	Cleared         bool   `json:"cleared"`
	CredentialsPath string `json:"credentials_path,omitempty"`
}

// SchemaName disambiguates CharmhubAuthLogoutResult in OpenAPI generation.
func (CharmhubAuthLogoutResult) SchemaName() string { return "CharmhubAuthLogoutResult" }

// StoreCredentialRecord describes loaded or saved store credentials (Snap Store or Charmhub).
//
// For Charmhub, Macaroon is the short-lived exchanged publisher token that
// `/v1/charm/...` callers send on `Authorization: Macaroon <token>`, and
// DischargedBundle is the long-lived client-discharged bundle kept so the
// token can be re-exchanged without a fresh browser discharge when it
// expires. For Snap Store, only Macaroon is populated.
type StoreCredentialRecord struct {
	Macaroon         string
	DischargedBundle string
	Source           string
	Path             string
}
