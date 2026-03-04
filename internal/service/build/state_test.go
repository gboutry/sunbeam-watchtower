// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import "testing"

func TestParseBuildState(t *testing.T) {
	tests := []struct {
		input string
		want  BuildState
	}{
		{"Needs building", BuildPending},
		{"Dependency wait", BuildPending},
		{"Building", BuildBuilding},
		{"Currently building", BuildBuilding},
		{"Uploading build", BuildBuilding},
		{"Gathering build output", BuildBuilding},
		{"Successfully built", BuildSucceeded},
		{"Failed to build", BuildFailed},
		{"Failed to upload", BuildFailed},
		{"Chroot problem", BuildFailed},
		{"Cancelled", BuildCancelled},
		{"Cancelling build", BuildCancelling},
		{"Superseded", BuildSuperseded},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseBuildState(tt.input); got != tt.want {
				t.Errorf("ParseBuildState(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseBuildStateUnknown(t *testing.T) {
	unknowns := []string{"", "something random", "built", "pending"}
	for _, s := range unknowns {
		t.Run(s, func(t *testing.T) {
			if got := ParseBuildState(s); got != BuildUnknown {
				t.Errorf("ParseBuildState(%q) = %v, want BuildUnknown", s, got)
			}
		})
	}
}

func TestBuildStateString(t *testing.T) {
	tests := []struct {
		state BuildState
		want  string
	}{
		{BuildPending, "Pending"},
		{BuildBuilding, "Building"},
		{BuildSucceeded, "Succeeded"},
		{BuildFailed, "Failed"},
		{BuildCancelled, "Cancelled"},
		{BuildCancelling, "Cancelling"},
		{BuildSuperseded, "Superseded"},
		{BuildUnknown, "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("BuildState(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := map[BuildState]bool{
		BuildPending:    false,
		BuildBuilding:   false,
		BuildSucceeded:  true,
		BuildFailed:     true,
		BuildCancelled:  true,
		BuildCancelling: false,
		BuildSuperseded: true,
		BuildUnknown:    false,
	}
	for state, want := range terminal {
		t.Run(state.String(), func(t *testing.T) {
			if got := state.IsTerminal(); got != want {
				t.Errorf("%v.IsTerminal() = %v, want %v", state, got, want)
			}
		})
	}
}

func TestIsActive(t *testing.T) {
	active := map[BuildState]bool{
		BuildPending:    true,
		BuildBuilding:   true,
		BuildSucceeded:  false,
		BuildFailed:     false,
		BuildCancelled:  false,
		BuildCancelling: true,
		BuildSuperseded: false,
		BuildUnknown:    false,
	}
	for state, want := range active {
		t.Run(state.String(), func(t *testing.T) {
			if got := state.IsActive(); got != want {
				t.Errorf("%v.IsActive() = %v, want %v", state, got, want)
			}
		})
	}
}

func TestIsFailure(t *testing.T) {
	failure := map[BuildState]bool{
		BuildPending:    false,
		BuildBuilding:   false,
		BuildSucceeded:  false,
		BuildFailed:     true,
		BuildCancelled:  true,
		BuildCancelling: false,
		BuildSuperseded: false,
		BuildUnknown:    false,
	}
	for state, want := range failure {
		t.Run(state.String(), func(t *testing.T) {
			if got := state.IsFailure(); got != want {
				t.Errorf("%v.IsFailure() = %v, want %v", state, got, want)
			}
		})
	}
}
