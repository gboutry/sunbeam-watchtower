// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	gh "github.com/gboutry/sunbeam-watchtower/pkg/github/v1"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
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
