// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package pkg

import (
	"context"
	"fmt"
	"sort"

	distro "github.com/gboutry/sunbeam-watchtower/internal/pkg/distro/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// DiffOpts controls the diff operation.
type DiffOpts struct {
	Packages   []string        // package names to compare
	Sources    []ProjectSource // which distros/backports to query
	Suites     []string        // optional suite filter
	Components []string        // optional component filter
}

// DiffResult holds one package's versions across all queried sources.
type DiffResult struct {
	Package  string                           `json:"package" yaml:"package"`
	Versions map[string][]distro.SourcePackage `json:"versions" yaml:"versions"` // source name → versions
	Upstream string                           `json:"upstream,omitempty" yaml:"upstream,omitempty"`
}

// Diff compares package versions across multiple sources.
func (s *Service) Diff(ctx context.Context, opts DiffOpts) ([]DiffResult, error) {
	// Map: package name → source name → []SourcePackage
	grouped := make(map[string]map[string][]distro.SourcePackage)

	for _, src := range opts.Sources {
		pkgs, err := s.cache.Query(ctx, src.Name, port.QueryOpts{
			Packages:   opts.Packages,
			Suites:     opts.Suites,
			Components: opts.Components,
		})
		if err != nil {
			return nil, fmt.Errorf("querying %s: %w", src.Name, err)
		}

		for _, pkg := range pkgs {
			if grouped[pkg.Package] == nil {
				grouped[pkg.Package] = make(map[string][]distro.SourcePackage)
			}
			grouped[pkg.Package][src.Name] = append(grouped[pkg.Package][src.Name], pkg)
		}
	}

	results := make([]DiffResult, 0, len(grouped))
	for name, versions := range grouped {
		results = append(results, DiffResult{
			Package:  name,
			Versions: versions,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Package < results[j].Package
	})

	return results, nil
}
