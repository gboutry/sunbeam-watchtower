// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
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

func (s BuildState) IsTerminal() bool {
	switch s {
	case BuildSucceeded, BuildFailed, BuildCancelled, BuildSuperseded:
		return true
	default:
		return false
	}
}

func (s BuildState) IsActive() bool {
	switch s {
	case BuildBuilding, BuildCancelling, BuildPending:
		return true
	default:
		return false
	}
}

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
	Name         string       `json:"name" yaml:"name"`
	ArtifactType ArtifactType `json:"artifact_type" yaml:"artifact_type"`
	Owner        string       `json:"owner" yaml:"owner"`
	Project      string       `json:"project" yaml:"project"`
	SelfLink     string       `json:"self_link" yaml:"self_link"`
	WebLink      string       `json:"web_link" yaml:"web_link"`
	GitPath      string       `json:"git_path" yaml:"git_path"`
	BuildPath    string       `json:"build_path" yaml:"build_path"`
	AutoBuild    bool         `json:"auto_build" yaml:"auto_build"`
	CreatedAt    time.Time    `json:"created_at" yaml:"created_at"`
}

// Build represents a single build of a recipe (unified).
type Build struct {
	Recipe       string       `json:"recipe" yaml:"recipe"`
	Project      string       `json:"project" yaml:"project"`
	ArtifactType ArtifactType `json:"artifact_type" yaml:"artifact_type"`
	Title        string       `json:"title" yaml:"title"`
	State        BuildState   `json:"state" yaml:"state"`
	Arch         string       `json:"arch" yaml:"arch"`
	BuildLogURL  string       `json:"build_log_url,omitempty" yaml:"build_log_url,omitempty"`
	WebLink      string       `json:"web_link" yaml:"web_link"`
	SelfLink     string       `json:"self_link" yaml:"self_link"`
	CanRetry     bool         `json:"can_retry" yaml:"can_retry"`
	CanCancel    bool         `json:"can_cancel" yaml:"can_cancel"`
	CreatedAt    time.Time    `json:"created_at" yaml:"created_at"`
	StartedAt    time.Time    `json:"started_at" yaml:"started_at"`
	BuiltAt      time.Time    `json:"built_at" yaml:"built_at"`
}

// BuildRequest represents the result of requesting builds.
type BuildRequest struct {
	SelfLink             string `json:"self_link" yaml:"self_link"`
	WebLink              string `json:"web_link" yaml:"web_link"`
	Status               string `json:"status" yaml:"status"`
	ErrorMessage         string `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	BuildsCollectionLink string `json:"builds_collection_link,omitempty" yaml:"builds_collection_link,omitempty"`
}

// PreparedBuildBackend identifies the backend that can execute a prepared build source.
type PreparedBuildBackend string

const (
	PreparedBuildBackendLaunchpad PreparedBuildBackend = "launchpad"
)

// PreparedBuildRecipe holds prepared recipe inputs for one backend recipe/buildable unit.
type PreparedBuildRecipe struct {
	SourceRef string `json:"source_ref" yaml:"source_ref"`
	BuildPath string `json:"build_path,omitempty" yaml:"build_path,omitempty"`
}

// PreparedBuildSource holds frontend-prepared backend references for split build workflows.
// It is produced locally and sent to the server so the server can execute the
// durable build workflow without needing local filesystem access.
//
// The generic fields are the preferred contract. The Launchpad-specific fields
// are kept as compatibility aliases for existing clients and are normalized into
// the generic view by helper methods on this type.
type PreparedBuildSource struct {
	Backend       PreparedBuildBackend           `json:"backend,omitempty" yaml:"backend,omitempty"`
	TargetProject string                         `json:"target_project,omitempty" yaml:"target_project,omitempty"`
	Repository    string                         `json:"repository,omitempty" yaml:"repository,omitempty"`
	Recipes       map[string]PreparedBuildRecipe `json:"recipes,omitempty" yaml:"recipes,omitempty"`

	LPProject    string            `json:"lp_project,omitempty" yaml:"lp_project,omitempty"`
	RepoSelfLink string            `json:"repo_self_link,omitempty" yaml:"repo_self_link,omitempty"`
	GitRefLinks  map[string]string `json:"git_ref_links,omitempty" yaml:"git_ref_links,omitempty"`
	BuildPaths   map[string]string `json:"build_paths,omitempty" yaml:"build_paths,omitempty"`
}

// Normalize returns a copy with compatibility aliases folded into the generic fields.
func (p *PreparedBuildSource) Normalize() *PreparedBuildSource {
	if p == nil {
		return nil
	}

	normalized := *p
	if normalized.Backend == "" && (normalized.LPProject != "" || normalized.RepoSelfLink != "" || len(normalized.GitRefLinks) > 0) {
		normalized.Backend = PreparedBuildBackendLaunchpad
	}
	if normalized.TargetProject == "" {
		normalized.TargetProject = normalized.LPProject
	}
	if normalized.Repository == "" {
		normalized.Repository = normalized.RepoSelfLink
	}
	if len(normalized.Recipes) == 0 && (len(normalized.GitRefLinks) > 0 || len(normalized.BuildPaths) > 0) {
		normalized.Recipes = make(map[string]PreparedBuildRecipe, len(normalized.GitRefLinks))
		for recipeName, sourceRef := range normalized.GitRefLinks {
			normalized.Recipes[recipeName] = PreparedBuildRecipe{
				SourceRef: sourceRef,
				BuildPath: normalized.BuildPaths[recipeName],
			}
		}
		for recipeName, buildPath := range normalized.BuildPaths {
			if _, ok := normalized.Recipes[recipeName]; ok {
				continue
			}
			normalized.Recipes[recipeName] = PreparedBuildRecipe{BuildPath: buildPath}
		}
	}

	return &normalized
}

// CreateRecipeOpts holds parameters for creating a new recipe.
type CreateRecipeOpts struct {
	Name        string
	Owner       string
	Project     string
	GitRepoLink string
	GitRefLink  string
	BuildPath   string
}

// RequestBuildsOpts holds parameters for requesting builds.
type RequestBuildsOpts struct {
	Channels      map[string]string
	Architectures []string
	ArchiveLink   string
	Pocket        string
}

// BuildRecipeAction is the action determined for a recipe after assessment.
type BuildRecipeAction int

const (
	BuildActionCreateRecipe BuildRecipeAction = iota
	BuildActionRequestBuilds
	BuildActionRetryFailed
	BuildActionMonitor
	BuildActionDownload
	BuildActionNoop
)

// BuildTriggerResult holds the result of a trigger operation.
type BuildTriggerResult struct {
	Project       string              `json:"project" yaml:"project"`
	RecipeResults []BuildRecipeResult `json:"recipe_results" yaml:"recipe_results"`
}

// BuildRecipeResult holds the result of a single recipe action.
type BuildRecipeResult struct {
	Name         string            `json:"name" yaml:"name"`
	Action       BuildRecipeAction `json:"action" yaml:"action"`
	Recipe       *Recipe           `json:"-" yaml:"-"`
	BuildRequest *BuildRequest     `json:"build_request,omitempty" yaml:"build_request,omitempty"`
	Builds       []Build           `json:"builds,omitempty" yaml:"builds,omitempty"`
	ErrorMessage string            `json:"error,omitempty" yaml:"error,omitempty"`
	Error        error             `json:"-" yaml:"-"`
}
