// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package charmhub

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
	defaultTokensEndpoint = "https://api.charmhub.io/v1/tokens"
	defaultSSOBaseURL     = "https://api.jujucharms.com/identity"
	defaultFlowTTL        = 10 * time.Minute
	pollInterval          = 2 * time.Second
)

// tokensRequest is the JSON body for the Charmhub tokens endpoint.
type tokensRequest struct {
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// tokensResponse is the JSON body returned by the Charmhub tokens endpoint.
type tokensResponse struct {
	Macaroon string `json:"macaroon"`
}

// Authenticator performs Charmhub authentication via Ubuntu SSO macaroon discharge.
type Authenticator struct {
	tokensEndpoint string
	ssoBaseURL     string
	logger         *slog.Logger
	httpClient     *http.Client
}

// NewAuthenticator creates a Charmhub SSO authenticator adapter.
func NewAuthenticator(logger *slog.Logger, httpClient *http.Client) *Authenticator {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Authenticator{
		tokensEndpoint: defaultTokensEndpoint,
		ssoBaseURL:     defaultSSOBaseURL,
		logger:         logger,
		httpClient:     httpClient,
	}
}

// BeginAuth requests a root macaroon from Charmhub, extracts the Ubuntu SSO
// third-party caveat, and initiates the browser-based discharge flow.
func (a *Authenticator) BeginAuth(ctx context.Context) (*sa.PendingAuthFlow, error) {
	a.logger.Info("requesting root macaroon from charmhub")

	rootMacaroon, err := a.requestRootMacaroon(ctx)
	if err != nil {
		return nil, fmt.Errorf("requesting charmhub root macaroon: %w", err)
	}

	caveatID, err := ubuntusso.ExtractSSOCaveatID(rootMacaroon, a.ssoBaseURL)
	if err != nil {
		return nil, fmt.Errorf("extracting SSO caveat from charmhub macaroon: %w", err)
	}

	a.logger.Info("starting SSO discharge flow", "sso_base_url", a.ssoBaseURL)

	dischargeURL := strings.TrimRight(a.ssoBaseURL, "/") + "/discharge"
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
	a.logger.Info("polling SSO for charmhub discharge")

	dischargeMacaroon, err := ubuntusso.PollDischarge(ctx, a.httpClient, flow.WaitURL, pollInterval)
	if err != nil {
		return "", fmt.Errorf("polling charmhub SSO discharge: %w", err)
	}

	credential, err := ubuntusso.BindDischarge(flow.RootMacaroon, dischargeMacaroon)
	if err != nil {
		return "", fmt.Errorf("binding charmhub discharge: %w", err)
	}

	return credential, nil
}

func (a *Authenticator) requestRootMacaroon(ctx context.Context) (string, error) {
	body, err := json.Marshal(tokensRequest{
		Description: "sunbeam-watchtower",
		Permissions: []string{
			"account-view-packages",
			"package-manage-acl",
			"package-view",
			"package-view-acl",
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshaling tokens request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokensEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("creating tokens request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing tokens request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("charmhub tokens request failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var tokensResp tokensResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokensResp); err != nil {
		return "", fmt.Errorf("decoding tokens response: %w", err)
	}
	if tokensResp.Macaroon == "" {
		return "", fmt.Errorf("charmhub returned empty macaroon")
	}

	return tokensResp.Macaroon, nil
}
