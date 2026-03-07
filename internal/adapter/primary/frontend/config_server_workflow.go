// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ConfigServerWorkflow exposes reusable server-side config workflows for the HTTP API.
type ConfigServerWorkflow struct {
	application *app.App
}

// NewConfigServerWorkflow creates a server-side config workflow.
func NewConfigServerWorkflow(application *app.App) *ConfigServerWorkflow {
	return &ConfigServerWorkflow{application: application}
}

// Show returns the loaded configuration as a public DTO.
func (w *ConfigServerWorkflow) Show(context.Context) (*dto.Config, error) {
	if w.application == nil || w.application.Config == nil {
		return nil, errors.New("no configuration loaded")
	}
	return configToDTO(w.application.Config), nil
}

func configToDTO(cfg *config.Config) *dto.Config {
	if cfg == nil {
		return nil
	}

	out := &dto.Config{
		Launchpad: dto.LaunchpadConfig{
			DefaultOwner:     cfg.Launchpad.DefaultOwner,
			UseKeyring:       cfg.Launchpad.UseKeyring,
			Series:           append([]string(nil), cfg.Launchpad.Series...),
			DevelopmentFocus: cfg.Launchpad.DevelopmentFocus,
		},
		GitHub: dto.GitHubConfig{
			UseKeyring: cfg.GitHub.UseKeyring,
		},
		Build: dto.BuildConfig{
			DefaultPrefix:  cfg.Build.DefaultPrefix,
			TimeoutMinutes: cfg.Build.TimeoutMinutes,
			ArtifactsDir:   cfg.Build.ArtifactsDir,
		},
	}

	out.Gerrit.Hosts = make([]dto.GerritHost, len(cfg.Gerrit.Hosts))
	for i, host := range cfg.Gerrit.Hosts {
		out.Gerrit.Hosts[i] = dto.GerritHost{URL: host.URL}
	}

	out.Projects = make([]dto.ProjectConfig, len(cfg.Projects))
	for i, project := range cfg.Projects {
		outProject := dto.ProjectConfig{
			Name:             project.Name,
			ArtifactType:     project.ArtifactType,
			Series:           append([]string(nil), project.Series...),
			DevelopmentFocus: project.DevelopmentFocus,
			Code: dto.CodeConfig{
				Forge:   project.Code.Forge,
				Owner:   project.Code.Owner,
				Host:    project.Code.Host,
				Project: project.Code.Project,
				GitURL:  project.Code.GitURL,
			},
		}

		if project.Build != nil {
			outProject.Build = &dto.ProjectBuildConfig{
				Owner:          project.Build.Owner,
				Artifacts:      append([]string(nil), project.Build.Artifacts...),
				PrepareCommand: project.Build.PrepareCommand,
			}
		}

		outProject.Bugs = make([]dto.BugTrackerConfig, len(project.Bugs))
		for j, bug := range project.Bugs {
			outProject.Bugs[j] = dto.BugTrackerConfig{
				Forge:   bug.Forge,
				Owner:   bug.Owner,
				Host:    bug.Host,
				Project: bug.Project,
			}
		}
		outProject.Publications = make([]dto.ProjectPublicationConfig, len(project.Publications))
		for j, publication := range project.Publications {
			outProject.Publications[j] = dto.ProjectPublicationConfig{
				Name:      publication.Name,
				Type:      publication.Type,
				Tracks:    append([]string(nil), publication.Tracks...),
				Resources: append([]string(nil), publication.Resources...),
			}
		}

		out.Projects[i] = outProject
	}

	if len(cfg.Packages.Distros) > 0 {
		out.Packages.Distros = make(map[string]dto.DistroConfig, len(cfg.Packages.Distros))
		for name, distro := range cfg.Packages.Distros {
			outDistro := dto.DistroConfig{
				Mirror:     distro.Mirror,
				Components: append([]string(nil), distro.Components...),
				Releases:   make(map[string]dto.ReleaseConfig, len(distro.Releases)),
			}
			if distro.Excuses != nil {
				outDistro.Excuses = &dto.ExcusesConfig{
					Provider: distro.Excuses.Provider,
					URL:      distro.Excuses.URL,
					TeamURL:  distro.Excuses.TeamURL,
				}
			}
			for releaseName, release := range distro.Releases {
				outRelease := dto.ReleaseConfig{
					Suites:    append([]string(nil), release.Suites...),
					Backports: make(map[string]dto.BackportConfig, len(release.Backports)),
				}
				if len(release.Backports) > 0 {
					for backportName, backport := range release.Backports {
						outBackport := dto.BackportConfig{
							ParentRelease: backport.ParentRelease,
							Sources:       make([]dto.DistroSourceConfig, len(backport.Sources)),
						}
						for i, source := range backport.Sources {
							outBackport.Sources[i] = dto.DistroSourceConfig{
								Mirror:     source.Mirror,
								Suites:     append([]string(nil), source.Suites...),
								Components: append([]string(nil), source.Components...),
							}
						}
						outRelease.Backports[backportName] = outBackport
					}
				}
				outDistro.Releases[releaseName] = outRelease
			}
			out.Packages.Distros[name] = outDistro
		}
	}

	if len(cfg.Packages.Sets) > 0 {
		out.Packages.Sets = make(map[string][]string, len(cfg.Packages.Sets))
		for setName, packages := range cfg.Packages.Sets {
			out.Packages.Sets[setName] = append([]string(nil), packages...)
		}
	}

	if cfg.Packages.Upstream != nil {
		out.Packages.Upstream = &dto.UpstreamConfig{
			Provider:         cfg.Packages.Upstream.Provider,
			ReleasesRepo:     cfg.Packages.Upstream.ReleasesRepo,
			RequirementsRepo: cfg.Packages.Upstream.RequirementsRepo,
		}
	}

	return out
}
