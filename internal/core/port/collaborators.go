// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// StoreCollaboratorManager manages collaborators on a backing store artifact.
type StoreCollaboratorManager interface {
	ListCollaborators(ctx context.Context, storeName string) ([]dto.StoreCollaborator, error)
	InviteCollaborator(ctx context.Context, storeName string, email string) error
}

// LaunchpadTeamProvider fetches members of a Launchpad team.
type LaunchpadTeamProvider interface {
	GetTeamMembers(ctx context.Context, teamName string) ([]dto.TeamMember, error)
}
