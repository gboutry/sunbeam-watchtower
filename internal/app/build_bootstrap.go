// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	adaptergit "github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/git"
	lpadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/launchpad"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

// BuildRecipeBuilders creates per-project RecipeBuilder instances from config.
func (a *App) BuildRecipeBuilders() (map[string]build.ProjectBuilder, error) {
	cfg := a.Config
	if cfg == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	result := make(map[string]build.ProjectBuilder)
	var lpClient *lp.Client

	for _, proj := range cfg.Projects {
		if proj.Build == nil && proj.ArtifactType == "" {
			continue
		}

		if lpClient == nil {
			lpClient = NewLaunchpadClient(a.LaunchpadCredentialStore(), a.Logger)
			if lpClient == nil {
				a.Logger.Warn("skipping build projects (no LP auth configured)")
				return result, nil
			}
		}

		artifactType := proj.ArtifactType

		var builder port.RecipeBuilder
		var strategy build.ArtifactStrategy
		switch artifactType {
		case "rock":
			builder = lpadapter.NewRockBuilder(lpClient)
			strategy = &build.RockStrategy{}
		case "charm":
			builder = lpadapter.NewCharmBuilder(lpClient)
			strategy = &build.CharmStrategy{}
		case "snap":
			builder = lpadapter.NewSnapBuilder(lpClient, "", "")
			strategy = &build.SnapStrategy{}
		default:
			return nil, fmt.Errorf("unsupported artifact type %q for project %s", artifactType, proj.Name)
		}

		var owner string
		var artifacts []string
		var lpProject string
		var officialCodehosting bool
		if proj.Build != nil {
			owner = proj.Build.Owner
			artifacts = proj.Build.Artifacts
			lpProject = proj.Build.LPProject
			officialCodehosting = proj.Build.OfficialCodehosting
		}

		series := proj.Series
		if len(series) == 0 {
			series = cfg.Launchpad.Series
		}
		devFocus := proj.DevelopmentFocus
		if devFocus == "" {
			devFocus = cfg.Launchpad.DevelopmentFocus
		}

		result[proj.Name] = build.ProjectBuilder{
			Builder:             builder,
			Owner:               owner,
			Project:             proj.Code.Project,
			LPProject:           lpProject,
			Artifacts:           artifacts,
			Series:              series,
			DevFocus:            devFocus,
			OfficialCodehosting: officialCodehosting,
			Strategy:            strategy,
		}
	}

	return result, nil
}

// BuildService creates the build service with all required dependencies wired.
func (a *App) BuildService() (*build.Service, error) {
	builders, err := a.BuildRecipeBuilders()
	if err != nil {
		return nil, err
	}
	repoMgr, err := a.BuildRepoManager()
	if err != nil {
		return nil, err
	}
	return build.NewService(builders, repoMgr, a.Logger), nil
}

// BuildRepoManager creates a RepoManager backed by Launchpad.
func (a *App) BuildRepoManager() (port.RepoManager, error) {
	if a.Config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	lpClient := NewLaunchpadClient(a.LaunchpadCredentialStore(), a.Logger)
	if lpClient == nil {
		return nil, nil
	}

	return lpadapter.NewRepoManager(lpClient, a.Logger), nil
}

// GitClient returns a new git client for local repository operations.
func (a *App) GitClient() port.GitClient {
	return adaptergit.NewClient(a.Logger)
}
