// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package snapstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/ubuntusso"
	sa "github.com/gboutry/sunbeam-watchtower/pkg/storeauth/v1"
)

const (
	defaultACLEndpoint = "https://dashboard.snapcraft.io/dev/api/acl/"
	defaultSSOBaseURL  = "https://login.ubuntu.com"
	defaultFlowTTL     = 10 * time.Minute
	pollInterval       = 2 * time.Second
)

// aclRequest is the JSON body for the Snap Store ACL endpoint.
type aclRequest struct {
	Permissions []string `json:"permissions"`
}

// aclResponse is the JSON body returned by the Snap Store ACL endpoint.
type aclResponse struct {
	Macaroon string `json:"macaroon"`
}

// Authenticator performs Snap Store authentication via Ubuntu SSO macaroon discharge.
type Authenticator struct {
	aclEndpoint string
	ssoBaseURL  string
	permissions []string
	logger      *slog.Logger
	httpClient  *http.Client
}

// NewAuthenticator creates a Snap Store SSO authenticator adapter.
func NewAuthenticator(logger *slog.Logger, httpClient *http.Client) *Authenticator {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Authenticator{
		aclEndpoint: defaultACLEndpoint,
		ssoBaseURL:  defaultSSOBaseURL,
		permissions: []string{"package_access", "package_push", "package_register"},
		logger:      logger,
		httpClient:  httpClient,
	}
}

// BeginAuth requests a root macaroon from the Snap Store, extracts the Ubuntu SSO
// third-party caveat, and initiates the browser-based discharge flow.
func (a *Authenticator) BeginAuth(ctx context.Context) (*sa.PendingAuthFlow, error) {
	a.logger.Info("requesting root macaroon from snap store")

	rootMacaroon, err := a.requestRootMacaroon(ctx)
	if err != nil {
		return nil, fmt.Errorf("requesting snap store root macaroon: %w", err)
	}

	caveatID, err := ubuntusso.ExtractSSOCaveatID(rootMacaroon, a.ssoBaseURL)
	if err != nil {
		return nil, fmt.Errorf("extracting SSO caveat from snap store macaroon: %w", err)
	}

	a.logger.Info("starting SSO discharge flow", "sso_base_url", a.ssoBaseURL)

	dischargeURL := strings.TrimRight(a.ssoBaseURL, "/") + ubuntusso.DischargeEndpoint
	visitURL, waitURL, err := ubuntusso.BeginDischarge(ctx, a.httpClient, dischargeURL, caveatID)
	if err != nil {
		return nil, fmt.Errorf("starting SSO discharge: %w", err)
	}

	now := time.Now().UTC()
	return &sa.PendingAuthFlow{
		RootMacaroon: rootMacaroon,
		CaveatID:     caveatID,
		VisitURL:     visitURL,
		WaitURL:      waitURL,
		CreatedAt:    now,
		ExpiresAt:    now.Add(defaultFlowTTL),
	}, nil
}

// PollAuth polls the SSO wait URL until the user completes browser authentication,
// then binds the discharge macaroon to the root and returns the serialized credential.
func (a *Authenticator) PollAuth(ctx context.Context, flow *sa.PendingAuthFlow) (string, error) {
	a.logger.Info("polling SSO for snap store discharge")

	dischargeMacaroon, err := ubuntusso.PollDischarge(ctx, a.httpClient, flow.WaitURL, pollInterval)
	if err != nil {
		return "", fmt.Errorf("polling snap store SSO discharge: %w", err)
	}

	credential, err := ubuntusso.BindDischarge(flow.RootMacaroon, dischargeMacaroon)
	if err != nil {
		return "", fmt.Errorf("binding snap store discharge: %w", err)
	}

	return credential, nil
}

func (a *Authenticator) requestRootMacaroon(ctx context.Context) (string, error) {
	body, err := json.Marshal(aclRequest{Permissions: a.permissions})
	if err != nil {
		return "", fmt.Errorf("marshaling ACL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.aclEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("creating ACL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing ACL request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("snap store ACL request failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var aclResp aclResponse
	if err := json.NewDecoder(resp.Body).Decode(&aclResp); err != nil {
		return "", fmt.Errorf("decoding ACL response: %w", err)
	}
	if aclResp.Macaroon == "" {
		return "", fmt.Errorf("snap store returned empty macaroon")
	}

	return aclResp.Macaroon, nil
}
