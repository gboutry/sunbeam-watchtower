// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
	distro "github.com/gboutry/sunbeam-watchtower/internal/pkg/distro/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// PackageSource associates a name (e.g. "ubuntu", "uca") with its source entries.
type PackageSource struct {
	Name    string             `json:"name" yaml:"name"`
	Entries []port.SourceEntry `json:"entries" yaml:"entries"`
}

// PackageDiffResult holds one package's versions across all queried sources.
type PackageDiffResult struct {
	Package  string                            `json:"package" yaml:"package"`
	Versions map[string][]distro.SourcePackage `json:"versions" yaml:"versions"`
	Upstream string                            `json:"upstream,omitempty" yaml:"upstream,omitempty"`
}

// PackageDscResult holds .dsc URL lookup results for a package/version pair.
type PackageDscResult struct {
	Package string   `json:"package" yaml:"package"`
	Version string   `json:"version" yaml:"version"`
	URLs    []string `json:"urls" yaml:"urls"`
}
