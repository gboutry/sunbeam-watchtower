// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package ubuntusso implements macaroon discharge via httpbakery for
// Ubuntu SSO and Candid identity providers.
package ubuntusso

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery"
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/httpbakery"
	"gopkg.in/macaroon.v2"
)

// DischargeAll takes a serialized root macaroon (JSON or base64), discharges
// all its third-party caveats using httpbakery with browser-based interaction,
// and returns the serialized credential (base64 root + " " + base64 bound-discharge).
//
// openURL is called with the URL the user must visit in a browser. It must not
// block — the httpbakery client polls the wait URL internally.
func DischargeAll(ctx context.Context, serializedMacaroon string, openURL func(u *url.URL) error) (string, error) {
	m, err := decodeMacaroonAny(serializedMacaroon)
	if err != nil {
		return "", fmt.Errorf("decoding root macaroon: %w", err)
	}

	// Wrap the raw macaroon.v2 macaroon into a bakery.Macaroon so
	// httpbakery can discharge it.
	bm, err := bakery.NewLegacyMacaroon(m)
	if err != nil {
		return "", fmt.Errorf("wrapping macaroon for bakery: %w", err)
	}

	client := httpbakery.NewClient()
	if openURL != nil {
		client.AddInteractor(httpbakery.WebBrowserInteractor{
			OpenWebBrowser: openURL,
		})
	}

	discharged, err := client.DischargeAll(ctx, bm)
	if err != nil {
		return "", fmt.Errorf("discharging macaroon: %w", err)
	}

	// discharged is a macaroon.Slice: [root, discharge1, discharge2, ...]
	// Serialize as: base64(root) + " " + base64(d1) + " " + base64(d2) ...
	return serializeMacaroonSlice(discharged)
}

// serializeMacaroonSlice serializes a macaroon slice into a space-separated
// string of base64-encoded macaroons. This is the format expected by store
// APIs in the Authorization header.
func serializeMacaroonSlice(ms macaroon.Slice) (string, error) {
	if len(ms) == 0 {
		return "", fmt.Errorf("empty macaroon slice")
	}
	result := ""
	for i, m := range ms {
		raw, err := m.MarshalBinary()
		if err != nil {
			return "", fmt.Errorf("marshaling macaroon %d: %w", i, err)
		}
		if i > 0 {
			result += " "
		}
		result += base64.RawURLEncoding.EncodeToString(raw)
	}
	return result, nil
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

	// Try JSON even if it doesn't start with { or ".
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
