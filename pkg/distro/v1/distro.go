// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package v1 defines domain types for APT source package tracking.
package v1

// SourcePackage represents a source package found in an APT Sources index.
type SourcePackage struct {
	Package   string `json:"package" yaml:"package"`
	Version   string `json:"version" yaml:"version"`
	Suite     string `json:"suite" yaml:"suite"`
	Component string `json:"component" yaml:"component"`
}

// SourcePackageDetail extends SourcePackage with build dependency information.
type SourcePackageDetail struct {
	SourcePackage
	BuildDepends []string `json:"build_depends,omitempty" yaml:"build_depends,omitempty"`
}

// SourcePackageFiles extends SourcePackage with file listing for .dsc URL construction.
type SourcePackageFiles struct {
	SourcePackage
	Directory string   `json:"directory,omitempty" yaml:"directory,omitempty"`
	Files     []string `json:"files,omitempty" yaml:"files,omitempty"` // filenames only
}

// SourcePackageInfo captures all metadata fields from an APT Sources paragraph.
type SourcePackageInfo struct {
	SourcePackage
	Fields []FieldEntry `json:"fields" yaml:"fields"`
}

// FieldEntry is a single key-value pair from an APT Sources paragraph.
type FieldEntry struct {
	Key   string `json:"key" yaml:"key"`
	Value string `json:"value" yaml:"value"`
}

// VersionComparison holds one package's versions across all queried sources.
type VersionComparison struct {
	Package  string                     `json:"package" yaml:"package"`
	Versions map[string][]SourcePackage `json:"versions" yaml:"versions"` // keyed by distro/backport name
}
