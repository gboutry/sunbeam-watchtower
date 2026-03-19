// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/charmhub"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/snapstore"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/teamsync"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

// lpTeamProvider adapts the LP client to the port.LaunchpadTeamProvider interface.
type lpTeamProvider struct {
	client *lp.Client
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
		result[i] = dto.TeamMember{Username: m.Username, Email: m.Email}
	}
	return result, nil
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

		teamProvider := &lpTeamProvider{client: lpClient}

		// Build store collaborator managers.
		// Auth tokens are empty for now — Task 8 will add real store auth.
		stores := map[dto.ArtifactType]port.StoreCollaboratorManager{
			dto.ArtifactSnap:  snapstore.NewCollaboratorManager(""),
			dto.ArtifactCharm: charmhub.NewCollaboratorManager(""),
		}

		a.teamSyncService = teamsync.NewService(teamProvider, stores, a.Logger)
	})
	return a.teamSyncService, a.teamSyncServiceErr
}
