// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package launchpad

import (
	"context"

	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// RockBuilder implements port.RecipeBuilder for rock artifacts.
type RockBuilder struct {
	client *lp.Client
}

var _ port.RecipeBuilder = (*RockBuilder)(nil)

func NewRockBuilder(client *lp.Client) *RockBuilder {
	return &RockBuilder{client: client}
}

func (c *RockBuilder) ArtifactType() port.ArtifactType {
	return port.ArtifactRock
}

func (c *RockBuilder) GetRecipe(ctx context.Context, owner, project, name string) (*port.Recipe, error) {
	r, err := c.client.GetRockRecipe(ctx, owner, project, name)
	if err != nil {
		return nil, err
	}
	return rockRecipeToPortRecipe(r), nil
}

func (c *RockBuilder) CreateRecipe(ctx context.Context, opts port.CreateRecipeOpts) (*port.Recipe, error) {
	r, err := c.client.CreateRockRecipe(ctx, lp.CreateRockRecipeOpts{
		Name:       opts.Name,
		Owner:      opts.Owner,
		Project:    opts.Project,
		GitRefLink: opts.GitRefLink,
		BuildPath:  opts.BuildPath,
	})
	if err != nil {
		return nil, err
	}
	return rockRecipeToPortRecipe(r), nil
}

func (c *RockBuilder) DeleteRecipe(ctx context.Context, selfLink string) error {
	return c.client.DeleteRockRecipe(ctx, selfLink)
}

func (c *RockBuilder) RequestBuilds(ctx context.Context, recipe *port.Recipe, opts port.RequestBuildsOpts) (*port.BuildRequest, error) {
	br, err := c.client.RequestRockRecipeBuilds(ctx, recipe.SelfLink, opts.Channels, opts.Architectures)
	if err != nil {
		return nil, err
	}
	return buildRequestToPort(br), nil
}

func (c *RockBuilder) ListBuilds(ctx context.Context, recipe *port.Recipe) ([]port.Build, error) {
	builds, err := c.client.GetRockRecipeBuilds(ctx, recipe.SelfLink)
	if err != nil {
		return nil, err
	}
	out := make([]port.Build, len(builds))
	for i, b := range builds {
		out[i] = rockBuildToPortBuild(b, recipe.Name)
	}
	return out, nil
}

func (c *RockBuilder) RetryBuild(ctx context.Context, selfLink string) error {
	return c.client.RetryRockRecipeBuild(ctx, selfLink)
}

func (c *RockBuilder) CancelBuild(ctx context.Context, selfLink string) error {
	return c.client.CancelRockRecipeBuild(ctx, selfLink)
}

func (c *RockBuilder) GetBuildFileURLs(ctx context.Context, selfLink string) ([]string, error) {
	return c.client.GetRockRecipeBuildFileURLs(ctx, selfLink)
}

func rockRecipeToPortRecipe(r lp.RockRecipe) *port.Recipe {
	return &port.Recipe{
		Name:         r.Name,
		ArtifactType: port.ArtifactRock,
		Owner:        extractNameFromLink(r.OwnerLink),
		Project:      extractNameFromLink(r.ProjectLink),
		SelfLink:     r.SelfLink,
		WebLink:      r.WebLink,
		GitPath:      r.GitPath,
		BuildPath:    r.BuildPath,
		AutoBuild:    r.AutoBuild,
		CreatedAt:    timeOrZero(r.DateCreated),
	}
}

func rockBuildToPortBuild(b lp.RockRecipeBuild, recipeName string) port.Build {
	return port.Build{
		Recipe:       recipeName,
		ArtifactType: port.ArtifactRock,
		Title:        b.Title,
		State:        parseBuildState(b.BuildState),
		Arch:         b.ArchTag,
		BuildLogURL:  b.BuildLogURL,
		WebLink:      b.WebLink,
		SelfLink:     b.SelfLink,
		CanRetry:     b.CanBeRetried,
		CanCancel:    b.CanBeCancelled,
		CreatedAt:    timeOrZero(b.DateCreated),
		StartedAt:    timeOrZero(b.DateStarted),
		BuiltAt:      timeOrZero(b.DateBuilt),
	}
}
