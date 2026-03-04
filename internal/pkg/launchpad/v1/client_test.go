// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(handler http.Handler) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	creds := &Credentials{
		ConsumerKey:       "test-app",
		AccessToken:       "token",
		AccessTokenSecret: "secret",
	}
	c := NewClient(creds, nil)
	return c, server
}

func TestClient_Get(t *testing.T) {
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if !strings.Contains(auth, `oauth_consumer_key="test-app"`) {
			t.Error("request not signed")
		}

		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want application/json", r.Header.Get("Accept"))
		}

		w.Write([]byte(`{"name": "test-user"}`))
	}))
	defer server.Close()

	data, err := c.Get(context.Background(), server.URL+"/~test-user")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if !strings.Contains(string(data), "test-user") {
		t.Errorf("unexpected response: %s", data)
	}
}

func TestClient_GetJSON(t *testing.T) {
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Person{Name: "jdoe", SelfLink: "https://api.launchpad.net/devel/~jdoe"})
	}))
	defer server.Close()

	var p Person
	err := c.GetJSON(context.Background(), server.URL+"/~jdoe", &p)
	if err != nil {
		t.Fatalf("GetJSON() error: %v", err)
	}
	if p.Name != "jdoe" {
		t.Errorf("Name = %q, want %q", p.Name, "jdoe")
	}
}

func TestClient_Get_HTTPError(t *testing.T) {
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	_, err := c.Get(context.Background(), server.URL+"/~nobody")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestClient_Post(t *testing.T) {
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want form-urlencoded", ct)
		}

		auth := r.Header.Get("Authorization")
		if !strings.Contains(auth, "oauth_consumer_key") {
			t.Error("request not signed")
		}

		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	data, err := c.Post(context.Background(), server.URL+"/action", nil)
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}
	if !strings.Contains(string(data), "ok") {
		t.Errorf("unexpected response: %s", data)
	}
}

func TestClient_Delete(t *testing.T) {
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := c.Delete(context.Background(), server.URL+"/resource")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
}

func TestClient_ResolveURL(t *testing.T) {
	c := NewClient(&Credentials{}, nil)

	tests := []struct {
		input string
		want  string
	}{
		{"/~username", APIBaseURL + "/~username"},
		{"https://api.launchpad.net/devel/~me", "https://api.launchpad.net/devel/~me"},
	}

	for _, tt := range tests {
		got := c.resolveURL(tt.input)
		if got != tt.want {
			t.Errorf("resolveURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLogin_NilPromptFn(t *testing.T) {
	_, _, err := Login("test-app", nil, nil)
	if err == nil {
		t.Fatal("expected error for nil promptFn, got nil")
	}
}
