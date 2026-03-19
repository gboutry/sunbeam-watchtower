// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/charmhub"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/snapstore"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/teamsync"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
	"gopkg.in/yaml.v3"
)

// lpTeamProvider adapts the LP client to the port.LaunchpadTeamProvider interface.
type lpTeamProvider struct {
	client         *lp.Client
	emailOverrides map[string]string // username → email
}

// Compile-time interface compliance check.
var _ port.LaunchpadTeamProvider = (*lpTeamProvider)(nil)

func (p *lpTeamProvider) GetTeamMembers(ctx context.Context, teamName string) ([]dto.TeamMember, error) {
	members, err := p.client.GetTeamMembersWithEmails(ctx, teamName)
	if err != nil {
		return nil, err
	}
	result := make([]dto.TeamMember, len(members))
	for i, m := range members {
		email := m.Email
		if email == "" {
			if override, ok := p.emailOverrides[m.Username]; ok {
				email = override
			}
		}
		result[i] = dto.TeamMember{Username: m.Username, Email: email}
	}
	return result, nil
}

// loadEmailOverrides reads a YAML file mapping LP usernames to email addresses.
// Returns nil map if path is empty or file doesn't exist.
func loadEmailOverrides(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading email overrides: %w", err)
	}
	var overrides map[string]string
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("parsing email overrides: %w", err)
	}
	return overrides, nil
}

// TeamSyncService returns the lazily initialized team sync service.
func (a *App) TeamSyncService() (*teamsync.Service, error) {
	a.teamSyncServiceOnce.Do(func() {
		if a.Config == nil {
			a.teamSyncServiceErr = fmt.Errorf("no configuration loaded")
			return
		}
		if a.Config.Collaborators == nil {
			a.teamSyncServiceErr = fmt.Errorf("collaborators not configured")
			return
		}

		lpClient := newLaunchpadClient(a.LaunchpadCredentialStore(), a.Logger, a.upstreamHTTPClient("launchpad", 30*time.Second))
		if lpClient == nil {
			a.teamSyncServiceErr = ErrLaunchpadAuthRequired
			return
		}

		emailOverrides, err := loadEmailOverrides(a.Config.Collaborators.EmailOverrides)
		if err != nil {
			a.teamSyncServiceErr = err
			return
		}

		teamProvider := &lpTeamProvider{client: lpClient, emailOverrides: emailOverrides}

		// Load store credentials for collaborator managers.
		snapStoreAuth := ""
		if rec, err := a.SnapStoreCredentialStore().Load(context.Background()); err == nil && rec != nil {
			snapStoreAuth = rec.Macaroon
		}
		charmhubAuth := ""
		if rec, err := a.CharmhubCredentialStore().Load(context.Background()); err == nil && rec != nil {
			charmhubAuth = rec.Macaroon
		}

		stores := map[dto.ArtifactType]port.StoreCollaboratorManager{
			dto.ArtifactSnap:  snapstore.NewCollaboratorManager(snapStoreAuth),
			dto.ArtifactCharm: charmhub.NewCollaboratorManager(charmhubAuth),
		}

		a.teamSyncService = teamsync.NewService(teamProvider, stores, a.Logger)
	})
	return a.teamSyncService, a.teamSyncServiceErr
}
