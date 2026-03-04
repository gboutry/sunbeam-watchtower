// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"testing"
)

func TestExtractBugRefs(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    []string
	}{
		{
			name:    "LP colon format",
			message: "Fix crash on startup\n\nLP: #12345",
			want:    []string{"12345"},
		},
		{
			name:    "Closes-Bug format",
			message: "Closes-Bug: #67890\n\nSome description",
			want:    []string{"67890"},
		},
		{
			name:    "Partial-Bug format",
			message: "Partial-Bug: 11111",
			want:    []string{"11111"},
		},
		{
			name:    "Related-Bug format",
			message: "Related-Bug: #22222",
			want:    []string{"22222"},
		},
		{
			name:    "multiple bugs",
			message: "LP: #100\nCloses-Bug: #200\nPartial-Bug: #300",
			want:    []string{"100", "200", "300"},
		},
		{
			name:    "duplicate deduplication",
			message: "LP: #100\nLP: #100",
			want:    []string{"100"},
		},
		{
			name:    "no bugs",
			message: "Just a regular commit message",
			want:    nil,
		},
		{
			name:    "case insensitive",
			message: "closes-bug: #555\nRELATED-BUG: #666",
			want:    []string{"555", "666"},
		},
		{
			name:    "LP with multiple spaces",
			message: "LP:  #999",
			want:    nil, // LP: requires single space
		},
		{
			name:    "LP single space",
			message: "LP: #999",
			want:    []string{"999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBugRefs(tt.message)
			if len(got) != len(tt.want) {
				t.Fatalf("extractBugRefs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractBugRefs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
