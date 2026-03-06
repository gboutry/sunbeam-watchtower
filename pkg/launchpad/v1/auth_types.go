// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"errors"
	"time"
)

const (
	// CredentialSourceEnvironment means credentials were loaded from environment variables.
	CredentialSourceEnvironment = "environment"
	// CredentialSourceFile means credentials were loaded from the file-backed cache.
	CredentialSourceFile = "file"
)

// CredentialRecord describes loaded or saved Launchpad credentials.
type CredentialRecord struct {
	Credentials *Credentials
	Source      string
	Path        string
}

// PendingAuthFlow stores the server-side state for a pending Launchpad auth flow.
type PendingAuthFlow struct {
	ID                 string
	RequestToken       string
	RequestTokenSecret string
	CreatedAt          time.Time
	ExpiresAt          time.Time
}

var (
	// ErrPendingAuthFlowNotFound indicates the requested auth flow does not exist.
	ErrPendingAuthFlowNotFound = errors.New("launchpad auth flow not found")
	// ErrPendingAuthFlowExpired indicates the requested auth flow has expired.
	ErrPendingAuthFlowExpired = errors.New("launchpad auth flow expired")
)
