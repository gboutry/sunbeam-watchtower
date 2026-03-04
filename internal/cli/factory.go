package cli

import (
	"context"
	"fmt"

	"github.com/andygrunwald/go-gerrit"
	"github.com/google/go-github/v68/github"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
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

func newLaunchpadForge(lpCfg config.LaunchpadConfig, opts *Options) *forge.LaunchpadForge {
	// For now, create an unauthenticated LP client.
	// Full OAuth flow will be wired in a later phase.
	_ = lpCfg
	client := lp.NewClient(&lp.Credentials{ConsumerKey: "sunbeam-watchtower"}, opts.Logger)
	return forge.NewLaunchpadForge(client)
}
