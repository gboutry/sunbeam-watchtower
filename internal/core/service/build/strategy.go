// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"

// ArtifactStrategy encapsulates artifact-type-specific logic.
type ArtifactStrategy interface {
	// ArtifactType returns the type of artifact this strategy handles.
	ArtifactType() dto.ArtifactType

	// MetadataFileName returns the metadata file name (e.g. "rockcraft.yaml").
	MetadataFileName() string

	// BuildPath returns the LP build path for an artifact name.
	// E.g. for rocks: "rocks/keystone", for charms: "charms/mysql-k8s"
	BuildPath(artifactName string) string

	// ParsePlatforms parses the metadata file content and returns expected platforms.
	// Returns a set of architecture strings like {"amd64", "arm64"}.
	ParsePlatforms(metadataContent []byte) ([]string, error)

	// TempRecipeName generates a temporary recipe name.
	// Format: {prefix}-{shortSHA}-{artifactName}
	TempRecipeName(artifactName, commitSHA, prefix string) string

	// DiscoverRecipes finds artifact names by scanning a local repo directory
	// for metadata files. Returns the list of discovered recipe names.
	DiscoverRecipes(repoPath string) ([]string, error)

	// OfficialRecipeName returns the recipe name for official/remote builds.
	// For the development focus series, returns just the artifact name.
	// For other series, returns artifactName-series.
	OfficialRecipeName(artifactName, series, devFocus string) string

	// BranchForSeries returns the git branch for a given series.
	// For the development focus, returns the repo's default branch (e.g. "main").
	// For other series, returns "stable/<series>".
	BranchForSeries(series, devFocus, defaultBranch string) string
}
