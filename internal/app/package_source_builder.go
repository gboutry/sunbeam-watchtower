// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"log/slog"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// buildPackageSources resolves distro, release, suite, and backport filters
// against the given package config to produce source entries.
//
// Backport filter semantics:
//   - nil: include all backports (used by cache sync)
//   - empty or ["none"]: skip all backports (default for query commands)
//   - ["gazpacho", ...]: include only named backports
func buildPackageSources(cfg config.PackagesConfig, distros, releases, suites, backports []string, logger *slog.Logger) []dto.PackageSource {
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
			logger.Warn("unknown distro in config, skipping", "distro", name)
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
