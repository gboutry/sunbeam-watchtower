// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"
	"time"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// RecipeBuilder abstracts LP recipe operations for a specific artifact type.
type RecipeBuilder interface {
	ArtifactType() dto.ArtifactType
	GetRecipe(ctx context.Context, owner, project, name string) (*dto.Recipe, error)
	CreateRecipe(ctx context.Context, opts dto.CreateRecipeOpts) (*dto.Recipe, error)
	DeleteRecipe(ctx context.Context, recipeSelfLink string) error
	ListRecipesByOwner(ctx context.Context, owner string) ([]*dto.Recipe, error)
	RequestBuilds(ctx context.Context, recipe *dto.Recipe, opts dto.RequestBuildsOpts) (*dto.BuildRequest, error)
	ListBuilds(ctx context.Context, recipe *dto.Recipe) ([]dto.Build, error)
	RetryBuild(ctx context.Context, buildSelfLink string) error
	CancelBuild(ctx context.Context, buildSelfLink string) error
	GetBuildFileURLs(ctx context.Context, buildSelfLink string) ([]string, error)
}

// RepoManager handles temporary git repo/branch lifecycle on LP.
type RepoManager interface {
	GetCurrentUser(ctx context.Context) (string, error)
	GetDefaultRepo(ctx context.Context, projectName string) (repoSelfLink string, defaultBranch string, err error)
	GetOrCreateProject(ctx context.Context, owner string) (projectName string, err error)
	GetOrCreateRepo(ctx context.Context, owner, project, repoName string) (repoSelfLink, gitSSHURL string, err error)
	GetGitRef(ctx context.Context, repoSelfLink, refPath string) (refSelfLink string, err error)
	WaitForGitRef(ctx context.Context, repoSelfLink, refPath string, timeout time.Duration) (refSelfLink string, err error)
}
