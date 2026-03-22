// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"gopkg.in/macaroon.v2"
)

func TestDecodeMacaroonAny_JSON(t *testing.T) {
	// Create a macaroon and marshal to JSON.
	m, err := macaroon.New([]byte("root-key"), []byte("id"), "location", macaroon.LatestVersion)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	decoded, err := DecodeMacaroonAny(string(data))
	if err != nil {
		t.Fatalf("DecodeMacaroonAny() error = %v", err)
	}
	if string(decoded.Id()) != "id" {
		t.Fatalf("Id = %q, want id", decoded.Id())
	}
}

func TestDecodeMacaroonAny_Base64Binary(t *testing.T) {
	m, err := macaroon.New([]byte("root-key"), []byte("id"), "location", macaroon.LatestVersion)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	bin, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(bin)

	decoded, err := DecodeMacaroonAny(encoded)
	if err != nil {
		t.Fatalf("DecodeMacaroonAny() error = %v", err)
	}
	if string(decoded.Id()) != "id" {
		t.Fatalf("Id = %q, want id", decoded.Id())
	}
}

func TestDecodeMacaroonAny_Invalid(t *testing.T) {
	_, err := DecodeMacaroonAny("not-a-macaroon")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestSerializeMacaroonSlice(t *testing.T) {
	m, err := macaroon.New([]byte("key"), []byte("id"), "loc", macaroon.LatestVersion)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := SerializeMacaroonSlice(macaroon.Slice{m})
	if err != nil {
		t.Fatalf("SerializeMacaroonSlice() error = %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestSerializeMacaroonSlice_Empty(t *testing.T) {
	_, err := SerializeMacaroonSlice(macaroon.Slice{})
	if err == nil {
		t.Fatal("expected error for empty slice")
	}
}
