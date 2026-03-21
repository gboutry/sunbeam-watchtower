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
	"unicode/utf8"

	"gopkg.in/macaroon.v2"

	sa "github.com/gboutry/sunbeam-watchtower/pkg/storeauth/v1"
)

const (
	// DefaultSSOBaseURL is the Ubuntu SSO API base URL.
	DefaultSSOBaseURL = "https://login.ubuntu.com"
	// DischargeEndpoint is the path for starting a macaroon discharge.
	DischargeEndpoint = "/api/v2/tokens/discharge"
)

// dischargeRequest is the JSON body for SSO/Candid discharge endpoints.
// Ubuntu SSO uses "caveat_id", Candid uses "id" — we send both.
type dischargeRequest struct {
	ID       string `json:"id"`
	CaveatID string `json:"caveat_id"`
}

// dischargeWaitResponse is the response from the wait URL.
type dischargeWaitResponse struct {
	DischargeMacaroon string `json:"discharge_macaroon"`
}

// ExtractSSOCaveatID decodes a serialized macaroon and returns
// the ID of the third-party caveat issued by login.ubuntu.com.
// It supports multiple serialization formats: JSON, base64 binary (standard
// and URL-safe, with and without padding).
func ExtractSSOCaveatID(serializedMacaroon string, ssoBaseURL string) (string, error) {
	m, err := decodeMacaroonAny(serializedMacaroon)
	if err != nil {
		return "", fmt.Errorf("decoding macaroon: %w", err)
	}

	var locations []string
	for _, caveat := range m.Caveats() {
		loc := caveat.Location
		if loc != "" {
			locations = append(locations, loc)
		}
		if loc != "" && strings.Contains(loc, "login.ubuntu.com") {
			return encodeCaveatID(caveat.Id), nil
		}
		if ssoBaseURL != "" && loc != "" && strings.Contains(loc, ssoBaseURL) {
			return encodeCaveatID(caveat.Id), nil
		}
	}

	return "", fmt.Errorf("no third-party caveat from login.ubuntu.com found in macaroon (caveat locations: %v, total caveats: %d)", locations, len(m.Caveats()))
}

// decodeMacaroonAny tries multiple deserialization strategies for a macaroon.
func decodeMacaroonAny(s string) (*macaroon.Macaroon, error) {
	var m macaroon.Macaroon

	// Try JSON first (Charmhub returns JSON-serialized macaroons).
	if len(s) > 0 && (s[0] == '{' || s[0] == '"') {
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			return &m, nil
		}
	}

	// Try JSON even if it doesn't start with { (some APIs wrap in quotes).
	if err := json.Unmarshal([]byte(s), &m); err == nil {
		return &m, nil
	}

	// Try various base64 encodings → binary unmarshal.
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.StdEncoding,
		base64.RawStdEncoding,
	} {
		if raw, err := enc.DecodeString(s); err == nil {
			if err := m.UnmarshalBinary(raw); err == nil {
				return &m, nil
			}
		}
	}

	return nil, fmt.Errorf("could not decode macaroon from any supported format (JSON, base64 binary)")
}

// BeginDischarge initiates a discharge flow with an identity provider.
// It POSTs the caveat_id to the discharge endpoint and expects interaction
// info with visit/wait URLs. The dischargeURL should be the full URL
// (e.g. "https://login.ubuntu.com/api/v2/tokens/discharge" or
// "https://api.jujucharms.com/identity/discharge").
func BeginDischarge(ctx context.Context, httpClient *http.Client, dischargeURL, caveatID string) (visitURL, waitURL string, err error) {
	body, err := json.Marshal(dischargeRequest{ID: caveatID, CaveatID: caveatID})
	if err != nil {
		return "", "", fmt.Errorf("marshaling discharge request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dischargeURL, strings.NewReader(string(body)))
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
func BindDischarge(rootSerialized, dischargeSerialized string) (string, error) {
	root, err := decodeMacaroonAny(rootSerialized)
	if err != nil {
		return "", fmt.Errorf("decoding root macaroon: %w", err)
	}

	discharge, err := decodeMacaroonAny(dischargeSerialized)
	if err != nil {
		return "", fmt.Errorf("decoding discharge macaroon: %w", err)
	}

	discharge.Bind(root.Signature())

	rootBin, err := root.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshaling root macaroon: %w", err)
	}

	dischargeBin, err := discharge.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshaling bound discharge macaroon: %w", err)
	}

	rootB64 := base64.RawURLEncoding.EncodeToString(rootBin)
	dischargeB64 := base64.RawURLEncoding.EncodeToString(dischargeBin)

	return rootB64 + " " + dischargeB64, nil
}

// encodeCaveatID returns the caveat ID as a string suitable for the discharge
// request. If the raw bytes are valid UTF-8 text (e.g. already base64-encoded
// by the macaroon issuer), they are returned as-is. Otherwise the bytes are
// base64-encoded.
func encodeCaveatID(id []byte) string {
	s := string(id)
	if utf8.ValidString(s) && len(s) > 0 {
		return s
	}
	return base64.StdEncoding.EncodeToString(id)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
