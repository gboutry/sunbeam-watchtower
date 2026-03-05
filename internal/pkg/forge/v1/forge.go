// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package v1 defines a unified interface for interacting with code forges
// (GitHub, Launchpad, Gerrit) and provides concrete implementations.
package v1

import (
	"context"
	"time"
)

// ForgeType identifies which forge a resource originates from.
type ForgeType int

const (
	ForgeGitHub ForgeType = iota
	ForgeLaunchpad
	ForgeGerrit
)

// String returns a human-readable name for the forge type.
func (f ForgeType) String() string {
	switch f {
	case ForgeGitHub:
		return "GitHub"
	case ForgeLaunchpad:
		return "Launchpad"
	case ForgeGerrit:
		return "Gerrit"
	default:
		return "Unknown"
	}
}

// MergeState represents the lifecycle state of a merge request.
type MergeState int

const (
	MergeStateOpen MergeState = iota
	MergeStateMerged
	MergeStateClosed
	MergeStateAbandoned
	MergeStateWIP
)

func (s MergeState) String() string {
	switch s {
	case MergeStateOpen:
		return "Open"
	case MergeStateMerged:
		return "Merged"
	case MergeStateClosed:
		return "Closed"
	case MergeStateAbandoned:
		return "Abandoned"
	case MergeStateWIP:
		return "WIP"
	default:
		return "Unknown"
	}
}

// ReviewState represents the review status of a merge request.
type ReviewState int

const (
	ReviewStatePending ReviewState = iota
	ReviewStateApproved
	ReviewStateChangesRequested
	ReviewStateRejected
)

func (s ReviewState) String() string {
	switch s {
	case ReviewStatePending:
		return "Pending"
	case ReviewStateApproved:
		return "Approved"
	case ReviewStateChangesRequested:
		return "Changes Requested"
	case ReviewStateRejected:
		return "Rejected"
	default:
		return "Unknown"
	}
}

// CheckState represents the status of a CI check.
type CheckState int

const (
	CheckStatePending CheckState = iota
	CheckStateRunning
	CheckStatePassed
	CheckStateFailed
)

func (s CheckState) String() string {
	switch s {
	case CheckStatePending:
		return "Pending"
	case CheckStateRunning:
		return "Running"
	case CheckStatePassed:
		return "Passed"
	case CheckStateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// Check represents a CI/CD check result on a merge request.
type Check struct {
	Name  string
	State CheckState
	URL   string
}

// MergeRequest unifies GitHub PRs, Launchpad merge proposals, and Gerrit changes.
type MergeRequest struct {
	Forge        ForgeType
	Repo         string // canonical repo identifier from config
	ID           string // "#123" for GH, MP self_link for LP, change number for Gerrit
	Title        string
	Description  string
	Author       string
	SourceBranch string
	TargetBranch string
	State        MergeState
	ReviewState  ReviewState
	Checks       []Check
	URL          string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CommitMergeRequest annotates a commit with its associated merge request info.
// Nil for commits on the main branch.
type CommitMergeRequest struct {
	ID    string     // "#123" for GitHub, change number for Gerrit, etc.
	State MergeState // Open, Merged, Closed, Abandoned, WIP
	URL   string     // web link to the merge request
}

// Commit is a forge-agnostic commit.
type Commit struct {
	Forge        ForgeType
	Repo         string
	SHA          string
	Message      string
	Author       string
	Date         time.Time
	URL          string
	BugRefs      []string             // extracted LP bug references (LP: #NNNNN, etc.)
	MergeRequest *CommitMergeRequest  // non-nil if commit comes from a merge request ref
}

// ListMergeRequestsOpts holds options for listing merge requests.
type ListMergeRequestsOpts struct {
	State  MergeState
	Author string
}

// ListCommitsOpts holds options for listing commits.
type ListCommitsOpts struct {
	Branch string
	Since  *time.Time
	Author string
}

// Forge is the unified interface for interacting with code forges.
// Each forge (GitHub, Launchpad, Gerrit) implements this interface.
type Forge interface {
	// Type returns which forge this client represents.
	Type() ForgeType

	// ListMergeRequests returns merge requests for the given repository.
	ListMergeRequests(ctx context.Context, repo string, opts ListMergeRequestsOpts) ([]MergeRequest, error)

	// GetMergeRequest returns a single merge request by its ID.
	GetMergeRequest(ctx context.Context, repo string, id string) (*MergeRequest, error)

	// ListCommits returns commits for the given repository.
	ListCommits(ctx context.Context, repo string, opts ListCommitsOpts) ([]Commit, error)
}
