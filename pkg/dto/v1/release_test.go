// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
	"testing"
	"time"
)

func TestParseReleaseRisk(t *testing.T) {
	for _, risk := range []string{"edge", "beta", "candidate", "stable"} {
		parsed, err := ParseReleaseRisk(risk)
		if err != nil {
			t.Fatalf("ParseReleaseRisk(%q) error = %v", risk, err)
		}
		if string(parsed) != risk {
			t.Fatalf("ParseReleaseRisk(%q) = %q", risk, parsed)
		}
	}
	if _, err := ParseReleaseRisk("weird"); err == nil {
		t.Fatal("ParseReleaseRisk() should reject invalid risk")
	}
}

func TestNormalizePublicationSnapshot(t *testing.T) {
	now := time.Now().UTC()
	snapshot := NormalizePublicationSnapshot(PublishedArtifactSnapshot{
		Project:      "sunbeam",
		Name:         "keystone-k8s",
		ArtifactType: ArtifactCharm,
		Tracks:       []string{"2024.1", "2024.1", "2025.1"},
		Channels: []ReleaseChannelSnapshot{{
			Track:     "2025.1",
			Risk:      ReleaseRiskEdge,
			Branch:    "risc-v",
			Targets:   []ReleaseTargetSnapshot{{Architecture: "arm64"}, {Architecture: "amd64"}},
			Resources: []ReleaseResourceSnapshot{{Name: "b"}, {Name: "a"}},
		}, {
			Track:   "2024.1",
			Risk:    ReleaseRiskStable,
			Channel: "2024.1/stable",
		}},
		UpdatedAt: now,
	})
	if len(snapshot.Tracks) != 2 || snapshot.Tracks[0] != "2024.1" {
		t.Fatalf("Tracks = %+v, want unique sorted tracks", snapshot.Tracks)
	}
	if snapshot.Channels[0].Track != "2024.1" || snapshot.Channels[1].Channel != "2025.1/edge/risc-v" {
		t.Fatalf("Channels = %+v, want sorted channels with derived name", snapshot.Channels)
	}
	if snapshot.Channels[1].Targets[0].Architecture != "amd64" || snapshot.Channels[1].Resources[0].Name != "a" {
		t.Fatalf("normalized channel = %+v, want sorted targets/resources", snapshot.Channels[1])
	}
}

func TestValidateReleaseTracks(t *testing.T) {
	if err := ValidateReleaseTracks([]string{"2024.1", "2025.1"}); err != nil {
		t.Fatalf("ValidateReleaseTracks() error = %v", err)
	}
	if err := ValidateReleaseTracks([]string{"2024.1", "2024.1"}); err == nil {
		t.Fatal("ValidateReleaseTracks() should reject duplicates")
	}
	if err := ValidateReleaseTracks([]string{"", "2024.1"}); err == nil {
		t.Fatal("ValidateReleaseTracks() should reject empty track")
	}
}

func TestParseReleaseChannelName(t *testing.T) {
	track, risk, branch, err := ParseReleaseChannelName("2024.1/stable/risc-v")
	if err != nil {
		t.Fatalf("ParseReleaseChannelName() error = %v", err)
	}
	if track != "2024.1" || risk != ReleaseRiskStable || branch != "risc-v" {
		t.Fatalf("ParseReleaseChannelName() = %q %q %q", track, risk, branch)
	}
}

func TestTrackedPublicationAllowsChannel(t *testing.T) {
	publication := TrackedPublication{
		Tracks: []string{"2024.1"},
		Branches: []TrackedReleaseBranch{{
			Track:  "2024.1",
			Branch: "risc-v",
			Risks:  []ReleaseRisk{ReleaseRiskEdge, ReleaseRiskStable},
		}},
	}
	if !publication.AllowsChannel("2024.1", ReleaseRiskStable, "") {
		t.Fatal("AllowsChannel() should allow tracked base channel")
	}
	if !publication.AllowsChannel("2024.1", ReleaseRiskEdge, "risc-v") {
		t.Fatal("AllowsChannel() should allow tracked branch channel")
	}
	if publication.AllowsChannel("2024.1", ReleaseRiskBeta, "risc-v") {
		t.Fatal("AllowsChannel() should reject untracked branch risk")
	}
}
