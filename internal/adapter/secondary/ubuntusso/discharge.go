// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package ubuntusso implements the Ubuntu SSO Candid-style macaroon discharge flow.
package ubuntusso

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gopkg.in/macaroon.v2"

	sa "github.com/gboutry/sunbeam-watchtower/pkg/storeauth/v1"
)

const (
	// DefaultSSOBaseURL is the Ubuntu SSO API base URL.
	DefaultSSOBaseURL = "https://login.ubuntu.com"
	// DischargeEndpoint is the path for starting a macaroon discharge.
	DischargeEndpoint = "/api/v2/tokens/discharge"
)

// dischargeRequest is the JSON body for the SSO discharge endpoint.
type dischargeRequest struct {
	Permissions []string `json:"permissions,omitempty"`
	CaveatID    string   `json:"caveat_id"`
}

// dischargeWaitResponse is the response from the wait URL.
type dischargeWaitResponse struct {
	DischargeMacaroon string `json:"discharge_macaroon"`
}

// ExtractSSOCaveatID decodes a base64-encoded serialized macaroon and returns
// the ID of the third-party caveat issued by login.ubuntu.com.
func ExtractSSOCaveatID(serializedMacaroon string, ssoBaseURL string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(serializedMacaroon)
	if err != nil {
		// Try standard base64.
		raw, err = base64.StdEncoding.DecodeString(serializedMacaroon)
		if err != nil {
			return "", fmt.Errorf("decoding macaroon: %w", err)
		}
	}

	var m macaroon.Macaroon
	if err := m.UnmarshalBinary(raw); err != nil {
		// Try JSON unmarshal as fallback (some stores return JSON-encoded macaroons).
		if jsonErr := json.Unmarshal([]byte(serializedMacaroon), &m); jsonErr != nil {
			return "", fmt.Errorf("unmarshaling macaroon (binary: %w, json: %w)", err, jsonErr)
		}
	}

	for _, caveat := range m.Caveats() {
		loc := caveat.Location
		if loc != "" && strings.Contains(loc, "login.ubuntu.com") {
			return string(caveat.Id), nil
		}
		if ssoBaseURL != "" && loc != "" && strings.Contains(loc, ssoBaseURL) {
			return string(caveat.Id), nil
		}
	}

	return "", fmt.Errorf("no third-party caveat from login.ubuntu.com found in macaroon")
}

// BeginDischarge initiates a discharge flow with Ubuntu SSO.
// It POSTs the caveat_id and expects a 401 response with interaction info,
// or alternatively parses visit/wait URLs from the response body.
func BeginDischarge(ctx context.Context, httpClient *http.Client, ssoBaseURL, caveatID string) (visitURL, waitURL string, err error) {
	if ssoBaseURL == "" {
		ssoBaseURL = DefaultSSOBaseURL
	}

	body, err := json.Marshal(dischargeRequest{CaveatID: caveatID})
	if err != nil {
		return "", "", fmt.Errorf("marshaling discharge request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ssoBaseURL+DischargeEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", "", fmt.Errorf("creating discharge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("executing discharge request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", "", fmt.Errorf("reading discharge response: %w", err)
	}

	// The SSO endpoint returns interaction-required info.
	// The response structure contains visit/wait URL pointers.
	var interactionInfo struct {
		Kind        string `json:"kind"`
		Message     string `json:"message"`
		Code        string `json:"code"`
		VisitURL    string `json:"visit_url"`
		WaitURL     string `json:"wait_url"`
		Interaction struct {
			VisitURL string `json:"visit_url"`
			WaitURL  string `json:"wait_url"`
		} `json:"interaction"`
		// Candid-style Info object.
		Info struct {
			VisitURL string `json:"visit_url"`
			WaitURL  string `json:"wait_url"`
		} `json:"Info"`
	}
	if err := json.Unmarshal(respBody, &interactionInfo); err != nil {
		return "", "", fmt.Errorf("parsing discharge response (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Try each nesting level for visit/wait URLs.
	visitURL = firstNonEmpty(interactionInfo.VisitURL, interactionInfo.Interaction.VisitURL, interactionInfo.Info.VisitURL)
	waitURL = firstNonEmpty(interactionInfo.WaitURL, interactionInfo.Interaction.WaitURL, interactionInfo.Info.WaitURL)

	if visitURL == "" || waitURL == "" {
		return "", "", fmt.Errorf("SSO discharge response missing visit/wait URLs (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return visitURL, waitURL, nil
}

// PollDischarge polls the wait URL until the discharge macaroon is available.
func PollDischarge(ctx context.Context, httpClient *http.Client, waitURL string, interval time.Duration) (string, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	firstAttempt := true
	for {
		if !firstAttempt {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
		firstAttempt = false

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, waitURL, nil)
		if err != nil {
			return "", fmt.Errorf("creating wait request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("polling wait URL: %w", err)
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("reading wait response: %w", readErr)
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			var waitResp dischargeWaitResponse
			if err := json.Unmarshal(body, &waitResp); err != nil {
				return "", fmt.Errorf("parsing wait response: %w", err)
			}
			if waitResp.DischargeMacaroon != "" {
				return waitResp.DischargeMacaroon, nil
			}
			// Successful response but empty discharge - treat as pending.
			continue
		case resp.StatusCode == http.StatusAccepted:
			// 202 means the discharge is still pending.
			continue
		case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
			return "", sa.ErrDischargeDenied
		case resp.StatusCode == http.StatusGone:
			return "", sa.ErrDischargeExpired
		case resp.StatusCode >= 400:
			return "", fmt.Errorf("SSO wait request failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
		default:
			continue
		}
	}
}

// BindDischarge binds a discharge macaroon to a root macaroon and returns
// the serialized credential string (base64-encoded root + " " + base64-encoded discharge).
func BindDischarge(rootMacaroonB64, dischargeMacaroonB64 string) (string, error) {
	rootRaw, err := decodeMacaroon(rootMacaroonB64)
	if err != nil {
		return "", fmt.Errorf("decoding root macaroon: %w", err)
	}

	dischargeRaw, err := decodeMacaroon(dischargeMacaroonB64)
	if err != nil {
		return "", fmt.Errorf("decoding discharge macaroon: %w", err)
	}

	var root macaroon.Macaroon
	if err := root.UnmarshalBinary(rootRaw); err != nil {
		if jsonErr := json.Unmarshal([]byte(rootMacaroonB64), &root); jsonErr != nil {
			return "", fmt.Errorf("unmarshaling root macaroon: %w", err)
		}
	}

	var discharge macaroon.Macaroon
	if err := discharge.UnmarshalBinary(dischargeRaw); err != nil {
		if jsonErr := json.Unmarshal([]byte(dischargeMacaroonB64), &discharge); jsonErr != nil {
			return "", fmt.Errorf("unmarshaling discharge macaroon: %w", err)
		}
	}

	discharge.Bind(root.Signature())

	boundDischargeRaw, err := discharge.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshaling bound discharge macaroon: %w", err)
	}

	rootB64 := base64.RawURLEncoding.EncodeToString(rootRaw)
	dischargeB64 := base64.RawURLEncoding.EncodeToString(boundDischargeRaw)

	return rootB64 + " " + dischargeB64, nil
}

func decodeMacaroon(s string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
	}
	return raw, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
