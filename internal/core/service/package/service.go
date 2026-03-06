// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package pkg

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	distro "github.com/gboutry/sunbeam-watchtower/pkg/distro/v1"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ProjectSource associates a name (e.g. "ubuntu", "uca") with its source entries.
type ProjectSource = dto.PackageSource

// Service orchestrates package-related operations using a DistroCache.
type Service struct {
	cache  port.DistroCache
	logger *slog.Logger
}

// NewService creates a new package service.
func NewService(cache port.DistroCache, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{cache: cache, logger: logger}
}

// Show returns version information for a single package across all given sources.
func (s *Service) Show(ctx context.Context, pkgName string, sources []ProjectSource) (*DiffResult, error) {
	results, err := s.Diff(ctx, DiffOpts{
		Packages: []string{pkgName},
		Sources:  sources,
	})
	if err != nil {
		return nil, err
	}
	for i := range results {
		if results[i].Package == pkgName {
			return &results[i], nil
		}
	}
	return &DiffResult{Package: pkgName, Versions: make(map[string][]distro.SourcePackage)}, nil
}

// List returns all source packages from a single source group matching the given opts.
func (s *Service) List(ctx context.Context, source string, opts dto.QueryOpts) ([]distro.SourcePackage, error) {
	return s.cache.Query(ctx, source, opts)
}

// UpdateCache downloads and indexes Sources files for the given sources.
func (s *Service) UpdateCache(ctx context.Context, sources []ProjectSource) error {
	for _, src := range sources {
		s.logger.Info("updating cache", "source", src.Name, "entries", len(src.Entries))
		if err := s.cache.Update(ctx, src.Name, src.Entries); err != nil {
			return err
		}
	}
	return nil
}

// PackageVersionPair identifies a source package by name and version.
type PackageVersionPair struct {
	Package string
	Version string
}

// DscResult holds .dsc URL lookup results for a package/version pair.
type DscResult = dto.PackageDscResult

// FindDsc searches raw Sources files for .dsc URLs matching the given package/version pairs.
func (s *Service) FindDsc(ctx context.Context, pairs []PackageVersionPair, sources []ProjectSource) ([]DscResult, error) {
	// Build a lookup key set for fast matching.
	type pairKey struct{ pkg, ver string }
	wanted := make(map[pairKey]bool, len(pairs))
	for _, p := range pairs {
		wanted[pairKey{p.Package, p.Version}] = true
	}

	// Collect URLs per pair.
	type pairResult struct{ urls []string }
	found := make(map[pairKey]*pairResult, len(pairs))
	for k := range wanted {
		found[k] = &pairResult{}
	}

	cacheDir := s.cache.CacheDir()

	for _, src := range sources {
		for _, entry := range src.Entries {
			mirror := strings.TrimRight(entry.Mirror, "/")

			// Try both formats (xz, gz) to find the raw Sources file.
			var pkgs []distro.SourcePackageFiles
			for _, format := range []string{"xz", "gz"} {
				fname := distro.SourcesFileName(entry.Suite, entry.Component, format)
				path := filepath.Join(cacheDir, "sources", src.Name, fname)
				parsed, err := distro.ParseSourcesFileWithFiles(path, format, entry.Suite, entry.Component)
				if err != nil {
					continue
				}
				pkgs = parsed
				break
			}

			for _, p := range pkgs {
				k := pairKey{p.Package, p.Version}
				pr, ok := found[k]
				if !ok {
					continue
				}
				// Find the .dsc filename.
				for _, f := range p.Files {
					if strings.HasSuffix(f, ".dsc") {
						url := mirror + "/" + p.Directory + "/" + f
						pr.urls = append(pr.urls, url)
					}
				}
			}
		}
	}

	results := make([]DscResult, 0, len(pairs))
	for _, p := range pairs {
		k := pairKey{p.Package, p.Version}
		results = append(results, DscResult{
			Package: p.Package,
			Version: p.Version,
			URLs:    found[k].urls,
		})
	}

	return results, nil
}

// ShowDetail returns the full APT metadata for a specific package. If version is
// provided, returns the exact match. Otherwise, returns the highest version found
// across the given sources.
func (s *Service) ShowDetail(ctx context.Context, pkgName, version string, sources []ProjectSource) (*distro.SourcePackageInfo, error) {
	// If no explicit version, use the cache to find the highest version.
	if version == "" {
		result, err := s.Show(ctx, pkgName, sources)
		if err != nil {
			return nil, err
		}
		var best *distro.SourcePackage
		for _, versions := range result.Versions {
			h := distro.PickHighest(versions)
			if h != nil && (best == nil || distro.CompareVersions(h.Version, best.Version) > 0) {
				best = h
			}
		}
		if best == nil {
			return nil, fmt.Errorf("package %q not found in configured sources", pkgName)
		}
		version = best.Version
	}

	cacheDir := s.cache.CacheDir()

	for _, src := range sources {
		for _, entry := range src.Entries {
			var pkgs []distro.SourcePackageInfo
			for _, format := range []string{"xz", "gz"} {
				fname := distro.SourcesFileName(entry.Suite, entry.Component, format)
				path := filepath.Join(cacheDir, "sources", src.Name, fname)
				parsed, err := distro.ParseSourcesFileFull(path, format, entry.Suite, entry.Component)
				if err != nil {
					continue
				}
				pkgs = parsed
				break
			}

			for i := range pkgs {
				if pkgs[i].Package == pkgName && pkgs[i].Version == version {
					return &pkgs[i], nil
				}
			}
		}
	}

	return nil, fmt.Errorf("package %s=%s not found in Sources files", pkgName, version)
}

// CacheStatus returns metadata about the cached source groups.
func (s *Service) CacheStatus() ([]dto.CacheStatus, error) {
	return s.cache.Status()
}
func (s *Service) ReverseDepends(ctx context.Context, pkg string, sources []ProjectSource, opts dto.QueryOpts) ([]distro.SourcePackageDetail, error) {
	// Build the set of names to match (python-foo ↔ python3-foo).
	matchNames := map[string]bool{pkg: true}
	if strings.HasPrefix(pkg, "python3-") {
		matchNames["python-"+strings.TrimPrefix(pkg, "python3-")] = true
	} else if strings.HasPrefix(pkg, "python-") {
		matchNames["python3-"+strings.TrimPrefix(pkg, "python-")] = true
	}

	var results []distro.SourcePackageDetail
	for _, src := range sources {
		pkgs, err := s.cache.QueryDetailed(ctx, src.Name, opts)
		if err != nil {
			return nil, fmt.Errorf("querying detailed %s: %w", src.Name, err)
		}
		for _, p := range pkgs {
			for _, dep := range p.BuildDepends {
				if matchNames[dep] {
					results = append(results, p)
					break
				}
			}
		}
	}
	return results, nil
}
