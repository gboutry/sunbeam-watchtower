package cli

import (
	"context"
	"fmt"

	"github.com/andygrunwald/go-gerrit"
	"github.com/google/go-github/v68/github"

	lpadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/launchpad"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"github.com/gboutry/sunbeam-watchtower/internal/service/bug"
	"github.com/gboutry/sunbeam-watchtower/internal/service/build"
	"github.com/gboutry/sunbeam-watchtower/internal/service/review"
)

// buildForgeClients creates forge clients from config, caching one per forge type/host.
func buildForgeClients(opts *Options) (map[string]review.ProjectForge, error) {
	cfg := opts.Config
	if cfg == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	result := make(map[string]review.ProjectForge, len(cfg.Projects))

	// Cache one client per forge type/host to avoid creating duplicates.
	var ghClient *forge.GitHubForge
	gerritClients := make(map[string]*forge.GerritForge)
	var lpClient *forge.LaunchpadForge

	for _, proj := range cfg.Projects {
		var pf review.ProjectForge
		code := proj.Code

		switch code.Forge {
		case "github":
			if ghClient == nil {
				ghClient = forge.NewGitHubForge(github.NewClient(nil))
			}
			pf = review.ProjectForge{
				Forge:     ghClient,
				ProjectID: code.Owner + "/" + code.Project,
			}

		case "gerrit":
			gc, ok := gerritClients[code.Host]
			if !ok {
				client, err := gerrit.NewClient(context.Background(), code.Host, nil)
				if err != nil {
					return nil, fmt.Errorf("creating Gerrit client for %s: %w", code.Host, err)
				}
				gc = forge.NewGerritForge(client, code.Host)
				gerritClients[code.Host] = gc
			}
			pf = review.ProjectForge{
				Forge:     gc,
				ProjectID: code.Project,
			}

		case "launchpad":
			if lpClient == nil {
				lpClient = newLaunchpadForge(cfg.Launchpad, opts)
			}
			if lpClient == nil {
				opts.Logger.Warn("skipping Launchpad project (no auth configured)", "project", proj.Name)
				continue
			}
			pf = review.ProjectForge{
				Forge:     lpClient,
				ProjectID: code.Project,
			}

		default:
			return nil, fmt.Errorf("unknown forge type %q for project %s", code.Forge, proj.Name)
		}

		result[proj.Name] = pf
	}

	return result, nil
}

// buildBugTrackers creates bug tracker clients from config, deduplicating by (forge, project).
func buildBugTrackers(opts *Options) (map[string]bug.ProjectBugTracker, map[string][]string, error) {
	cfg := opts.Config
	if cfg == nil {
		return nil, nil, fmt.Errorf("no configuration loaded")
	}

	trackers := make(map[string]bug.ProjectBugTracker)
	projectMap := make(map[string][]string)

	var lpBugTracker *forge.LaunchpadBugTracker

	for _, proj := range cfg.Projects {
		for _, b := range proj.Bugs {
			switch b.Forge {
			case "launchpad":
				if lpBugTracker == nil {
					lpClient := newLaunchpadClient(cfg.Launchpad, opts)
					if lpClient == nil {
						opts.Logger.Warn("skipping Launchpad bug tracker (no auth configured)", "project", proj.Name)
						continue
					}
					lpBugTracker = forge.NewLaunchpadBugTracker(lpClient)
				}

				key := "launchpad:" + b.Project
				if _, ok := trackers[key]; !ok {
					trackers[key] = bug.ProjectBugTracker{
						Tracker:   lpBugTracker,
						ProjectID: b.Project,
					}
				}
				projectMap[key] = append(projectMap[key], proj.Name)

			default:
				return nil, nil, fmt.Errorf("unsupported bug tracker forge %q for project %s", b.Forge, proj.Name)
			}
		}
	}

	return trackers, projectMap, nil
}

// newLaunchpadClient creates an LP client with credentials from env/file cache.
// Returns nil if no credentials are available.
func newLaunchpadClient(lpCfg config.LaunchpadConfig, opts *Options) *lp.Client {
	_ = lpCfg
	creds, err := lp.LoadCredentials()
	if err != nil {
		opts.Logger.Warn("failed to load LP credentials", "error", err)
		return nil
	}
	if creds == nil {
		return nil
	}
	return lp.NewClient(creds, opts.Logger)
}

func newLaunchpadForge(lpCfg config.LaunchpadConfig, opts *Options) *forge.LaunchpadForge {
	client := newLaunchpadClient(lpCfg, opts)
	if client == nil {
		return nil
	}
	return forge.NewLaunchpadForge(client)
}

// buildRecipeBuilders creates per-project RecipeBuilder instances from config.
func buildRecipeBuilders(opts *Options) (map[string]build.ProjectBuilder, error) {
	cfg := opts.Config
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
			lpClient = newLaunchpadClient(cfg.Launchpad, opts)
			if lpClient == nil {
				opts.Logger.Warn("skipping build projects (no LP auth configured)")
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
		var recipes []string
		if proj.Build != nil {
			owner = proj.Build.Owner
			recipes = proj.Build.Recipes
		}

		result[proj.Name] = build.ProjectBuilder{
			Builder:  builder,
			Owner:    owner,
			Project:  proj.Code.Project,
			Recipes:  recipes,
			Strategy: strategy,
		}
	}

	return result, nil
}

// buildRepoManager creates a RepoManager backed by Launchpad.
func buildRepoManager(opts *Options) (port.RepoManager, error) {
	if opts.Config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	lpClient := newLaunchpadClient(opts.Config.Launchpad, opts)
	if lpClient == nil {
		return nil, nil
	}

	return lpadapter.NewRepoManager(lpClient, opts.Logger), nil
}
