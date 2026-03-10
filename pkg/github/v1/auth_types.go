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

// Credentials stores GitHub authentication state.
type Credentials struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

// CredentialRecord describes loaded or saved GitHub credentials.
type CredentialRecord struct {
	Credentials *Credentials
	Source      string
	Path        string
}

// User is the authenticated GitHub identity.
type User struct {
	Login string
	Name  string
}

// PendingAuthFlow stores the server-side state for a pending GitHub device flow.
type PendingAuthFlow struct {
	ID              string
	DeviceCode      string
	UserCode        string
	VerificationURI string
	IntervalSeconds int
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

var (
	// ErrPendingAuthFlowNotFound indicates the requested auth flow does not exist.
	ErrPendingAuthFlowNotFound = errors.New("github auth flow not found")
	// ErrPendingAuthFlowExpired indicates the requested auth flow has expired.
	ErrPendingAuthFlowExpired = errors.New("github auth flow expired")
	// ErrAuthorizationPending indicates the device flow has not yet been authorized.
	ErrAuthorizationPending = errors.New("github authorization pending")
	// ErrSlowDown indicates polling should slow down.
	ErrSlowDown = errors.New("github device flow slow down")
	// ErrAccessDenied indicates the user denied the device flow.
	ErrAccessDenied = errors.New("github device flow access denied")
	// ErrExpiredToken indicates the device flow expired before authorization completed.
	ErrExpiredToken = errors.New("github device flow expired token")
	// ErrIncorrectDeviceCode indicates the stored device code is invalid.
	ErrIncorrectDeviceCode = errors.New("github device flow incorrect device code")
)
