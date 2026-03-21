// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	gh "github.com/gboutry/sunbeam-watchtower/pkg/github/v1"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
	sa "github.com/gboutry/sunbeam-watchtower/pkg/storeauth/v1"
)

// LaunchpadCredentialStore manages persisted Launchpad credentials.
type LaunchpadCredentialStore interface {
	Load(ctx context.Context) (*lp.CredentialRecord, error)
	Save(ctx context.Context, creds *lp.Credentials) (*lp.CredentialRecord, error)
	Clear(ctx context.Context) error
}

// LaunchpadAuthenticator performs Launchpad OAuth and identity lookups.
type LaunchpadAuthenticator interface {
	ConsumerKey() string
	ObtainRequestToken(ctx context.Context) (*lp.RequestToken, error)
	ExchangeAccessToken(ctx context.Context, token *lp.RequestToken) (*lp.Credentials, error)
	CurrentUser(ctx context.Context, creds *lp.Credentials) (lp.Person, error)
}

// LaunchpadPendingAuthFlowStore stores short-lived pending auth flows server-side.
type LaunchpadPendingAuthFlowStore interface {
	Put(ctx context.Context, flow *lp.PendingAuthFlow) error
	Get(ctx context.Context, id string) (*lp.PendingAuthFlow, error)
	Delete(ctx context.Context, id string) error
}

// GitHubCredentialStore manages persisted GitHub credentials.
type GitHubCredentialStore interface {
	Load(ctx context.Context) (*gh.CredentialRecord, error)
	Save(ctx context.Context, creds *gh.Credentials) (*gh.CredentialRecord, error)
	Clear(ctx context.Context) error
}

// GitHubAuthenticator performs GitHub device-flow auth and identity lookups.
type GitHubAuthenticator interface {
	ClientID() string
	BeginDeviceFlow(ctx context.Context) (*gh.PendingAuthFlow, error)
	PollAccessToken(ctx context.Context, flow *gh.PendingAuthFlow) (*gh.Credentials, error)
	CurrentUser(ctx context.Context, creds *gh.Credentials) (gh.User, error)
}

// GitHubPendingAuthFlowStore stores short-lived pending GitHub auth flows server-side.
type GitHubPendingAuthFlowStore interface {
	Put(ctx context.Context, flow *gh.PendingAuthFlow) error
	Get(ctx context.Context, id string) (*gh.PendingAuthFlow, error)
	Delete(ctx context.Context, id string) error
}

// SnapStoreCredentialStore manages persisted Snap Store credentials.
type SnapStoreCredentialStore interface {
	Load(ctx context.Context) (*dto.StoreCredentialRecord, error)
	Save(ctx context.Context, macaroon string) (*dto.StoreCredentialRecord, error)
	Clear(ctx context.Context) error
}

// CharmhubCredentialStore manages persisted Charmhub credentials.
type CharmhubCredentialStore interface {
	Load(ctx context.Context) (*dto.StoreCredentialRecord, error)
	Save(ctx context.Context, macaroon string) (*dto.StoreCredentialRecord, error)
	Clear(ctx context.Context) error
}

// SnapStoreAuthenticator performs Snap Store macaroon discharge via Ubuntu SSO.
type SnapStoreAuthenticator interface {
	// BeginAuth requests a root macaroon from the store, extracts the
	// third-party caveat, and initiates an SSO discharge flow.
	// Returns a pending flow with a URL the user must visit in a browser.
	BeginAuth(ctx context.Context) (*sa.PendingAuthFlow, error)

	// PollAuth polls the SSO wait URL until the user has authenticated.
	// Returns the serialized credential (bound macaroon) ready for store API use.
	PollAuth(ctx context.Context, flow *sa.PendingAuthFlow) (string, error)
}

// CharmhubAuthenticator performs Charmhub macaroon discharge via Ubuntu SSO.
type CharmhubAuthenticator interface {
	// BeginAuth requests a root macaroon from Charmhub, extracts the
	// third-party caveat, and initiates an SSO discharge flow.
	BeginAuth(ctx context.Context) (*sa.PendingAuthFlow, error)

	// PollAuth polls the SSO wait URL until the user has authenticated.
	PollAuth(ctx context.Context, flow *sa.PendingAuthFlow) (string, error)
}

// SnapStorePendingAuthFlowStore stores short-lived pending Snap Store auth flows.
type SnapStorePendingAuthFlowStore interface {
	Put(ctx context.Context, flow *sa.PendingAuthFlow) error
	Get(ctx context.Context, id string) (*sa.PendingAuthFlow, error)
	Delete(ctx context.Context, id string) error
}

// CharmhubPendingAuthFlowStore stores short-lived pending Charmhub auth flows.
type CharmhubPendingAuthFlowStore interface {
	Put(ctx context.Context, flow *sa.PendingAuthFlow) error
	Get(ctx context.Context, id string) (*sa.PendingAuthFlow, error)
	Delete(ctx context.Context, id string) error
}
