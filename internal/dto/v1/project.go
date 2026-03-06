// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

// ProjectSyncActionType describes the kind of sync action.
type ProjectSyncActionType string

// SchemaName returns a unique name for Huma OpenAPI schema registration.
func (ProjectSyncActionType) SchemaName() string { return "ProjectSyncActionType" }

const (
	ProjectSyncActionCreateSeries      ProjectSyncActionType = "create_series"
	ProjectSyncActionSetDevFocus       ProjectSyncActionType = "set_dev_focus"
	ProjectSyncActionDevFocusUnchanged ProjectSyncActionType = "dev_focus_unchanged"
)

// ProjectSyncAction represents a single action taken (or planned) during sync.
type ProjectSyncAction struct {
	Project    string                `json:"project" yaml:"project"`
	Series     string                `json:"series" yaml:"series"`
	ActionType ProjectSyncActionType `json:"action_type" yaml:"action_type"`
	OldValue   string                `json:"old_value,omitempty" yaml:"old_value,omitempty"`
	NewValue   string                `json:"new_value,omitempty" yaml:"new_value,omitempty"`
}

// SchemaName returns a unique name for Huma OpenAPI schema registration.
func (ProjectSyncAction) SchemaName() string { return "ProjectSyncAction" }

// ProjectSyncResult holds the outcome of a sync operation.
type ProjectSyncResult struct {
	Actions []ProjectSyncAction `json:"actions" yaml:"actions"`
	Errors  []error             `json:"-" yaml:"-"`
}
