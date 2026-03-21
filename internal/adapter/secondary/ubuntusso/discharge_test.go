// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package ubuntusso

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBeginDischargeExtractsVisitAndWaitURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req dischargeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if req.CaveatID != "test-caveat-id" {
			t.Fatalf("CaveatID = %q, want test-caveat-id", req.CaveatID)
		}

		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":      "interaction-required",
			"message":   "human verification required",
			"visit_url": "https://login.ubuntu.com/+interact/visit123",
			"wait_url":  "https://login.ubuntu.com/+interact/wait123",
		})
	}))
	defer srv.Close()

	visitURL, waitURL, err := BeginDischarge(context.Background(), srv.Client(), srv.URL, "test-caveat-id")
	if err != nil {
		t.Fatalf("BeginDischarge() error = %v", err)
	}
	if visitURL != "https://login.ubuntu.com/+interact/visit123" {
		t.Fatalf("visitURL = %q", visitURL)
	}
	if waitURL != "https://login.ubuntu.com/+interact/wait123" {
		t.Fatalf("waitURL = %q", waitURL)
	}
}

func TestBeginDischargeExtractsNestedURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Info": map[string]string{
				"visit_url": "https://login.ubuntu.com/+candid/visit",
				"wait_url":  "https://login.ubuntu.com/+candid/wait",
			},
		})
	}))
	defer srv.Close()

	visitURL, waitURL, err := BeginDischarge(context.Background(), srv.Client(), srv.URL, "caveat")
	if err != nil {
		t.Fatalf("BeginDischarge() error = %v", err)
	}
	if visitURL != "https://login.ubuntu.com/+candid/visit" {
		t.Fatalf("visitURL = %q", visitURL)
	}
	if waitURL != "https://login.ubuntu.com/+candid/wait" {
		t.Fatalf("waitURL = %q", waitURL)
	}
}

func TestBeginDischargeFailsOnMissingURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "interaction-required",
			"message": "missing URLs",
		})
	}))
	defer srv.Close()

	_, _, err := BeginDischarge(context.Background(), srv.Client(), srv.URL, "caveat")
	if err == nil {
		t.Fatal("expected error when visit/wait URLs are missing")
	}
}

func TestPollDischargeReturnsOnSuccess(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_ = json.NewEncoder(w).Encode(dischargeWaitResponse{
			DischargeMacaroon: "discharge-mac-b64",
		})
	}))
	defer srv.Close()

	result, err := PollDischarge(context.Background(), srv.Client(), srv.URL, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("PollDischarge() error = %v", err)
	}
	if result != "discharge-mac-b64" {
		t.Fatalf("result = %q, want discharge-mac-b64", result)
	}
	if callCount < 2 {
		t.Fatalf("callCount = %d, expected at least 2", callCount)
	}
}

func TestPollDischargeRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := PollDischarge(ctx, srv.Client(), srv.URL, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		values []string
		want   string
	}{
		{[]string{"", "", "c"}, "c"},
		{[]string{"a", "b"}, "a"},
		{[]string{"", ""}, ""},
	}
	for _, tt := range tests {
		got := firstNonEmpty(tt.values...)
		if got != tt.want {
			t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
		}
	}
}
