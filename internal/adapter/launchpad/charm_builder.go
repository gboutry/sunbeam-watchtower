// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package launchpad

import (
	"context"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/port"

	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
)

// CharmBuilder adapts LP charm recipe operations to the port.RecipeBuilder interface.
type CharmBuilder struct {
	client *lp.Client
}

var _ port.RecipeBuilder = (*CharmBuilder)(nil)

// NewCharmBuilder creates a new CharmBuilder backed by the given LP client.
func NewCharmBuilder(client *lp.Client) *CharmBuilder {
	return &CharmBuilder{client: client}
}

func (b *CharmBuilder) ArtifactType() port.ArtifactType {
	return port.ArtifactCharm
}

func (b *CharmBuilder) GetRecipe(ctx context.Context, owner, project, name string) (*port.Recipe, error) {
	r, err := b.client.GetCharmRecipe(ctx, owner, project, name)
	if err != nil {
		return nil, err
	}
	return charmRecipeToPortRecipe(r), nil
}

func (b *CharmBuilder) CreateRecipe(ctx context.Context, opts port.CreateRecipeOpts) (*port.Recipe, error) {
	r, err := b.client.CreateCharmRecipe(ctx, lp.CreateCharmRecipeOpts{
		Name:        opts.Name,
		OwnerLink:   opts.Owner,
		ProjectLink: opts.Project,
		GitRefLink:  opts.GitRefLink,
		BuildPath:   opts.BuildPath,
	})
	if err != nil {
		return nil, err
	}
	return charmRecipeToPortRecipe(r), nil
}

func (b *CharmBuilder) DeleteRecipe(ctx context.Context, recipeSelfLink string) error {
	return b.client.DeleteCharmRecipe(ctx, recipeSelfLink)
}

func (b *CharmBuilder) RequestBuilds(ctx context.Context, recipe *port.Recipe, opts port.RequestBuildsOpts) (*port.BuildRequest, error) {
	br, err := b.client.RequestCharmRecipeBuilds(ctx, recipe.SelfLink, opts.Channels, opts.Architectures)
	if err != nil {
		return nil, err
	}
	return buildRequestToPort(br), nil
}

func (b *CharmBuilder) ListBuilds(ctx context.Context, recipe *port.Recipe) ([]port.Build, error) {
	builds, err := b.client.GetCharmRecipeBuilds(ctx, recipe.SelfLink)
	if err != nil {
		return nil, err
	}
	out := make([]port.Build, len(builds))
	for i, lb := range builds {
		out[i] = charmBuildToPortBuild(lb, recipe.Name)
	}
	return out, nil
}

func (b *CharmBuilder) RetryBuild(ctx context.Context, buildSelfLink string) error {
	return b.client.RetryCharmRecipeBuild(ctx, buildSelfLink)
}

func (b *CharmBuilder) CancelBuild(ctx context.Context, buildSelfLink string) error {
	return b.client.CancelCharmRecipeBuild(ctx, buildSelfLink)
}

func (b *CharmBuilder) GetBuildFileURLs(ctx context.Context, buildSelfLink string) ([]string, error) {
	return b.client.GetCharmRecipeBuildFileURLs(ctx, buildSelfLink)
}

// charmRecipeToPortRecipe converts an LP CharmRecipe to a port.Recipe.
func charmRecipeToPortRecipe(r lp.CharmRecipe) *port.Recipe {
	return &port.Recipe{
		Name:         r.Name,
		ArtifactType: port.ArtifactCharm,
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

// charmBuildToPortBuild converts an LP CharmRecipeBuild to a port.Build.
func charmBuildToPortBuild(b lp.CharmRecipeBuild, recipeName string) port.Build {
	return port.Build{
		Recipe:       recipeName,
		ArtifactType: port.ArtifactCharm,
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

// timeOrZero safely dereferences an *lp.Time, returning zero time if nil.
func timeOrZero(t *lp.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.Time
}

// parseBuildState maps LP build state strings to port.BuildState.
func parseBuildState(lpState string) port.BuildState {
	switch lpState {
	case "Needs building", "Dependency wait":
		return port.BuildPending
	case "Building", "Currently building", "Uploading build", "Gathering build output":
		return port.BuildBuilding
	case "Successfully built":
		return port.BuildSucceeded
	case "Failed to build", "Failed to upload", "Chroot problem":
		return port.BuildFailed
	case "Cancelled":
		return port.BuildCancelled
	case "Cancelling build":
		return port.BuildCancelling
	case "Superseded":
		return port.BuildSuperseded
	default:
		return port.BuildPending
	}
}

// extractNameFromLink extracts the last path segment from an LP link (e.g. "~owner" → "~owner").
func extractNameFromLink(link string) string {
	if link == "" {
		return ""
	}
	for i := len(link) - 1; i >= 0; i-- {
		if link[i] == '/' {
			return link[i+1:]
		}
	}
	return link
}

// buildRequestToPort converts an LP BuildRequest to a port.BuildRequest.
func buildRequestToPort(br lp.BuildRequest) *port.BuildRequest {
	return &port.BuildRequest{
		SelfLink:             br.SelfLink,
		WebLink:              br.WebLink,
		Status:               br.Status,
		ErrorMessage:         br.ErrorMessage,
		BuildsCollectionLink: br.BuildsCollectionLink,
	}
}
