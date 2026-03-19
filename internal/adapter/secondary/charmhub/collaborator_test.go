// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package charmhub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCollaboratorManager_ListCollaborators(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
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
	var receivedBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
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
	if receivedBody["email"] != "carol@example.com" {
		t.Fatalf("request body email = %q, want carol@example.com", receivedBody["email"])
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
