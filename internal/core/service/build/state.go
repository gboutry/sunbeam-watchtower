// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

// BuildState represents the state of a build in Launchpad.
type BuildState int

const (
	BuildPending BuildState = iota
	BuildBuilding
	BuildSucceeded
	BuildFailed
	BuildCancelled
	BuildCancelling
	BuildSuperseded
	BuildUnknown
)

func (s BuildState) String() string {
	switch s {
	case BuildPending:
		return "Pending"
	case BuildBuilding:
		return "Building"
	case BuildSucceeded:
		return "Succeeded"
	case BuildFailed:
		return "Failed"
	case BuildCancelled:
		return "Cancelled"
	case BuildCancelling:
		return "Cancelling"
	case BuildSuperseded:
		return "Superseded"
	case BuildUnknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// ParseBuildState maps Launchpad's buildstate strings to BuildState.
func ParseBuildState(s string) BuildState {
	switch s {
	case "Needs building", "Dependency wait":
		return BuildPending
	case "Building", "Currently building", "Uploading build", "Gathering build output":
		return BuildBuilding
	case "Successfully built":
		return BuildSucceeded
	case "Failed to build", "Failed to upload", "Chroot problem":
		return BuildFailed
	case "Cancelled":
		return BuildCancelled
	case "Cancelling build":
		return BuildCancelling
	case "Superseded":
		return BuildSuperseded
	default:
		return BuildUnknown
	}
}

// IsTerminal returns true if the build is in a terminal state.
func (s BuildState) IsTerminal() bool {
	switch s {
	case BuildSucceeded, BuildFailed, BuildCancelled, BuildSuperseded:
		return true
	default:
		return false
	}
}

// IsActive returns true if the build is in an active state.
func (s BuildState) IsActive() bool {
	switch s {
	case BuildBuilding, BuildCancelling, BuildPending:
		return true
	default:
		return false
	}
}

// IsFailure returns true if the build ended in failure.
func (s BuildState) IsFailure() bool {
	switch s {
	case BuildFailed, BuildCancelled:
		return true
	default:
		return false
	}
}
