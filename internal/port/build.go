// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"
	"fmt"
	"time"
)

// ArtifactType identifies the kind of artifact being built.
type ArtifactType int

const (
	ArtifactRock ArtifactType = iota
	ArtifactCharm
	ArtifactSnap
)

func (a ArtifactType) String() string {
	switch a {
	case ArtifactRock:
		return "rock"
	case ArtifactCharm:
		return "charm"
	case ArtifactSnap:
		return "snap"
	default:
		return "unknown"
	}
}

// ParseArtifactType parses a string into an ArtifactType.
func ParseArtifactType(s string) (ArtifactType, error) {
	switch s {
	case "rock":
		return ArtifactRock, nil
	case "charm":
		return ArtifactCharm, nil
	case "snap":
		return ArtifactSnap, nil
	default:
		return 0, fmt.Errorf("unknown artifact type %q (must be rock, charm, or snap)", s)
	}
}

// BuildState represents the lifecycle state of a build.
// NOTE: The full state machine with parsing is in service/build/state.go.
// This is the port-level type.
type BuildState int

const (
	BuildPending BuildState = iota
	BuildBuilding
	BuildSucceeded
	BuildFailed
	BuildCancelled
	BuildCancelling
	BuildSuperseded
)

func (s BuildState) String() string {
	switch s {
	case BuildPending:
		return "pending"
	case BuildBuilding:
		return "building"
	case BuildSucceeded:
		return "succeeded"
	case BuildFailed:
		return "failed"
	case BuildCancelled:
		return "cancelled"
	case BuildCancelling:
		return "cancelling"
	case BuildSuperseded:
		return "superseded"
	default:
		return "unknown"
	}
}

// IsTerminal returns true if the build is in a final state.
func (s BuildState) IsTerminal() bool {
	switch s {
	case BuildSucceeded, BuildFailed, BuildCancelled, BuildSuperseded:
		return true
	default:
		return false
	}
}

// IsActive returns true if the build is still in progress.
func (s BuildState) IsActive() bool {
	switch s {
	case BuildBuilding, BuildCancelling, BuildPending:
		return true
	default:
		return false
	}
}

// IsFailure returns true if the build ended in a failure state.
func (s BuildState) IsFailure() bool {
	switch s {
	case BuildFailed, BuildCancelled:
		return true
	default:
		return false
	}
}

// Recipe represents a buildable recipe on LP (unified across rock/charm/snap).
type Recipe struct {
	Name         string
	ArtifactType ArtifactType
	Owner        string
	Project      string
	SelfLink     string
	WebLink      string
	GitPath      string
	BuildPath    string
	AutoBuild    bool
	CreatedAt    time.Time
}

// Build represents a single build of a recipe (unified).
type Build struct {
	Recipe       string // recipe name
	Project      string // watchtower project name (set by service layer)
	ArtifactType ArtifactType
	Title        string
	State        BuildState
	Arch         string
	BuildLogURL  string
	WebLink      string
	SelfLink     string
	CanRetry     bool
	CanCancel    bool
	CreatedAt    time.Time
	StartedAt    time.Time
	BuiltAt      time.Time
}

// BuildRequest represents the result of requesting builds.
type BuildRequest struct {
	SelfLink             string
	WebLink              string
	Status               string
	ErrorMessage         string
	BuildsCollectionLink string
}

// CreateRecipeOpts holds parameters for creating a new recipe.
type CreateRecipeOpts struct {
	Name        string
	Owner       string
	Project     string // LP project name
	GitRepoLink string // self_link of the LP git repo
	GitRefLink  string // self_link of the LP git ref
	BuildPath   string // e.g. "rocks/keystone"
}

// RequestBuildsOpts holds parameters for requesting builds.
type RequestBuildsOpts struct {
	Channels      map[string]string
	Architectures []string
	// Snap-specific
	ArchiveLink string
	Pocket      string
}

// RecipeBuilder abstracts LP recipe operations for a specific artifact type.
type RecipeBuilder interface {
	ArtifactType() ArtifactType

	GetRecipe(ctx context.Context, owner, project, name string) (*Recipe, error)
	CreateRecipe(ctx context.Context, opts CreateRecipeOpts) (*Recipe, error)
	DeleteRecipe(ctx context.Context, recipeSelfLink string) error

	RequestBuilds(ctx context.Context, recipe *Recipe, opts RequestBuildsOpts) (*BuildRequest, error)
	ListBuilds(ctx context.Context, recipe *Recipe) ([]Build, error)
	RetryBuild(ctx context.Context, buildSelfLink string) error
	CancelBuild(ctx context.Context, buildSelfLink string) error
	GetBuildFileURLs(ctx context.Context, buildSelfLink string) ([]string, error)
}

// RepoManager handles temporary git repo/branch lifecycle on LP.
type RepoManager interface {
	GetOrCreateProject(ctx context.Context, owner string) (projectName string, err error)
	GetOrCreateRepo(ctx context.Context, owner, project, repoName string) (repoSelfLink, gitSSHURL string, err error)
	GetGitRef(ctx context.Context, repoSelfLink, refPath string) (refSelfLink string, err error)
	WaitForGitRef(ctx context.Context, repoSelfLink, refPath string, timeout time.Duration) (refSelfLink string, err error)
}
