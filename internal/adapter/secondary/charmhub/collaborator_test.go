// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package charmhub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
)

func TestCollaboratorManager_ListCollaborators(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/charm/my-charm/collaborators" {
			t.Fatalf("path = %q, want /v1/charm/my-charm/collaborators", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Macaroon test-auth" {
			t.Fatalf("Authorization = %q, want Macaroon test-auth", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"collaborators": []map[string]any{
				{"email": "alice@example.com", "username": "alice", "display-name": "Alice Smith", "status": "accepted"},
				{"email": "bob@example.com", "username": "bob", "display-name": "Bob Jones", "status": "pending"},
			},
		})
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("test-auth", server.Client())
	mgr.baseURL = server.URL

	collaborators, err := mgr.ListCollaborators(context.Background(), "my-charm")
	if err != nil {
		t.Fatalf("ListCollaborators() error = %v", err)
	}
	if len(collaborators) != 2 {
		t.Fatalf("ListCollaborators() = %d collaborators, want 2", len(collaborators))
	}
	if collaborators[0].Email != "alice@example.com" || collaborators[0].Username != "alice" || collaborators[0].Status != "accepted" {
		t.Fatalf("collaborators[0] = %+v, want alice/accepted", collaborators[0])
	}
	if collaborators[1].Email != "bob@example.com" || collaborators[1].Username != "bob" || collaborators[1].Status != "pending" {
		t.Fatalf("collaborators[1] = %+v, want bob/pending", collaborators[1])
	}
}

func TestCollaboratorManager_InviteCollaborator(t *testing.T) {
	var receivedBody struct {
		Invites []struct {
			Email string `json:"email"`
		} `json:"invites"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/charm/my-charm/collaborators/invites" {
			t.Fatalf("path = %q, want /v1/charm/my-charm/collaborators/invites", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Macaroon test-auth" {
			t.Fatalf("Authorization = %q, want Macaroon test-auth", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("test-auth", server.Client())
	mgr.baseURL = server.URL

	err := mgr.InviteCollaborator(context.Background(), "my-charm", "carol@example.com")
	if err != nil {
		t.Fatalf("InviteCollaborator() error = %v", err)
	}
	if len(receivedBody.Invites) != 1 || receivedBody.Invites[0].Email != "carol@example.com" {
		t.Fatalf("request invites = %+v, want single invite for carol@example.com", receivedBody.Invites)
	}
}

func TestCollaboratorManager_InviteCollaborator_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("bad-auth", server.Client())
	mgr.baseURL = server.URL

	err := mgr.InviteCollaborator(context.Background(), "my-charm", "carol@example.com")
	if err == nil {
		t.Fatal("InviteCollaborator() expected error for HTTP 403, got nil")
	}
}

func TestCollaboratorManager_ListCollaborators_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("bad-auth", server.Client())
	mgr.baseURL = server.URL

	_, err := mgr.ListCollaborators(context.Background(), "my-charm")
	if err == nil {
		t.Fatal("ListCollaborators() expected error for HTTP 401, got nil")
	}
}

// TestCollaboratorManager_ListCollaborators_DecodesErrorList: a 400 with
// the documented error-list body must surface both the status and the
// decoded code/message in the wrapped error string.
func TestCollaboratorManager_ListCollaborators_DecodesErrorList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error-list":[{"code":"macaroon-needs-refresh","message":"macaroon has expired"}]}`))
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("expired-auth", server.Client())
	mgr.baseURL = server.URL

	_, err := mgr.ListCollaborators(context.Background(), "my-charm")
	if err == nil {
		t.Fatal("ListCollaborators() expected error, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"HTTP 400", "macaroon-needs-refresh", "macaroon has expired"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	// An auth-error code maps to ErrUnauthorized even on a 400 status.
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("errors.Is(err, ErrUnauthorized) = false, want true (err=%v)", err)
	}
	if !errors.Is(err, port.ErrStoreAuthExpired) {
		t.Errorf("errors.Is(err, port.ErrStoreAuthExpired) = false, want true")
	}
}

// TestCollaboratorManager_ListCollaborators_401MapsToUnauthorized: plain
// 401 without a JSON body must still map to ErrUnauthorized.
func TestCollaboratorManager_ListCollaborators_401MapsToUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("bad-auth", server.Client())
	mgr.baseURL = server.URL

	_, err := mgr.ListCollaborators(context.Background(), "my-charm")
	if err == nil {
		t.Fatal("ListCollaborators() expected error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("errors.Is(err, ErrUnauthorized) = false for HTTP 401 (err=%v)", err)
	}
}

// TestCollaboratorManager_ListCollaborators_MalformedBodyFallsBack: when
// the response body isn't either documented JSON shape, the raw text must
// be included verbatim in the error.
func TestCollaboratorManager_ListCollaborators_MalformedBodyFallsBack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream exploded: backend timeout"))
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("ok-auth", server.Client())
	mgr.baseURL = server.URL

	_, err := mgr.ListCollaborators(context.Background(), "my-charm")
	if err == nil {
		t.Fatal("ListCollaborators() expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "HTTP 500") || !strings.Contains(msg, "upstream exploded") {
		t.Errorf("error %q should contain status + raw body fallback", msg)
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Errorf("plain 500 must not map to ErrUnauthorized (err=%v)", err)
	}
}

// TestCollaboratorManager_InviteCollaborator_DecodesSingleErrorShape:
// exercise the `{"error":{...}}` shape on the invite path.
func TestCollaboratorManager_InviteCollaborator_DecodesSingleErrorShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"permission-required","message":"missing package-manage-collaborators"}}`))
	}))
	defer server.Close()

	mgr := NewCollaboratorManager("ok-auth", server.Client())
	mgr.baseURL = server.URL

	err := mgr.InviteCollaborator(context.Background(), "my-charm", "x@example.com")
	if err == nil {
		t.Fatal("InviteCollaborator() expected error")
	}
	msg := err.Error()
	for _, want := range []string{"HTTP 403", "permission-required", "missing package-manage-collaborators"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("permission-required code should map to ErrUnauthorized (err=%v)", err)
	}
}
