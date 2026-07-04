// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(handler http.Handler) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	creds := &Credentials{
		ConsumerKey:       "test-app",
		AccessToken:       "token",
		AccessTokenSecret: "secret",
	}
	c := NewClient(creds, nil, server.Client())
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
	var statusErr *HTTPError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error type = %T, want HTTPError", err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404", statusErr.StatusCode)
	}
}

func TestClient_GetRetriesTransientStatusThenSucceeds(t *testing.T) {
	attempts := 0
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"name": "test-user"}`))
	}))
	defer server.Close()

	data, err := c.Get(context.Background(), server.URL+"/~test-user")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if !strings.Contains(string(data), "test-user") {
		t.Fatalf("unexpected response: %s", data)
	}
}

func TestClient_GetRetriesTransientStatusCodes(t *testing.T) {
	statuses := []int{http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusTooManyRequests}

	for _, status := range statuses {
		t.Run(http.StatusText(status), func(t *testing.T) {
			attempts := 0
			c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts == 1 {
					http.Error(w, "temporary", status)
					return
				}
				w.Write([]byte(`{"name": "test-user"}`))
			}))
			defer server.Close()

			if _, err := c.Get(context.Background(), server.URL+"/~test-user"); err != nil {
				t.Fatalf("Get() error: %v", err)
			}
			if attempts != 2 {
				t.Fatalf("attempts = %d, want 2", attempts)
			}
		})
	}
}

func TestClient_GetDoesNotRetryPermanentStatus(t *testing.T) {
	attempts := 0
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := c.Get(context.Background(), server.URL+"/~nobody")
	if err == nil {
		t.Fatal("Get() error = nil, want HTTP error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestClient_PostDoesNotRetryTransientStatus(t *testing.T) {
	attempts := 0
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := c.Post(context.Background(), server.URL+"/action", nil)
	if err == nil {
		t.Fatal("Post() error = nil, want HTTP error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestClient_GetStopsRetryingWhenContextCanceled(t *testing.T) {
	attempts := 0
	ctx, cancel := context.WithCancel(context.Background())
	c, server := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		cancel()
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := c.Get(ctx, server.URL+"/~test-user")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want context canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestReadRetryDelayUsesOperationalBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 3, want: 4 * time.Second},
		{attempt: 4, want: 5 * time.Second},
	}

	for _, tt := range tests {
		if got := readRetryDelay(tt.attempt); got != tt.want {
			t.Fatalf("readRetryDelay(%d) = %s, want %s", tt.attempt, got, tt.want)
		}
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
	orig := requestTokenURL
	t.Cleanup(func() { requestTokenURL = orig })
	requestTokenURL = "http://127.0.0.1:1"

	_, _, err := Login("test-app", nil, nil)
	if err == nil {
		t.Fatal("expected error for nil promptFn, got nil")
	}
	if !strings.Contains(err.Error(), "promptFn is required") {
		t.Fatalf("err = %v, want promptFn requirement", err)
	}
}
