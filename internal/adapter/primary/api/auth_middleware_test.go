// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// okHandler is a simple handler that returns 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestBearerAuthMiddleware_AllowsUnixSocket(t *testing.T) {
	mw := BearerAuthMiddleware("secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/builds", nil)
	req = req.WithContext(context.WithValue(req.Context(), TransportKindKey, TransportUnix))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for unix socket connection, got %d", rr.Code)
	}
}

func TestBearerAuthMiddleware_RejectsTCPWithoutToken(t *testing.T) {
	mw := BearerAuthMiddleware("secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/builds", nil)
	req = req.WithContext(context.WithValue(req.Context(), TransportKindKey, TransportTCP))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for TCP without token, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	want := `{"error":"authentication required"}`
	if body != want {
		t.Fatalf("expected body %q, got %q", want, body)
	}
}

func TestBearerAuthMiddleware_AllowsTCPWithValidToken(t *testing.T) {
	mw := BearerAuthMiddleware("secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/builds", nil)
	req = req.WithContext(context.WithValue(req.Context(), TransportKindKey, TransportTCP))
	req.Header.Set("Authorization", "Bearer secret")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for TCP with valid token, got %d", rr.Code)
	}
}

func TestBearerAuthMiddleware_RejectsTCPWithWrongToken(t *testing.T) {
	mw := BearerAuthMiddleware("secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/builds", nil)
	req = req.WithContext(context.WithValue(req.Context(), TransportKindKey, TransportTCP))
	req.Header.Set("Authorization", "Bearer wrong")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for TCP with wrong token, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	want := `{"error":"authentication required"}`
	if body != want {
		t.Fatalf("expected body %q, got %q", want, body)
	}
}

func TestBearerAuthMiddleware_HealthExempt(t *testing.T) {
	mw := BearerAuthMiddleware("secret")
	handler := mw(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req = req.WithContext(context.WithValue(req.Context(), TransportKindKey, TransportTCP))
	// No Authorization header — should still pass.

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for health endpoint without token, got %d", rr.Code)
	}
}

func TestGenerateToken(t *testing.T) {
	tok1, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if len(tok1) != 64 {
		t.Fatalf("expected token length 64, got %d", len(tok1))
	}

	tok2, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() second call error = %v", err)
	}
	if tok1 == tok2 {
		t.Fatal("expected two GenerateToken calls to produce different values")
	}
}
