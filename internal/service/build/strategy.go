// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import "github.com/gboutry/sunbeam-watchtower/internal/port"

// ArtifactStrategy encapsulates artifact-type-specific logic.
type ArtifactStrategy interface {
	// ArtifactType returns the type of artifact this strategy handles.
	ArtifactType() port.ArtifactType

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
}
