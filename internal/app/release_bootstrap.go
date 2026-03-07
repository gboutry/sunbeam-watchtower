// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/charmhub"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/gitcache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/releasecache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/snapstore"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	"gopkg.in/yaml.v3"
)

// ReleaseCache returns a lazy-initialized release publication cache singleton.
func (a *App) ReleaseCache() (*releasecache.Cache, error) {
	a.releaseCacheOnce.Do(func() {
		path, err := cacheSubdir("releases")
		if err != nil {
			a.releaseCacheErr = err
			return
		}
		a.releaseCache, a.releaseCacheErr = releasecache.NewCache(path)
	})
	return a.releaseCache, a.releaseCacheErr
}

// TrackedReleases returns the discovered published snap/charm artifacts to track.
func (a *App) TrackedReleases(ctx context.Context) ([]dto.TrackedPublication, error) {
	publications, _, err := a.discoverTrackedReleases(ctx)
	return publications, err
}

func (a *App) discoverTrackedReleases(ctx context.Context) ([]dto.TrackedPublication, []string, error) {
	if a == nil || a.Config == nil {
		return nil, nil, nil
	}
	cache, err := a.GitCache()
	if err != nil {
		return nil, nil, err
	}

	byKey := make(map[string]dto.TrackedPublication)
	var warnings []string
	for _, project := range a.Config.Projects {
		artifactType, err := dto.ParseArtifactType(project.ArtifactType)
		if err != nil || (artifactType != dto.ArtifactSnap && artifactType != dto.ArtifactCharm) {
			warnings = append(warnings, fmt.Sprintf("%s: skipped (artifact_type must be snap or charm)", project.Name))
			continue
		}
		cloneURL, err := project.Code.CloneURL()
		if err != nil {
			return nil, nil, fmt.Errorf("project %s: resolving clone URL: %w", project.Name, err)
		}
		repoPath, err := cache.EnsureRepo(ctx, cloneURL, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("project %s: caching repo: %w", project.Name, err)
		}

		baseTracks, branches, err := resolveReleaseTracking(a.Config, project)
		if err != nil {
			return nil, nil, fmt.Errorf("project %s: %w", project.Name, err)
		}
		if len(baseTracks) == 0 && len(branches) == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: skipped (no series, release.tracks, or release.branches configured)", project.Name))
			continue
		}

		switch artifactType {
		case dto.ArtifactSnap:
			snapName, err := discoverSnapName(repoPath)
			if err != nil {
				return nil, nil, fmt.Errorf("project %s: %w", project.Name, err)
			}
			key := publicationKey(project.Name, artifactType, snapName)
			byKey[key] = dto.TrackedPublication{
				Project:      project.Name,
				Name:         snapName,
				ArtifactType: artifactType,
				Tracks:       append([]string(nil), baseTracks...),
				Branches:     append([]dto.TrackedReleaseBranch(nil), branches...),
			}
		case dto.ArtifactCharm:
			artifacts, err := discoverCharmPublications(repoPath)
			if err != nil {
				return nil, nil, fmt.Errorf("project %s: %w", project.Name, err)
			}
			for _, artifact := range artifacts {
				key := publicationKey(project.Name, artifactType, artifact.Name)
				byKey[key] = dto.TrackedPublication{
					Project:      project.Name,
					Name:         artifact.Name,
					ArtifactType: artifactType,
					Tracks:       append([]string(nil), baseTracks...),
					Resources:    append([]string(nil), artifact.Resources...),
					Branches:     append([]dto.TrackedReleaseBranch(nil), branches...),
				}
			}
		}
	}

	publications := make([]dto.TrackedPublication, 0, len(byKey))
	for _, publication := range byKey {
		publications = append(publications, publication)
	}
	sort.Slice(publications, func(i, j int) bool {
		if publications[i].Project == publications[j].Project {
			if publications[i].ArtifactType == publications[j].ArtifactType {
				return publications[i].Name < publications[j].Name
			}
			return publications[i].ArtifactType.String() < publications[j].ArtifactType.String()
		}
		return publications[i].Project < publications[j].Project
	})
	return publications, warnings, nil
}

// BuildReleaseSources creates the release sources supported by the current process.
func (a *App) BuildReleaseSources() map[dto.ArtifactType]port.ReleaseSource {
	client := &http.Client{Timeout: 30 * time.Second}
	return map[dto.ArtifactType]port.ReleaseSource{
		dto.ArtifactSnap:  snapstore.NewSource(client),
		dto.ArtifactCharm: charmhub.NewSource(client),
	}
}

type charmcraftMetadata struct {
	Name      string              `yaml:"name"`
	Resources map[string]struct{} `yaml:"resources"`
}

type snapcraftMetadata struct {
	Name string `yaml:"name"`
}

type discoveredCharm struct {
	Name      string
	Resources []string
}

// ReleaseDiscoveryResult captures the tracked artifacts selected for release sync.
type ReleaseDiscoveryResult struct {
	Publications []dto.TrackedPublication
	Warnings     []string
}

// DiscoverTrackedReleases resolves the tracked release inventory and skip reasons.
func (a *App) DiscoverTrackedReleases(ctx context.Context) (*ReleaseDiscoveryResult, error) {
	publications, warnings, err := a.discoverTrackedReleases(ctx)
	if err != nil {
		return nil, err
	}
	return &ReleaseDiscoveryResult{Publications: publications, Warnings: warnings}, nil
}

func resolveReleaseTracking(cfg *config.Config, project config.ProjectConfig) ([]string, []dto.TrackedReleaseBranch, error) {
	var tracks []string
	if project.Release != nil && len(project.Release.Tracks) > 0 {
		tracks = append([]string(nil), project.Release.Tracks...)
	} else {
		tracks = effectiveProjectSeries(cfg, project)
		if project.Release != nil && len(project.Release.TrackMap) > 0 {
			for idx, series := range tracks {
				if mapped, ok := project.Release.TrackMap[series]; ok {
					tracks[idx] = mapped
				}
			}
		}
	}
	if err := dto.ValidateReleaseTracks(tracks); err != nil {
		return nil, nil, err
	}
	tracks = dedupeStrings(tracks)

	var branches []dto.TrackedReleaseBranch
	if project.Release != nil {
		for _, branch := range project.Release.Branches {
			track := branch.Track
			if track == "" {
				track = branch.Series
				if mapped, ok := project.Release.TrackMap[track]; ok {
					track = mapped
				}
			}
			risks := dto.KnownReleaseRisks()
			if len(branch.Risks) > 0 {
				risks = make([]dto.ReleaseRisk, 0, len(branch.Risks))
				for _, raw := range branch.Risks {
					risk, err := dto.ParseReleaseRisk(raw)
					if err != nil {
						return nil, nil, err
					}
					risks = append(risks, risk)
				}
			}
			branches = append(branches, dto.TrackedReleaseBranch{
				Track:  track,
				Branch: branch.Branch,
				Risks:  risks,
			})
		}
	}
	return tracks, branches, nil
}

func effectiveProjectSeries(cfg *config.Config, project config.ProjectConfig) []string {
	if len(project.Series) > 0 {
		return append([]string(nil), project.Series...)
	}
	if cfg != nil && len(cfg.Launchpad.Series) > 0 {
		return append([]string(nil), cfg.Launchpad.Series...)
	}
	return nil
}

func discoverSnapName(repoPath string) (string, error) {
	for _, candidate := range []string{"snap/snapcraft.yaml", "snapcraft.yaml"} {
		content, err := gitcache.ReadHEADFile(repoPath, candidate)
		if err != nil {
			continue
		}
		var metadata snapcraftMetadata
		if err := yaml.Unmarshal(content, &metadata); err != nil {
			return "", fmt.Errorf("parsing %s: %w", candidate, err)
		}
		if metadata.Name == "" {
			return "", fmt.Errorf("%s does not declare a snap name", candidate)
		}
		return metadata.Name, nil
	}
	return "", fmt.Errorf("no snapcraft.yaml found at repo root or snap/")
}

func discoverCharmPublications(repoPath string) ([]discoveredCharm, error) {
	rootContent, err := gitcache.ReadHEADFile(repoPath, "charmcraft.yaml")
	if err == nil {
		charm, err := parseCharmcraft(rootContent, "charmcraft.yaml")
		if err != nil {
			return nil, err
		}
		return []discoveredCharm{charm}, nil
	}

	files, err := gitcache.FindHEADFilesByBaseName(repoPath, "charmcraft.yaml")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no charmcraft.yaml found in repository")
	}

	result := make([]discoveredCharm, 0, len(files))
	for _, file := range files {
		charm, err := parseCharmcraft(file.Content, file.Path)
		if err != nil {
			return nil, err
		}
		result = append(result, charm)
	}
	return result, nil
}

func parseCharmcraft(content []byte, source string) (discoveredCharm, error) {
	var metadata charmcraftMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		return discoveredCharm{}, fmt.Errorf("parsing %s: %w", source, err)
	}
	if metadata.Name == "" {
		return discoveredCharm{}, fmt.Errorf("%s does not declare a charm name", source)
	}
	resources := make([]string, 0, len(metadata.Resources))
	for name := range metadata.Resources {
		resources = append(resources, name)
	}
	sort.Strings(resources)
	return discoveredCharm{Name: metadata.Name, Resources: resources}, nil
}

func publicationKey(project string, artifactType dto.ArtifactType, name string) string {
	return project + "\x00" + artifactType.String() + "\x00" + name
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
