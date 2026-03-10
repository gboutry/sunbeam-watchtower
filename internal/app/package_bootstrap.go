// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/bugcache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/openstack"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/commit"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// UpstreamCacheDir returns the path to the upstream repos cache directory.
func UpstreamCacheDir() (string, error) {
	cacheDir, err := ResolveCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "upstream"), nil
}

// UpstreamRepoPath returns the local path for a given upstream repo URL.
func UpstreamRepoPath(cacheDir, repoURL string) string {
	name := filepath.Base(repoURL)
	name = strings.TrimSuffix(name, ".git")
	return filepath.Join(cacheDir, name)
}

// BuildUpstreamProvider creates an UpstreamProvider from config, or returns nil
// if upstream is not configured.
func (a *App) BuildUpstreamProvider() (port.UpstreamProvider, error) {
	cfg := a.Config
	if cfg == nil || cfg.Packages.Upstream == nil {
		return nil, nil
	}

	up := cfg.Packages.Upstream
	if up.Provider != "openstack" {
		return nil, fmt.Errorf("unsupported upstream provider %q", up.Provider)
	}

	upDir, err := UpstreamCacheDir()
	if err != nil {
		return nil, err
	}

	releasesDir := UpstreamRepoPath(upDir, up.ReleasesRepo)
	requirementsDir := UpstreamRepoPath(upDir, up.RequirementsRepo)

	return openstack.NewProvider(releasesDir, requirementsDir), nil
}

// BuildCommitSources creates commit sources backed by the local git cache.
func (a *App) BuildCommitSources() (map[string]port.CommitSource, error) {
	cfg := a.Config
	if cfg == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	a.Logger.Debug("building commit sources", "project_count", len(cfg.Projects))

	cache, err := a.GitCache()
	if err != nil {
		return nil, err
	}

	result := make(map[string]port.CommitSource, len(cfg.Projects))
	for _, proj := range cfg.Projects {
		cloneURL, err := proj.Code.CloneURL()
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", proj.Name, err)
		}

		a.Logger.Debug("configured commit source", "project", proj.Name, "cloneURL", cloneURL)

		forgeType := ForgeTypeFromConfig(proj.Code.Forge)
		result[proj.Name] = &commit.CachedGitSource{
			Cache:     cache,
			CloneURL:  cloneURL,
			Type:      forgeType,
			CommitURL: proj.Code.CommitURL,
		}
	}

	return result, nil
}

// SyncBugCache syncs the bug cache for configured projects.
// If projects is empty, all configured projects are synced.
func (a *App) SyncBugCache(ctx context.Context, projects []string) (int, error) {
	trackers, _, err := a.BuildBugTrackers()
	if err != nil {
		return 0, err
	}

	selected := stringSet(projects)
	total := 0
	for _, pbt := range trackers {
		if len(selected) > 0 && !selected[pbt.ProjectID] {
			continue
		}
		ct, ok := pbt.Tracker.(*bugcache.CachedBugTracker)
		if !ok {
			continue
		}
		synced, sErr := ct.Sync(ctx)
		if sErr != nil {
			return total, fmt.Errorf("syncing %s: %w", pbt.ProjectID, sErr)
		}
		total += synced
	}

	return total, nil
}

// BuildPackageSources resolves distro, release, suite, and backport filters against config
// to produce source entries for the package cache.
//
// Suite types are expanded relative to each release: "release" -> release name,
// "updates" -> "<release>-updates", etc.
//
// Backport filter semantics:
//   - empty/nil or ["none"]: skip all backports (default)
//   - ["gazpacho", "flamingo"]: include only those backports
func (a *App) BuildPackageSources(distros, releases, suites, backports []string) []dto.PackageSource {
	cfg := a.Config.Packages
	var sources []dto.PackageSource

	// Build backport filter.
	// nil -> include all backports (used by cache sync)
	// ["none"] -> skip all backports (default for query commands)
	// ["gazpacho", ...] -> include only named backports
	bpFilter := make(map[string]bool, len(backports))
	for _, bp := range backports {
		bpFilter[bp] = true
	}
	includeAllBackports := backports == nil
	skipAllBackports := !includeAllBackports && (len(bpFilter) == 0 || bpFilter["none"])
	filterBackports := !includeAllBackports && !skipAllBackports

	// Build release filter.
	relFilter := make(map[string]bool, len(releases))
	for _, r := range releases {
		relFilter[r] = true
	}
	filterReleases := len(relFilter) > 0

	// Build suite-type filter.
	stFilter := make(map[string]bool, len(suites))
	for _, s := range suites {
		stFilter[s] = true
	}
	filterSuiteTypes := len(stFilter) > 0

	// Resolve distros.
	distroNames := distros
	if len(distroNames) == 0 {
		for name := range cfg.Distros {
			distroNames = append(distroNames, name)
		}
	}

	for _, name := range distroNames {
		if name == "none" {
			continue
		}
		d, ok := cfg.Distros[name]
		if !ok {
			a.Logger.Warn("unknown distro in config, skipping", "distro", name)
			continue
		}

		// When backports are requested without an explicit --release filter,
		// infer releases from the config:
		//   - parent_release (where packages are uploaded natively) -> full main suites
		//   - backport target (where the backport config lives) -> backport pockets only
		// e.g. --backport gazpacho: resolute gets full suites, noble gets only gazpacho pockets.
		effectiveRelFilter := relFilter
		effectiveFilterReleases := filterReleases
		backportOnlyReleases := make(map[string]bool)
		if filterBackports && !filterReleases {
			effectiveRelFilter = make(map[string]bool)
			parentReleases := make(map[string]bool)
			for relName, rel := range d.Releases {
				for bpName, bp := range rel.Backports {
					if bpFilter[bpName] {
						effectiveRelFilter[relName] = true
						backportOnlyReleases[relName] = true
						if bp.ParentRelease != "" {
							effectiveRelFilter[bp.ParentRelease] = true
							parentReleases[bp.ParentRelease] = true
						}
					}
				}
			}
			// A release that is both a backport target and a parent release
			// for another requested backport gets full suites.
			for r := range parentReleases {
				delete(backportOnlyReleases, r)
			}
			effectiveFilterReleases = len(effectiveRelFilter) > 0
		}

		var entries []dto.SourceEntry
		for relName, rel := range d.Releases {
			if effectiveFilterReleases && !effectiveRelFilter[relName] {
				continue
			}
			// For backport-only releases, skip main suites — only include backport pockets.
			if !backportOnlyReleases[relName] {
				for _, suiteType := range rel.Suites {
					if filterSuiteTypes && !stFilter[suiteType] {
						continue
					}
					fullSuite := config.ExpandSuiteType(relName, suiteType)
					for _, comp := range d.Components {
						entries = append(entries, dto.SourceEntry{
							Mirror:    d.Mirror,
							Suite:     fullSuite,
							Component: comp,
						})
					}
				}
			}

			if skipAllBackports {
				continue
			}

			// Include backports belonging to this release.
			for bpName, bp := range rel.Backports {
				if filterBackports && !bpFilter[bpName] {
					continue
				}
				qualifiedName := name + "/" + bpName
				var bpEntries []dto.SourceEntry
				for _, src := range bp.Sources {
					for _, suite := range src.Suites {
						expandedSuite := config.ExpandBackportSuiteType(relName, bpName, suite)
						for _, comp := range src.Components {
							bpEntries = append(bpEntries, dto.SourceEntry{
								Mirror:    src.Mirror,
								Suite:     expandedSuite,
								Component: comp,
							})
						}
					}
				}
				sources = append(sources, dto.PackageSource{
					Name:    qualifiedName,
					Entries: bpEntries,
				})
			}
		}

		if len(entries) > 0 {
			sources = append(sources, dto.PackageSource{
				Name:    name,
				Entries: entries,
			})
		}
	}

	return sources
}
