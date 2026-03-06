// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
)

// ProjectBuilder groups a RecipeBuilder with its project-level metadata.
type ProjectBuilder struct {
	Builder  port.RecipeBuilder
	Owner    string
	Project  string
	Recipes  []string
	Strategy ArtifactStrategy
}
