// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"os"
	"path/filepath"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"

	"gopkg.in/yaml.v3"
)

// SnapStrategy implements ArtifactStrategy for snap artifacts.
type SnapStrategy struct{}

func (s *SnapStrategy) ArtifactType() dto.ArtifactType { return dto.ArtifactSnap }
func (s *SnapStrategy) MetadataFileName() string       { return "snapcraft.yaml" }
func (s *SnapStrategy) BuildPath(name string) string   { return "" }

// DiscoverRecipes looks for snapcraft.yaml at the repo root or snap/ subdirectory.
func (s *SnapStrategy) DiscoverRecipes(repoPath string) ([]string, error) {
	// snap/snapcraft.yaml
	snapDir := filepath.Join(repoPath, "snap", s.MetadataFileName())
	if _, err := os.Stat(snapDir); err == nil {
		return []string{filepath.Base(repoPath)}, nil
	}
	// snapcraft.yaml at root
	rootMeta := filepath.Join(repoPath, s.MetadataFileName())
	if _, err := os.Stat(rootMeta); err == nil {
		return []string{filepath.Base(repoPath)}, nil
	}
	return nil, nil
}

func (s *SnapStrategy) TempRecipeName(name, sha, prefix string) string {
	short := sha
	if len(sha) > 8 {
		short = sha[:8]
	}
	return prefix + "-" + short + "-" + name
}

type snapcraftYAML struct {
	Architectures []snapArchitecture            `yaml:"architectures"`
	Platforms     map[string]*snapPlatformEntry `yaml:"platforms"`
}

type snapArchitecture struct {
	BuildOn  yaml.Node `yaml:"build-on"`
	BuildFor yaml.Node `yaml:"build-for"`
}

type snapPlatformEntry struct {
	BuildOn  yaml.Node `yaml:"build-on"`
	BuildFor yaml.Node `yaml:"build-for"`
}

func (s *SnapStrategy) ParsePlatforms(content []byte) ([]string, error) {
	var sc snapcraftYAML
	if err := yaml.Unmarshal(content, &sc); err != nil {
		return nil, err
	}

	// New syntax: platforms
	if len(sc.Platforms) > 0 {
		seen := make(map[string]struct{})
		for key, val := range sc.Platforms {
			if val == nil || (val.BuildFor.Kind == 0 && val.BuildOn.Kind == 0) {
				seen[key] = struct{}{}
				continue
			}
			if val.BuildFor.Kind != 0 {
				archs, err := decodeStringOrSlice(&val.BuildFor)
				if err != nil {
					return nil, err
				}
				for _, a := range archs {
					seen[a] = struct{}{}
				}
			}
		}
		result := make([]string, 0, len(seen))
		for k := range seen {
			result = append(result, k)
		}
		return result, nil
	}

	// Old syntax: architectures
	if len(sc.Architectures) > 0 {
		seen := make(map[string]struct{})
		for _, arch := range sc.Architectures {
			if arch.BuildFor.Kind != 0 {
				archs, err := decodeStringOrSlice(&arch.BuildFor)
				if err != nil {
					return nil, err
				}
				for _, a := range archs {
					seen[a] = struct{}{}
				}
			} else if arch.BuildOn.Kind != 0 {
				archs, err := decodeStringOrSlice(&arch.BuildOn)
				if err != nil {
					return nil, err
				}
				for _, a := range archs {
					seen[a] = struct{}{}
				}
			}
		}
		if len(seen) > 0 {
			result := make([]string, 0, len(seen))
			for k := range seen {
				result = append(result, k)
			}
			return result, nil
		}
	}

	return []string{"amd64"}, nil
}
