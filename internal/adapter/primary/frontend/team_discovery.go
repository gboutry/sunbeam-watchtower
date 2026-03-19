// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"os"
	"path/filepath"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	"gopkg.in/yaml.v3"
)

type artifactManifest struct {
	Name string `yaml:"name"`
}

// DiscoverTargets scans a worktree directory for artifact manifest files
// and returns the store names found.
func DiscoverTargets(worktree string, artifactType dto.ArtifactType) ([]string, error) {
	var patterns []string
	switch artifactType {
	case dto.ArtifactSnap:
		patterns = []string{
			filepath.Join(worktree, "snaps", "*", "snap", "snapcraft.yaml"),
			filepath.Join(worktree, "snaps", "*", "*", "snap", "snapcraft.yaml"),
			filepath.Join(worktree, "snap", "snapcraft.yaml"),
		}
	case dto.ArtifactCharm:
		patterns = []string{
			filepath.Join(worktree, "charms", "*", "charmcraft.yaml"),
			filepath.Join(worktree, "charms", "*", "*", "charmcraft.yaml"),
			filepath.Join(worktree, "charmcraft.yaml"),
		}
	default:
		return nil, nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			name, err := parseManifestName(path)
			if err != nil || name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	return names, nil
}

func parseManifestName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var m artifactManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return "", err
	}
	return m.Name, nil
}
