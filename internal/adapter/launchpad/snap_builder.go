// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package launchpad

import (
	"context"

	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// SnapBuilder implements port.RecipeBuilder for snap artifacts.
// Key differences from rock/charm:
// - GetRecipe ignores the project parameter (snap path is /~owner/+snap/name)
// - RequestBuilds requires archiveLink and pocket from opts
type SnapBuilder struct {
	client         *lp.Client
	defaultArchive string // default Ubuntu archive link
	defaultPocket  string // default pocket (e.g. "Updates")
}

var _ port.RecipeBuilder = (*SnapBuilder)(nil)

func NewSnapBuilder(client *lp.Client, defaultArchive, defaultPocket string) *SnapBuilder {
	return &SnapBuilder{
		client:         client,
		defaultArchive: defaultArchive,
		defaultPocket:  defaultPocket,
	}
}

func (s *SnapBuilder) ArtifactType() port.ArtifactType {
	return port.ArtifactSnap
}

func (s *SnapBuilder) GetRecipe(ctx context.Context, owner, _ string, name string) (*port.Recipe, error) {
	snap, err := s.client.GetSnap(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	return snapToPortRecipe(snap), nil
}

func (s *SnapBuilder) CreateRecipe(ctx context.Context, opts port.CreateRecipeOpts) (*port.Recipe, error) {
	snap, err := s.client.CreateSnap(ctx, lp.CreateSnapOpts{
		Name:        opts.Name,
		OwnerLink:   opts.Owner,
		GitRefLink:  opts.GitRefLink,
		Description: opts.Name,
	})
	if err != nil {
		return nil, err
	}
	return snapToPortRecipe(snap), nil
}

func (s *SnapBuilder) DeleteRecipe(ctx context.Context, selfLink string) error {
	return s.client.DeleteSnap(ctx, selfLink)
}

func (s *SnapBuilder) RequestBuilds(ctx context.Context, recipe *port.Recipe, opts port.RequestBuildsOpts) (*port.BuildRequest, error) {
	archiveLink := s.defaultArchive
	if opts.ArchiveLink != "" {
		archiveLink = opts.ArchiveLink
	}
	pocket := s.defaultPocket
	if opts.Pocket != "" {
		pocket = opts.Pocket
	}

	br, err := s.client.RequestSnapBuilds(ctx, recipe.SelfLink, archiveLink, pocket, opts.Channels)
	if err != nil {
		return nil, err
	}
	return buildRequestToPort(br), nil
}

func (s *SnapBuilder) ListBuilds(ctx context.Context, recipe *port.Recipe) ([]port.Build, error) {
	builds, err := s.client.GetSnapBuilds(ctx, recipe.SelfLink)
	if err != nil {
		return nil, err
	}
	out := make([]port.Build, len(builds))
	for i, b := range builds {
		out[i] = snapBuildToPortBuild(b, recipe.Name)
	}
	return out, nil
}

func (s *SnapBuilder) RetryBuild(ctx context.Context, selfLink string) error {
	return s.client.RetrySnapBuild(ctx, selfLink)
}

func (s *SnapBuilder) CancelBuild(ctx context.Context, selfLink string) error {
	return s.client.CancelSnapBuild(ctx, selfLink)
}

func (s *SnapBuilder) GetBuildFileURLs(ctx context.Context, selfLink string) ([]string, error) {
	return s.client.GetSnapBuildFileURLs(ctx, selfLink)
}

func snapToPortRecipe(s lp.Snap) *port.Recipe {
	return &port.Recipe{
		Name:         s.Name,
		ArtifactType: port.ArtifactSnap,
		Owner:        extractNameFromLink(s.OwnerLink),
		Project:      "",
		SelfLink:     s.SelfLink,
		WebLink:      s.WebLink,
		GitPath:      s.GitPath,
		BuildPath:    "",
		AutoBuild:    s.AutoBuild,
		CreatedAt:    timeOrZero(s.DateCreated),
	}
}

func snapBuildToPortBuild(b lp.SnapBuild, recipeName string) port.Build {
	return port.Build{
		Recipe:       recipeName,
		ArtifactType: port.ArtifactSnap,
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
