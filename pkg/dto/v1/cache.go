// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
	"time"

	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

// SyncOptions controls additional behavior when syncing a cached repository.
type SyncOptions struct {
	ExtraRefSpecs []string
}

// MRMetadata stores merge request information as a sidecar to the cached repo.
type MRMetadata struct {
	ID      string           `json:"id"`
	State   forge.MergeState `json:"state"`
	URL     string           `json:"url"`
	HeadSHA string           `json:"head_sha"`
	GitRef  string           `json:"git_ref"`
}

// SourceEntry represents a single APT Sources index to download.
type SourceEntry struct {
	Mirror    string `json:"mirror" yaml:"mirror"`
	Suite     string `json:"suite" yaml:"suite"`
	Component string `json:"component" yaml:"component"`
}

// QueryOpts controls filtering when querying the cache.
type QueryOpts struct {
	Packages   []string `json:"packages,omitempty" yaml:"packages,omitempty"`
	Suites     []string `json:"suites,omitempty" yaml:"suites,omitempty"`
	Components []string `json:"components,omitempty" yaml:"components,omitempty"`
}

// CacheStatus reports metadata about a cached source group.
type CacheStatus struct {
	Name        string    `json:"name" yaml:"name"`
	EntryCount  int       `json:"entry_count" yaml:"entry_count"`
	LastUpdated time.Time `json:"last_updated" yaml:"last_updated"`
	DiskSize    int64     `json:"disk_size" yaml:"disk_size"`
}
