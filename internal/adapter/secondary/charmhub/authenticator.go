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
	"net/url"
	"strings"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/ubuntusso"
	sa "github.com/gboutry/sunbeam-watchtower/pkg/storeauth/v1"
)

const (
	defaultTokensEndpoint = "https://api.charmhub.io/v1/tokens"
	defaultFlowTTL        = 10 * time.Minute
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

// Authenticator performs Charmhub authentication via httpbakery macaroon discharge.
type Authenticator struct {
	tokensEndpoint string
	logger         *slog.Logger
	httpClient     *http.Client
}

// NewAuthenticator creates a Charmhub authenticator adapter.
func NewAuthenticator(logger *slog.Logger, httpClient *http.Client) *Authenticator {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Authenticator{
		tokensEndpoint: defaultTokensEndpoint,
		logger:         logger,
		httpClient:     httpClient,
	}
}

// BeginAuth requests a root macaroon from Charmhub and returns a pending flow.
// The actual discharge (browser interaction) happens in PollAuth.
func (a *Authenticator) BeginAuth(ctx context.Context) (*sa.PendingAuthFlow, error) {
	a.logger.Info("requesting root macaroon from charmhub")

	rootMacaroon, err := a.requestRootMacaroon(ctx)
	if err != nil {
		return nil, fmt.Errorf("requesting charmhub root macaroon: %w", err)
	}

	now := time.Now().UTC()
	return &sa.PendingAuthFlow{
		RootMacaroon: rootMacaroon,
		CreatedAt:    now,
		ExpiresAt:    now.Add(defaultFlowTTL),
	}, nil
}

// PollAuth discharges the root macaroon using httpbakery with browser-based
// interaction. It opens a browser for the user to authenticate, then returns
// the serialized credential.
func (a *Authenticator) PollAuth(ctx context.Context, flow *sa.PendingAuthFlow) (string, error) {
	a.logger.Info("starting httpbakery discharge for charmhub")

	var visitURL string
	credential, err := ubuntusso.DischargeAll(ctx, flow.RootMacaroon, func(u *url.URL) error {
		visitURL = u.String()
		a.logger.Info("browser visit required", "url", visitURL)
		// The openURL callback from the caller will handle this.
		// For now, just record it. The caller wraps this with actual browser opening.
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("discharging charmhub macaroon: %w", err)
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
