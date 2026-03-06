// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
	"testing"
	"time"
)

func TestParseHumanDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"30m", 30 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"3d", 3 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"1w2d", 9 * 24 * time.Hour, false},
		{"1w2d3h", 9*24*time.Hour + 3*time.Hour, false},
		{"1w2d3h30m", 9*24*time.Hour + 3*time.Hour + 30*time.Minute, false},
		{"90s", 90 * time.Second, false},
		{"", 0, true},
		{"abc", 0, true},
		{"0d", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseHumanDuration(tc.input)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error for %q, got %v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseHumanDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveSince(t *testing.T) {
	// Empty returns empty.
	got, err := ResolveSince("")
	if err != nil || got != "" {
		t.Fatalf("ResolveSince(\"\") = %q, %v", got, err)
	}

	// ISO 8601 timestamp passes through.
	got, err = ResolveSince("2025-06-15T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2025-06-15T10:00:00Z" {
		t.Errorf("expected passthrough, got %q", got)
	}

	// Date-only format.
	got, err = ResolveSince("2025-06-15")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2025-06-15T00:00:00Z" {
		t.Errorf("expected date, got %q", got)
	}

	// Human duration produces a time near now.
	before := time.Now().Add(-2 * 24 * time.Hour).Add(-time.Minute)
	got, err = ResolveSince("2d")
	if err != nil {
		t.Fatal(err)
	}
	parsed, pErr := time.Parse(time.RFC3339, got)
	if pErr != nil {
		t.Fatalf("invalid RFC3339: %v", pErr)
	}
	if parsed.Before(before) {
		t.Errorf("resolved time %v is too far in the past", parsed)
	}

	// Invalid input.
	_, err = ResolveSince("bogus")
	if err == nil {
		t.Fatal("expected error for bogus input")
	}
}
