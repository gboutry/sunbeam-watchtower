// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
	"fmt"
	"time"
)

const (
	// ExcusesTrackerUbuntu is the built-in tracker for Ubuntu proposed-migration.
	ExcusesTrackerUbuntu = "ubuntu"
	// ExcusesTrackerDebian is the built-in tracker for Debian britney excuses.
	ExcusesTrackerDebian = "debian"
)

// ExcusesSource identifies one upstream excuses feed and the provider-specific
// behavior needed to normalize it. URLs are owned by the provider; the cache
// resolves them from the provider name.
type ExcusesSource struct {
	Tracker  string `json:"tracker" yaml:"tracker"`
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
}

// KnownExcusesSources returns the built-in excuses feeds supported by Watchtower.
func KnownExcusesSources() []ExcusesSource {
	return []ExcusesSource{
		{Tracker: ExcusesTrackerUbuntu, Provider: ExcusesTrackerUbuntu},
		{Tracker: ExcusesTrackerDebian, Provider: ExcusesTrackerDebian},
	}
}

// ExcusesTrackers returns the tracker names present in the given sources.
func ExcusesTrackers(sources []ExcusesSource) []string {
	trackers := make([]string, 0, len(sources))
	for _, source := range sources {
		trackers = append(trackers, source.Tracker)
	}
	return trackers
}

// KnownExcusesTrackers returns the supported built-in tracker names.
func KnownExcusesTrackers() []string {
	return ExcusesTrackers(KnownExcusesSources())
}

// ExcusesSourceByTracker looks up an excuses feed by tracker name.
func ExcusesSourceByTracker(sources []ExcusesSource, tracker string) (ExcusesSource, bool) {
	for _, source := range sources {
		if source.Tracker == tracker {
			return source, true
		}
	}
	return ExcusesSource{}, false
}

// FilterExcusesSources returns the excuses feeds matching the requested trackers.
// If trackers is empty, all given feeds are returned.
func FilterExcusesSources(sources []ExcusesSource, trackers []string) []ExcusesSource {
	if len(trackers) == 0 {
		return append([]ExcusesSource(nil), sources...)
	}

	var filtered []ExcusesSource
	for _, tracker := range trackers {
		if source, ok := ExcusesSourceByTracker(sources, tracker); ok {
			filtered = append(filtered, source)
		}
	}
	return filtered
}

// ValidateExcusesTrackers validates that all requested trackers exist in the given sources.
func ValidateExcusesTrackers(sources []ExcusesSource, trackers []string) error {
	known := map[string]bool{}
	for _, tracker := range ExcusesTrackers(sources) {
		known[tracker] = true
	}
	for _, tracker := range trackers {
		if !known[tracker] {
			return fmt.Errorf("unknown excuses tracker %q", tracker)
		}
	}
	return nil
}

// ExcuseQueryOpts controls filtering when listing excuses from the cache.
type ExcuseQueryOpts struct {
	Trackers          []string `json:"trackers,omitempty" yaml:"trackers,omitempty"`
	Name              string   `json:"name,omitempty" yaml:"name,omitempty"`
	Component         string   `json:"component,omitempty" yaml:"component,omitempty"`
	Team              string   `json:"team,omitempty" yaml:"team,omitempty"`
	FTBFS             bool     `json:"ftbfs,omitempty" yaml:"ftbfs,omitempty"`
	Autopkgtest       bool     `json:"autopkgtest,omitempty" yaml:"autopkgtest,omitempty"`
	BlockedBy         string   `json:"blocked_by,omitempty" yaml:"blocked_by,omitempty"`
	Bugged            bool     `json:"bugged,omitempty" yaml:"bugged,omitempty"`
	MinAge            int      `json:"min_age,omitempty" yaml:"min_age,omitempty"`
	MaxAge            int      `json:"max_age,omitempty" yaml:"max_age,omitempty"`
	Limit             int      `json:"limit,omitempty" yaml:"limit,omitempty"`
	Reverse           bool     `json:"reverse,omitempty" yaml:"reverse,omitempty"`
	Packages          []string `json:"packages,omitempty" yaml:"packages,omitempty"`
	BlockedByPackages []string `json:"blocked_by_packages,omitempty" yaml:"blocked_by_packages,omitempty"`
}

// PackageExcuseSummary holds the normalized fields needed for list/table views.
type PackageExcuseSummary struct {
	Tracker       string   `json:"tracker" yaml:"tracker"`
	Package       string   `json:"package" yaml:"package"`
	ItemName      string   `json:"item_name,omitempty" yaml:"item_name,omitempty"`
	Version       string   `json:"version" yaml:"version"`
	OldVersion    string   `json:"old_version,omitempty" yaml:"old_version,omitempty"`
	Component     string   `json:"component,omitempty" yaml:"component,omitempty"`
	AgeDays       int      `json:"age_days,omitempty" yaml:"age_days,omitempty"`
	FTBFS         bool     `json:"ftbfs,omitempty" yaml:"ftbfs,omitempty"`
	Candidate     bool     `json:"candidate,omitempty" yaml:"candidate,omitempty"`
	Verdict       string   `json:"verdict,omitempty" yaml:"verdict,omitempty"`
	PrimaryReason string   `json:"primary_reason,omitempty" yaml:"primary_reason,omitempty"`
	Team          string   `json:"team,omitempty" yaml:"team,omitempty"`
	Maintainer    string   `json:"maintainer,omitempty" yaml:"maintainer,omitempty"`
	Bug           string   `json:"bug,omitempty" yaml:"bug,omitempty"`
	BlockedBy     []string `json:"blocked_by,omitempty" yaml:"blocked_by,omitempty"`
	BlocksCount   int      `json:"blocks_count,omitempty" yaml:"blocks_count,omitempty"`
}

// ExcuseReason represents one normalized reason or signal explaining why migration is blocked.
type ExcuseReason struct {
	Code     string `json:"code" yaml:"code"`
	Message  string `json:"message,omitempty" yaml:"message,omitempty"`
	Blocking bool   `json:"blocking,omitempty" yaml:"blocking,omitempty"`
}

// ExcuseDependency captures one blocker relationship between source packages.
type ExcuseDependency struct {
	Kind    string `json:"kind" yaml:"kind"`
	Package string `json:"package" yaml:"package"`
}

// ExcuseAutopkgtest captures one autopkgtest signal extracted from excuses data.
type ExcuseAutopkgtest struct {
	Package      string `json:"package,omitempty" yaml:"package,omitempty"`
	Architecture string `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	Status       string `json:"status,omitempty" yaml:"status,omitempty"`
	URL          string `json:"url,omitempty" yaml:"url,omitempty"`
	Message      string `json:"message,omitempty" yaml:"message,omitempty"`
}

// ExcuseBuildFailure captures one build-related blocker.
type ExcuseBuildFailure struct {
	Architecture string `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	Kind         string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Message      string `json:"message,omitempty" yaml:"message,omitempty"`
}

// PackageExcuse holds the normalized detail view for one package excuse.
type PackageExcuse struct {
	PackageExcuseSummary `yaml:",inline"`
	Reasons              []ExcuseReason       `json:"reasons,omitempty" yaml:"reasons,omitempty"`
	Dependencies         []ExcuseDependency   `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	ReverseDependencies  []string             `json:"reverse_dependencies,omitempty" yaml:"reverse_dependencies,omitempty"`
	Autopkgtests         []ExcuseAutopkgtest  `json:"autopkgtests,omitempty" yaml:"autopkgtests,omitempty"`
	BuildFailures        []ExcuseBuildFailure `json:"build_failures,omitempty" yaml:"build_failures,omitempty"`
	Links                map[string]string    `json:"links,omitempty" yaml:"links,omitempty"`
	Messages             []string             `json:"messages,omitempty" yaml:"messages,omitempty"`
}

// ExcusesCacheStatus reports the state of one cached excuses tracker.
type ExcusesCacheStatus struct {
	Tracker     string    `json:"tracker" yaml:"tracker"`
	URL         string    `json:"url" yaml:"url"`
	EntryCount  int       `json:"entry_count" yaml:"entry_count"`
	LastUpdated time.Time `json:"last_updated" yaml:"last_updated"`
	DiskSize    int64     `json:"disk_size" yaml:"disk_size"`
}
