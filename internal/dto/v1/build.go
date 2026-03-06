// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import "github.com/gboutry/sunbeam-watchtower/internal/port"

// BuildRecipeAction is the action determined for a recipe after assessment.
type BuildRecipeAction int

const (
	BuildActionCreateRecipe  BuildRecipeAction = iota // recipe doesn't exist yet
	BuildActionRequestBuilds                          // recipe exists but no builds
	BuildActionRetryFailed                            // some builds failed
	BuildActionMonitor                                // builds are active/pending
	BuildActionDownload                               // all builds succeeded
	BuildActionNoop                                   // nothing to do
)

// BuildTriggerResult holds the result of a trigger operation.
type BuildTriggerResult struct {
	Project       string              `json:"project" yaml:"project"`
	RecipeResults []BuildRecipeResult `json:"recipe_results" yaml:"recipe_results"`
}

// BuildRecipeResult holds the result of a single recipe action.
type BuildRecipeResult struct {
	Name         string             `json:"name" yaml:"name"`
	Action       BuildRecipeAction  `json:"action" yaml:"action"`
	BuildRequest *port.BuildRequest `json:"build_request,omitempty" yaml:"build_request,omitempty"`
	Builds       []port.Build       `json:"builds,omitempty" yaml:"builds,omitempty"`
	Error        error              `json:"-" yaml:"-"`
}
