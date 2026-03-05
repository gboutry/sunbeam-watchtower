// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"
	"time"

	distro "github.com/gboutry/sunbeam-watchtower/internal/pkg/distro/v1"
)

// DistroCache manages downloading, indexing, and querying APT Sources data.
type DistroCache interface {
	// Update downloads Sources indexes and rebuilds the bbolt index.
	// name identifies the source group (e.g. "ubuntu", "uca").
	// entries defines what to download (mirror + suite + component combos).
	Update(ctx context.Context, name string, entries []SourceEntry) error

	// Query returns source packages matching the given criteria from the index.
	// If opts.Packages is empty, returns all packages. Filters by suite/component if set.
	Query(ctx context.Context, name string, opts QueryOpts) ([]distro.SourcePackage, error)

	// QueryDetailed returns source packages with build dependency information.
	QueryDetailed(ctx context.Context, name string, opts QueryOpts) ([]distro.SourcePackageDetail, error)

	// Status returns cache metadata (last updated, entry count) per indexed source.
	Status() ([]CacheStatus, error)

	// CacheDir returns the base directory for the cache.
	CacheDir() string

	// Close releases resources held by the cache.
	Close() error
}

// SourceEntry represents a single APT Sources index to download.
type SourceEntry struct {
	Mirror    string
	Suite     string
	Component string
}

// QueryOpts controls filtering when querying the cache.
type QueryOpts struct {
	Packages   []string // filter by package name (empty = all)
	Suites     []string // filter by suite (empty = all)
	Components []string // filter by component (empty = all)
}

// CacheStatus reports metadata about a cached source group.
type CacheStatus struct {
	Name        string    `json:"name" yaml:"name"`
	EntryCount  int       `json:"entry_count" yaml:"entry_count"`
	LastUpdated time.Time `json:"last_updated" yaml:"last_updated"`
	DiskSize    int64     `json:"disk_size" yaml:"disk_size"`
}
