// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ReleaseRisk identifies one publication risk within a track.
type ReleaseRisk string

const (
	ReleaseRiskEdge      ReleaseRisk = "edge"
	ReleaseRiskBeta      ReleaseRisk = "beta"
	ReleaseRiskCandidate ReleaseRisk = "candidate"
	ReleaseRiskStable    ReleaseRisk = "stable"
)

// KnownReleaseRisks returns risks in the canonical progression order.
func KnownReleaseRisks() []ReleaseRisk {
	return []ReleaseRisk{ReleaseRiskEdge, ReleaseRiskBeta, ReleaseRiskCandidate, ReleaseRiskStable}
}

// ParseReleaseRisk parses one risk value.
func ParseReleaseRisk(s string) (ReleaseRisk, error) {
	switch ReleaseRisk(s) {
	case ReleaseRiskEdge, ReleaseRiskBeta, ReleaseRiskCandidate, ReleaseRiskStable:
		return ReleaseRisk(s), nil
	default:
		return "", fmt.Errorf("unknown release risk %q (must be edge, beta, candidate, or stable)", s)
	}
}

// ReleaseBase identifies the base/runtime variant attached to one published revision.
type ReleaseBase struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Channel      string `json:"channel,omitempty" yaml:"channel,omitempty"`
	Architecture string `json:"architecture,omitempty" yaml:"architecture,omitempty"`
}

// ReleaseTargetSnapshot captures one published revision variant within a channel.
type ReleaseTargetSnapshot struct {
	Architecture string      `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	Base         ReleaseBase `json:"base,omitempty" yaml:"base,omitempty"`
	Revision     int         `json:"revision,omitempty" yaml:"revision,omitempty"`
	Version      string      `json:"version,omitempty" yaml:"version,omitempty"`
	ReleasedAt   time.Time   `json:"released_at,omitempty" yaml:"released_at,omitempty"`
}

// ReleaseResourceSnapshot captures one resource revision attached to a charm channel.
type ReleaseResourceSnapshot struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
	Revision    int    `json:"revision,omitempty" yaml:"revision,omitempty"`
	Filename    string `json:"filename,omitempty" yaml:"filename,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ReleaseChannelSnapshot captures one artifact publication snapshot for one track/risk.
type ReleaseChannelSnapshot struct {
	Track     string                    `json:"track" yaml:"track"`
	Risk      ReleaseRisk               `json:"risk" yaml:"risk"`
	Channel   string                    `json:"channel" yaml:"channel"`
	Targets   []ReleaseTargetSnapshot   `json:"targets,omitempty" yaml:"targets,omitempty"`
	Resources []ReleaseResourceSnapshot `json:"resources,omitempty" yaml:"resources,omitempty"`
	UpdatedAt time.Time                 `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// PublishedArtifactSnapshot stores the cached publication state for one configured artifact.
type PublishedArtifactSnapshot struct {
	Project      string                   `json:"project" yaml:"project"`
	Name         string                   `json:"name" yaml:"name"`
	ArtifactType ArtifactType             `json:"artifact_type" yaml:"artifact_type"`
	Tracks       []string                 `json:"tracks,omitempty" yaml:"tracks,omitempty"`
	Channels     []ReleaseChannelSnapshot `json:"channels,omitempty" yaml:"channels,omitempty"`
	UpdatedAt    time.Time                `json:"updated_at" yaml:"updated_at"`
}

// ReleaseListEntry is one row in the flat releases list surface.
type ReleaseListEntry struct {
	Project      string                    `json:"project" yaml:"project"`
	Name         string                    `json:"name" yaml:"name"`
	ArtifactType ArtifactType              `json:"artifact_type" yaml:"artifact_type"`
	Track        string                    `json:"track" yaml:"track"`
	Risk         ReleaseRisk               `json:"risk" yaml:"risk"`
	Channel      string                    `json:"channel" yaml:"channel"`
	Targets      []ReleaseTargetSnapshot   `json:"targets,omitempty" yaml:"targets,omitempty"`
	Resources    []ReleaseResourceSnapshot `json:"resources,omitempty" yaml:"resources,omitempty"`
	UpdatedAt    time.Time                 `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// ReleaseShowResult contains the full release matrix for one artifact.
type ReleaseShowResult struct {
	Project      string                   `json:"project" yaml:"project"`
	Name         string                   `json:"name" yaml:"name"`
	ArtifactType ArtifactType             `json:"artifact_type" yaml:"artifact_type"`
	Tracks       []string                 `json:"tracks,omitempty" yaml:"tracks,omitempty"`
	Channels     []ReleaseChannelSnapshot `json:"channels,omitempty" yaml:"channels,omitempty"`
	UpdatedAt    time.Time                `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// ReleaseListQuery defines release-list filters.
type ReleaseListQuery struct {
	Names        []string
	Projects     []string
	ArtifactType *ArtifactType
	Tracks       []string
	Risks        []ReleaseRisk
}

// TrackedPublication defines one configured published artifact to track.
type TrackedPublication struct {
	Project      string
	Name         string
	ArtifactType ArtifactType
	Tracks       []string
	Resources    []string
}

// ReleaseCacheStatus reports cached publication metadata for one tracked artifact.
type ReleaseCacheStatus struct {
	Project      string       `json:"project" yaml:"project"`
	Name         string       `json:"name" yaml:"name"`
	ArtifactType ArtifactType `json:"artifact_type" yaml:"artifact_type"`
	TrackCount   int          `json:"track_count" yaml:"track_count"`
	ChannelCount int          `json:"channel_count" yaml:"channel_count"`
	LastUpdated  time.Time    `json:"last_updated" yaml:"last_updated"`
}

// NormalizeChannel fills derived fields and ordering on the snapshot.
func NormalizeChannel(channel ReleaseChannelSnapshot) ReleaseChannelSnapshot {
	if channel.Channel == "" && channel.Track != "" && channel.Risk != "" {
		channel.Channel = channel.Track + "/" + string(channel.Risk)
	}
	sort.Slice(channel.Targets, func(i, j int) bool {
		if channel.Targets[i].Architecture == channel.Targets[j].Architecture {
			return channel.Targets[i].Base.Channel < channel.Targets[j].Base.Channel
		}
		return channel.Targets[i].Architecture < channel.Targets[j].Architecture
	})
	sort.Slice(channel.Resources, func(i, j int) bool {
		return channel.Resources[i].Name < channel.Resources[j].Name
	})
	return channel
}

// NormalizePublicationSnapshot returns a normalized copy suitable for cache storage.
func NormalizePublicationSnapshot(snapshot PublishedArtifactSnapshot) PublishedArtifactSnapshot {
	snapshot.Tracks = uniqueSortedStrings(snapshot.Tracks)
	for i := range snapshot.Channels {
		snapshot.Channels[i] = NormalizeChannel(snapshot.Channels[i])
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool {
		if snapshot.Channels[i].Track == snapshot.Channels[j].Track {
			return riskOrder(snapshot.Channels[i].Risk) < riskOrder(snapshot.Channels[j].Risk)
		}
		return snapshot.Channels[i].Track < snapshot.Channels[j].Track
	})
	return snapshot
}

// ValidateReleaseTracks ensures configured tracks are non-empty and unique.
func ValidateReleaseTracks(tracks []string) error {
	seen := make(map[string]bool, len(tracks))
	for _, track := range tracks {
		track = strings.TrimSpace(track)
		if track == "" {
			return fmt.Errorf("release track cannot be empty")
		}
		if seen[track] {
			return fmt.Errorf("duplicate release track %q", track)
		}
		seen[track] = true
	}
	return nil
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func riskOrder(risk ReleaseRisk) int {
	for idx, known := range KnownReleaseRisks() {
		if risk == known {
			return idx
		}
	}
	return len(KnownReleaseRisks())
}
