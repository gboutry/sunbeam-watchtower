// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import "testing"

func TestSuiteMarker(t *testing.T) {
	tests := []struct {
		suite string
		want  string
	}{
		{"noble", "R"},
		{"noble-updates", "U"},
		{"noble-proposed", "P"},
		{"noble-security", "S"},
		{"bookworm", "R"},
		{"unstable", "U"},
		{"experimental", "E"},
		{"cloud-archive-staging", "S"},
		{"zesty-updates", "U"},
		{"", "R"},
	}
	for _, tt := range tests {
		t.Run(tt.suite, func(t *testing.T) {
			got := suiteMarker(tt.suite)
			if got != tt.want {
				t.Errorf("suiteMarker(%q) = %q, want %q", tt.suite, got, tt.want)
			}
		})
	}
}
