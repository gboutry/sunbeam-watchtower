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
		want    []BugRef
	}{
		{
			name:    "LP colon format",
			message: "Fix crash on startup\n\nLP: #12345",
			want:    []BugRef{{ID: "12345", Type: BugRefCloses}},
		},
		{
			name:    "Closes-Bug format",
			message: "Closes-Bug: #67890\n\nSome description",
			want:    []BugRef{{ID: "67890", Type: BugRefCloses}},
		},
		{
			name:    "Partial-Bug format",
			message: "Partial-Bug: 11111",
			want:    []BugRef{{ID: "11111", Type: BugRefPartial}},
		},
		{
			name:    "Related-Bug format",
			message: "Related-Bug: #22222",
			want:    []BugRef{{ID: "22222", Type: BugRefRelated}},
		},
		{
			name:    "multiple bugs different types",
			message: "LP: #100\nCloses-Bug: #200\nPartial-Bug: #300",
			want:    []BugRef{{ID: "100", Type: BugRefCloses}, {ID: "200", Type: BugRefCloses}, {ID: "300", Type: BugRefPartial}},
		},
		{
			name:    "duplicate deduplication strongest wins",
			message: "Related-Bug: #100\nCloses-Bug: #100",
			want:    []BugRef{{ID: "100", Type: BugRefCloses}},
		},
		{
			name:    "partial then closes promotes to closes",
			message: "Partial-Bug: #100\nLP: #100",
			want:    []BugRef{{ID: "100", Type: BugRefCloses}},
		},
		{
			name:    "no bugs",
			message: "Just a regular commit message",
			want:    nil,
		},
		{
			name:    "case insensitive",
			message: "closes-bug: #555\nRELATED-BUG: #666",
			want:    []BugRef{{ID: "555", Type: BugRefCloses}, {ID: "666", Type: BugRefRelated}},
		},
		{
			name:    "LP with multiple spaces",
			message: "LP:  #999",
			want:    nil, // LP: requires single space
		},
		{
			name:    "LP single space",
			message: "LP: #999",
			want:    []BugRef{{ID: "999", Type: BugRefCloses}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBugRefs(tt.message)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractBugRefs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i].ID != tt.want[i].ID {
					t.Errorf("ExtractBugRefs()[%d].ID = %q, want %q", i, got[i].ID, tt.want[i].ID)
				}
				if got[i].Type != tt.want[i].Type {
					t.Errorf("ExtractBugRefs()[%d].Type = %d, want %d", i, got[i].Type, tt.want[i].Type)
				}
			}
		})
	}
}
