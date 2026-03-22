// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package teamsync

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// Service coordinates team collaborator synchronization.
type Service struct {
	teamProvider port.LaunchpadTeamProvider
	stores       map[dto.ArtifactType]port.StoreCollaboratorManager
	logger       *slog.Logger
}

// NewService creates a team sync service.
func NewService(
	teamProvider port.LaunchpadTeamProvider,
	stores map[dto.ArtifactType]port.StoreCollaboratorManager,
	logger *slog.Logger,
) *Service {
	return &Service{teamProvider: teamProvider, stores: stores, logger: logger}
}

// Sync compares LP team members against store collaborators for each target.
func (s *Service) Sync(ctx context.Context, teamName string, targets []dto.SyncTarget, dryRun bool) (*dto.TeamSyncResult, error) {
	members, err := s.teamProvider.GetTeamMembers(ctx, teamName)
	if err != nil {
		return nil, fmt.Errorf("fetching team %s members: %w", teamName, err)
	}

	result := &dto.TeamSyncResult{}

	// Separate members with and without emails.
	var memberEmails []string
	for _, m := range members {
		if m.Email == "" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("member %q has hidden email — cannot match to store collaborators", m.Username))
			continue
		}
		memberEmails = append(memberEmails, m.Email)
	}
	teamSet := toSet(memberEmails)

	for _, target := range targets {
		art := s.syncTarget(ctx, target, teamSet, dryRun)
		result.Artifacts = append(result.Artifacts, art)
	}
	return result, nil
}

func (s *Service) syncTarget(ctx context.Context, target dto.SyncTarget, teamEmails map[string]bool, dryRun bool) dto.ArtifactSyncResult {
	art := dto.ArtifactSyncResult{
		Project:      target.Project,
		ArtifactType: target.ArtifactType,
		StoreName:    target.StoreName,
	}

	store, ok := s.stores[target.ArtifactType]
	if !ok {
		art.Error = fmt.Sprintf("no store manager for artifact type %s", target.ArtifactType)
		return art
	}

	collabs, err := store.ListCollaborators(ctx, target.StoreName)
	if err != nil {
		art.Error = fmt.Sprintf("listing collaborators for %s: %v", target.StoreName, err)
		return art
	}

	collabPending := make(map[string]bool)
	allCollabEmails := make(map[string]bool)
	for _, c := range collabs {
		email := strings.ToLower(c.Email)
		allCollabEmails[email] = true
		if c.Status == "pending" {
			collabPending[email] = true
		}
	}

	// Missing: in team, not in store.
	for email := range teamEmails {
		lower := strings.ToLower(email)
		if allCollabEmails[lower] {
			if collabPending[lower] {
				art.Pending = append(art.Pending, email)
			}
			continue
		}
		art.Invited = append(art.Invited, email)
		if !dryRun {
			if err := store.InviteCollaborator(ctx, target.StoreName, email); err != nil {
				s.logger.Warn("failed to invite collaborator", "store", target.StoreName, "email", email, "error", err)
			}
		}
	}

	// Extra: in store, not in team.
	for _, c := range collabs {
		lower := strings.ToLower(c.Email)
		if !teamEmails[lower] {
			art.Extra = append(art.Extra, c.Email)
		}
	}

	if len(art.Invited) == 0 && len(art.Extra) == 0 && len(art.Pending) == 0 {
		art.AlreadySync = true
	}

	return art
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[strings.ToLower(s)] = true
	}
	return m
}
