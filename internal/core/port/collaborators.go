// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"
	"errors"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ErrCollaboratorsUnsupported signals that a store backend does not support
// per-artifact collaborator management through a public API. Adapters return
// this sentinel (wrapped with an operator-facing message) when there is no
// safe programmatic path and operators must use the store's web UI instead.
var ErrCollaboratorsUnsupported = errors.New("store collaborator management is unsupported")

// StoreCollaboratorManager manages collaborators on a backing store artifact.
type StoreCollaboratorManager interface {
	ListCollaborators(ctx context.Context, storeName string) ([]dto.StoreCollaborator, error)
	InviteCollaborator(ctx context.Context, storeName string, email string) error
}

// LaunchpadTeamProvider fetches members of a Launchpad team.
type LaunchpadTeamProvider interface {
	GetTeamMembers(ctx context.Context, teamName string) ([]dto.TeamMember, error)
}
