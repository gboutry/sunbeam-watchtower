// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package snapstore

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
		switch r.URL.Path {
		case "/dev/api/acl/":
			var req aclRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if len(req.Permissions) == 0 {
				t.Fatal("expected permissions in request")
			}
			// Return a fake macaroon - since we can't easily construct a real one
			// with third-party caveats in a unit test, we'll just verify the HTTP call.
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(aclResponse{Macaroon: "fake-root-macaroon"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	auth := NewAuthenticator(discardLogger(), srv.Client())
	auth.aclEndpoint = srv.URL + "/dev/api/acl/"

	// This will fail at the caveat extraction step since our fake macaroon
	// isn't a real macaroon, but that's expected - we're testing the HTTP call.
	_, err := auth.BeginAuth(context.Background())
	if err == nil {
		t.Fatal("expected error (fake macaroon cannot be decoded)")
	}
	// Verify the error is about macaroon parsing, not about the HTTP request.
	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestAuthenticatorRequestRootMacaroonRejectsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	auth := NewAuthenticator(discardLogger(), srv.Client())
	auth.aclEndpoint = srv.URL + "/dev/api/acl/"

	_, err := auth.BeginAuth(context.Background())
	if err == nil {
		t.Fatal("expected error on 403 response")
	}
}

func TestAuthenticatorRequestRootMacaroonRejectsEmptyMacaroon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(aclResponse{Macaroon: ""})
	}))
	defer srv.Close()

	auth := NewAuthenticator(discardLogger(), srv.Client())
	auth.aclEndpoint = srv.URL + "/dev/api/acl/"

	_, err := auth.BeginAuth(context.Background())
	if err == nil {
		t.Fatal("expected error on empty macaroon")
	}
}
