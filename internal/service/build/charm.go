// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"gopkg.in/yaml.v3"
)

// CharmStrategy implements ArtifactStrategy for charm artifacts.
type CharmStrategy struct{}

func (s *CharmStrategy) ArtifactType() port.ArtifactType { return port.ArtifactCharm }
func (s *CharmStrategy) MetadataFileName() string        { return "charmcraft.yaml" }
func (s *CharmStrategy) BuildPath(name string) string    { return "charms/" + name }

// DiscoverRecipes scans for charmcraft.yaml in charms/*/ subdirectories
// or at the repo root (single-charm repo).
func (s *CharmStrategy) DiscoverRecipes(repoPath string) ([]string, error) {
	// Multi-charm repo: charms/*/charmcraft.yaml
	pattern := filepath.Join(repoPath, "charms", "*", s.MetadataFileName())
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, filepath.Base(filepath.Dir(m)))
		}
		sort.Strings(names)
		return names, nil
	}
	// Single-charm repo: charmcraft.yaml at root
	rootMeta := filepath.Join(repoPath, s.MetadataFileName())
	if _, err := os.Stat(rootMeta); err == nil {
		return []string{filepath.Base(repoPath)}, nil
	}
	return nil, nil
}

func (s *CharmStrategy) TempRecipeName(name, sha, prefix string) string {
	short := sha
	if len(sha) > 8 {
		short = sha[:8]
	}
	return prefix + "-" + short + "-" + name
}

type charmcraftBase struct {
	Name          string   `yaml:"name"`
	Channel       string   `yaml:"channel"`
	Architectures []string `yaml:"architectures"`
}

type charmcraftBaseEntry struct {
	BuildOn []charmcraftBase `yaml:"build-on"`
	RunOn   []charmcraftBase `yaml:"run-on"`
}

type charmcraftYAML struct {
	Platforms map[string]*rockcraftPlatformEntry `yaml:"platforms"`
	Bases     []charmcraftBaseEntry              `yaml:"bases"`
}

func (s *CharmStrategy) ParsePlatforms(content []byte) ([]string, error) {
	var cc charmcraftYAML
	if err := yaml.Unmarshal(content, &cc); err != nil {
		return nil, err
	}

	// New syntax: platforms (same as rocks)
	if len(cc.Platforms) > 0 {
		return parsePlatformsMap(cc.Platforms)
	}

	// Old syntax: bases
	if len(cc.Bases) > 0 {
		return parseCharmBases(cc.Bases)
	}

	return []string{"amd64"}, nil
}

func parsePlatformsMap(platforms map[string]*rockcraftPlatformEntry) ([]string, error) {
	seen := make(map[string]struct{})
	for key, val := range platforms {
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

func parseCharmBases(bases []charmcraftBaseEntry) ([]string, error) {
	seen := make(map[string]struct{})
	for _, entry := range bases {
		sources := entry.RunOn
		if len(sources) == 0 {
			sources = entry.BuildOn
		}
		for _, base := range sources {
			for _, arch := range base.Architectures {
				seen[arch] = struct{}{}
			}
		}
	}
	if len(seen) == 0 {
		return []string{"amd64"}, nil
	}
	result := make([]string, 0, len(seen))
	for k := range seen {
		result = append(result, k)
	}
	return result, nil
}
