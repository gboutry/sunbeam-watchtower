// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetTeamMembersWithEmails verifies that:
//   - sub-teams are filtered out
//   - emails are resolved for members with an email link and hide_email_addresses=false
//   - emails are not fetched when hide_email_addresses=true
//   - members without an email link get an empty email
func TestGetTeamMembersWithEmails(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/~sunbeam-team/members", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Collection[Person]{
			TotalSize: 4,
			Entries: []Person{
				{
					Name:                      "alice",
					IsTeam:                    false,
					HideEmailAddresses:        false,
					PreferredEmailAddressLink: server.URL + "/emails/alice",
				},
				{
					Name:                      "bob",
					IsTeam:                    false,
					HideEmailAddresses:        true,
					PreferredEmailAddressLink: server.URL + "/emails/bob",
				},
				{
					Name:   "sub-team",
					IsTeam: true,
				},
				{
					Name:                      "carol",
					IsTeam:                    false,
					HideEmailAddresses:        false,
					PreferredEmailAddressLink: "",
				},
			},
		})
	})

	mux.HandleFunc("/emails/alice", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TeamMemberEmail{Email: "alice@example.com"})
	})

	mux.HandleFunc("/emails/bob", func(w http.ResponseWriter, _ *http.Request) {
		// Should never be reached: HideEmailAddresses=true for bob.
		t.Error("email endpoint fetched for bob whose email addresses are hidden")
		w.WriteHeader(http.StatusForbidden)
	})

	client := testRewrittenClient(t, server)

	members, err := client.GetTeamMembersWithEmails(context.Background(), "sunbeam-team")
	if err != nil {
		t.Fatalf("GetTeamMembersWithEmails() error = %v", err)
	}

	// sub-team must be filtered out; 3 real members remain.
	if len(members) != 3 {
		t.Fatalf("len(members) = %d, want 3 (alice, bob, carol)", len(members))
	}

	byName := make(map[string]string, len(members))
	for _, m := range members {
		byName[m.Username] = m.Email
	}

	if email, ok := byName["alice"]; !ok {
		t.Error("alice not in result")
	} else if email != "alice@example.com" {
		t.Errorf("alice email = %q, want alice@example.com", email)
	}

	if email, ok := byName["bob"]; !ok {
		t.Error("bob not in result")
	} else if email != "" {
		t.Errorf("bob email = %q, want empty (hidden)", email)
	}

	if email, ok := byName["carol"]; !ok {
		t.Error("carol not in result")
	} else if email != "" {
		t.Errorf("carol email = %q, want empty (no email link)", email)
	}

	if _, ok := byName["sub-team"]; ok {
		t.Error("sub-team should have been filtered out")
	}
}

// TestGetTeamMembersWithEmails_PropagatesError checks that errors from
// GetTeamMembers are propagated rather than silently dropped.
func TestGetTeamMembersWithEmails_PropagatesError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/~broken-team/members", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := testRewrittenClient(t, server)

	_, err := client.GetTeamMembersWithEmails(context.Background(), "broken-team")
	if err == nil {
		t.Fatal("expected error when members endpoint fails, got nil")
	}
}

// TestGetTeamMembersWithEmails_EmailFetchFailureIsIgnored verifies that a
// transient error on a single member's email resource does not abort the
// whole call; the member is returned with an empty email instead.
func TestGetTeamMembersWithEmails_EmailFetchFailureIsIgnored(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/~team/members", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Collection[Person]{
			TotalSize: 1,
			Entries: []Person{
				{
					Name:                      "dave",
					IsTeam:                    false,
					HideEmailAddresses:        false,
					PreferredEmailAddressLink: server.URL + "/emails/dave",
				},
			},
		})
	})

	mux.HandleFunc("/emails/dave", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := testRewrittenClient(t, server)

	members, err := client.GetTeamMembersWithEmails(context.Background(), "team")
	if err != nil {
		t.Fatalf("GetTeamMembersWithEmails() error = %v, want nil (email errors are ignored)", err)
	}
	if len(members) != 1 {
		t.Fatalf("len(members) = %d, want 1", len(members))
	}
	if members[0].Username != "dave" {
		t.Errorf("Username = %q, want dave", members[0].Username)
	}
	if members[0].Email != "" {
		t.Errorf("Email = %q, want empty (fetch failed)", members[0].Email)
	}
}
