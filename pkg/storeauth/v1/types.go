// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"errors"
	"time"
)

// PendingAuthFlow stores the server-side state for a pending Ubuntu SSO discharge flow.
type PendingAuthFlow struct {
	ID           string
	RootMacaroon string // base64-encoded serialized root macaroon
	CaveatID     string // third-party caveat ID from login.ubuntu.com
	DischargeID  string // SSO discharge token ID for polling
	VisitURL     string // URL user must visit in browser
	WaitURL      string // URL to poll for discharge completion
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

var (
	// ErrPendingAuthFlowNotFound indicates the requested auth flow does not exist.
	ErrPendingAuthFlowNotFound = errors.New("store auth flow not found")
	// ErrPendingAuthFlowExpired indicates the requested auth flow has expired.
	ErrPendingAuthFlowExpired = errors.New("store auth flow expired")
	// ErrDischargePending indicates the SSO discharge has not yet completed.
	ErrDischargePending = errors.New("store auth discharge pending")
	// ErrDischargeExpired indicates the SSO discharge flow expired.
	ErrDischargeExpired = errors.New("store auth discharge expired")
	// ErrDischargeDenied indicates the user denied the SSO login.
	ErrDischargeDenied = errors.New("store auth discharge denied")
)
