// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package charmhub

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAuthenticatorBeginAuthRequestsRootMacaroon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req tokensRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req.Description != "sunbeam-watchtower" {
			t.Fatalf("Description = %q, want sunbeam-watchtower", req.Description)
		}
		if len(req.Permissions) == 0 {
			t.Fatal("expected permissions")
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokensResponse{Macaroon: "fake-root-macaroon"})
	}))
	defer srv.Close()

	auth := NewAuthenticator(discardLogger(), srv.Client())
	auth.tokensEndpoint = srv.URL + "/v1/tokens"

	flow, err := auth.BeginAuth(context.Background())
	if err != nil {
		t.Fatalf("BeginAuth() error = %v", err)
	}
	if flow.RootMacaroon != "fake-root-macaroon" {
		t.Fatalf("RootMacaroon = %q, want fake-root-macaroon", flow.RootMacaroon)
	}
}

func TestAuthenticatorBeginAuthRejectsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	auth := NewAuthenticator(discardLogger(), srv.Client())
	auth.tokensEndpoint = srv.URL + "/v1/tokens"

	_, err := auth.BeginAuth(context.Background())
	if err == nil {
		t.Fatal("expected error on 403 response")
	}
}

func TestAuthenticatorBeginAuthRejectsEmptyMacaroon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(tokensResponse{Macaroon: ""})
	}))
	defer srv.Close()

	auth := NewAuthenticator(discardLogger(), srv.Client())
	auth.tokensEndpoint = srv.URL + "/v1/tokens"

	_, err := auth.BeginAuth(context.Background())
	if err == nil {
		t.Fatal("expected error on empty macaroon")
	}
}
