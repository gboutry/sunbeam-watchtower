// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/charmhub"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/releasecache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/snapstore"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
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

// TrackedPublications returns the configured published snap/charm artifacts.
func (a *App) TrackedPublications() ([]dto.TrackedPublication, error) {
	if a == nil || a.Config == nil {
		return nil, nil
	}
	var publications []dto.TrackedPublication
	for _, project := range a.Config.Projects {
		for _, publication := range project.Publications {
			artifactType, err := dto.ParseArtifactType(publication.Type)
			if err != nil {
				return nil, fmt.Errorf("project %s publication %s: %w", project.Name, publication.Name, err)
			}
			publications = append(publications, dto.TrackedPublication{
				Project:      project.Name,
				Name:         publication.Name,
				ArtifactType: artifactType,
				Tracks:       append([]string(nil), publication.Tracks...),
				Resources:    append([]string(nil), publication.Resources...),
			})
		}
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
	return publications, nil
}

// BuildReleaseSources creates the release sources supported by the current process.
func (a *App) BuildReleaseSources() map[dto.ArtifactType]port.ReleaseSource {
	client := &http.Client{Timeout: 30 * time.Second}
	return map[dto.ArtifactType]port.ReleaseSource{
		dto.ArtifactSnap:  snapstore.NewSource(client),
		dto.ArtifactCharm: charmhub.NewSource(client),
	}
}
