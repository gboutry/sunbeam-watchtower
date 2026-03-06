// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import "testing"

func TestForgeType_String(t *testing.T) {
	tests := []struct {
		ft   ForgeType
		want string
	}{
		{ForgeGitHub, "GitHub"},
		{ForgeLaunchpad, "Launchpad"},
		{ForgeGerrit, "Gerrit"},
		{ForgeType(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.ft.String(); got != tt.want {
			t.Errorf("ForgeType(%d).String() = %q, want %q", tt.ft, got, tt.want)
		}
	}
}

func TestMergeState_String(t *testing.T) {
	tests := []struct {
		s    MergeState
		want string
	}{
		{MergeStateOpen, "Open"},
		{MergeStateMerged, "Merged"},
		{MergeStateClosed, "Closed"},
		{MergeStateAbandoned, "Abandoned"},
		{MergeStateWIP, "WIP"},
		{MergeState(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("MergeState(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestReviewState_String(t *testing.T) {
	tests := []struct {
		s    ReviewState
		want string
	}{
		{ReviewStatePending, "Pending"},
		{ReviewStateApproved, "Approved"},
		{ReviewStateChangesRequested, "Changes Requested"},
		{ReviewStateRejected, "Rejected"},
		{ReviewState(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("ReviewState(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestCheckState_String(t *testing.T) {
	tests := []struct {
		s    CheckState
		want string
	}{
		{CheckStatePending, "Pending"},
		{CheckStateRunning, "Running"},
		{CheckStatePassed, "Passed"},
		{CheckStateFailed, "Failed"},
		{CheckState(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("CheckState(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}
