// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

// TeamMember represents a member of a Launchpad team.
type TeamMember struct {
	Username string `json:"username" yaml:"username"`
	Email    string `json:"email,omitempty" yaml:"email,omitempty"` // empty if hidden
}

// StoreCollaborator represents a collaborator on a store artifact.
type StoreCollaborator struct {
	Username    string `json:"username" yaml:"username"`
	Email       string `json:"email" yaml:"email"`
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Status      string `json:"status" yaml:"status"` // "accepted", "pending", "expired"
}

// SyncTarget identifies one store artifact to check for collaborators.
type SyncTarget struct {
	Project      string       `json:"project" yaml:"project"`
	ArtifactType ArtifactType `json:"artifact_type" yaml:"artifact_type"`
	StoreName    string       `json:"store_name" yaml:"store_name"`
}

// TeamSyncRequest holds parameters for a team collaborator sync.
type TeamSyncRequest struct {
	Projects []string `json:"projects,omitempty" yaml:"projects,omitempty"`
	DryRun   bool     `json:"dry_run" yaml:"dry_run"`
}

// TeamSyncResult holds the outcome of a team collaborator sync.
type TeamSyncResult struct {
	Artifacts []ArtifactSyncResult `json:"artifacts" yaml:"artifacts"`
	Warnings  []string             `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// ArtifactSyncResult holds the sync outcome for one store artifact.
type ArtifactSyncResult struct {
	Project      string       `json:"project" yaml:"project"`
	ArtifactType ArtifactType `json:"artifact_type" yaml:"artifact_type"`
	StoreName    string       `json:"store_name" yaml:"store_name"`
	Invited      []string     `json:"invited,omitempty" yaml:"invited,omitempty"`
	Extra        []string     `json:"extra,omitempty" yaml:"extra,omitempty"`
	Pending      []string     `json:"pending,omitempty" yaml:"pending,omitempty"`
	AlreadySync  bool         `json:"already_sync" yaml:"already_sync"`
	Error        string       `json:"error,omitempty" yaml:"error,omitempty"`
}
