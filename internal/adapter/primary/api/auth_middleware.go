// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// TransportKind identifies how the client connected to the server.
type TransportKind string

const (
	// TransportUnix indicates the client connected over a Unix domain socket.
	// Unix socket connections are considered trusted and bypass bearer auth.
	TransportUnix TransportKind = "unix"

	// TransportTCP indicates the client connected over TCP and must present a
	// valid bearer token.
	TransportTCP TransportKind = "tcp"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

// TransportKindKey is the context key used to store the TransportKind for an
// incoming request. It is set by the server listener (Task 4) before the
// request reaches any handler or middleware.
const TransportKindKey contextKey = "transportKind"

// healthPath is the liveness probe path that is always exempt from auth.
const healthPath = "/api/v1/health"

// BearerAuthMiddleware returns chi-compatible middleware that enforces bearer
// token authentication for TCP connections. Unix socket connections are passed
// through without checking credentials, as are requests to the health endpoint.
func BearerAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Unix socket connections are trusted. If the transport kind is
			// absent from context (e.g. middleware misconfiguration), the zero
			// value falls through to the token check — deny-by-default.
			if kind, _ := r.Context().Value(TransportKindKey).(TransportKind); kind == TransportUnix {
				next.ServeHTTP(w, r)
				return
			}

			// Health endpoint is always exempt (liveness probes must not require auth).
			if r.URL.Path == healthPath {
				next.ServeHTTP(w, r)
				return
			}

			// Extract and validate the bearer token.
			authHeader := r.Header.Get("Authorization")
			bearerToken, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(bearerToken), []byte(token)) != 1 {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GenerateToken generates a cryptographically random 32-byte token and returns
// it hex-encoded (64 characters).
func GenerateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// writeUnauthorized writes a 401 Unauthorized response with a JSON body.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
}
