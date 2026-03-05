// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0-1", "1.0-2", -1},
		{"1.0-2", "1.0-1", 1},
		{"1.0-1", "1.0-1", 0},
		{"2:1.0-1", "1:2.0-1", 1},
		{"1:29.0.0-0ubuntu1", "1:29.1.0-0ubuntu1", -1},
		{"29.0.0-1", "1:29.0.0-0ubuntu1", -1},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestStripDebianRevision(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"3:32.0.0-0ubuntu1", "3:32.0.0"},
		{"1.0-1", "1.0"},
		{"1.0", "1.0"},
	}

	for _, tt := range tests {
		got := StripDebianRevision(tt.input)
		if got != tt.want {
			t.Errorf("StripDebianRevision(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPickHighest(t *testing.T) {
	pkgs := []SourcePackage{
		{Package: "nova", Version: "1:29.0.0-0ubuntu1", Suite: "noble", Component: "main"},
		{Package: "nova", Version: "1:29.1.0-0ubuntu1", Suite: "noble-updates", Component: "main"},
		{Package: "nova", Version: "1:29.0.0-0ubuntu2", Suite: "noble-proposed", Component: "main"},
	}

	best := PickHighest(pkgs)
	if best == nil {
		t.Fatal("expected non-nil result")
	}
	if best.Version != "1:29.1.0-0ubuntu1" {
		t.Errorf("PickHighest = %q, want %q", best.Version, "1:29.1.0-0ubuntu1")
	}

	if PickHighest(nil) != nil {
		t.Error("expected nil for empty slice")
	}
}
