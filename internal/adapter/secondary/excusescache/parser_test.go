// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package excusescache

import (
	"strings"
	"testing"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestParseUbuntuExcusesYAML(t *testing.T) {
	data := `
sources:
  - item-name: nova
    source: nova
    component: main
    old-version: 1:29.0.0-0ubuntu1
    new-version: 1:29.1.0-0ubuntu1
    reason:
      - autopkgtest
    excuses:
      - "autopkgtest for nova/1:29.1.0-0ubuntu1 failed on amd64"
    missing-builds:
      on-architectures:
        - arm64
    dependencies:
      blocked-by:
        - keystone
      migrate-after:
        - placement
    policy_info:
      age:
        current-age: 4
      update-excuse:
        "1234567": true
`
	results, err := parseExcusesYAML([]byte(data), dto.ExcusesSource{Tracker: dto.ExcusesTrackerUbuntu})
	if err != nil {
		t.Fatalf("parseExcusesYAML() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	if got.Package != "nova" {
		t.Fatalf("expected package nova, got %q", got.Package)
	}
	if got.PrimaryReason != "ftbfs" {
		t.Fatalf("expected primary reason ftbfs, got %q", got.PrimaryReason)
	}
	if !got.FTBFS {
		t.Fatal("expected FTBFS to be true")
	}
	if got.AgeDays != 4 {
		t.Fatalf("expected age 4, got %d", got.AgeDays)
	}
	if got.Bug != "LP: #1234567" {
		t.Fatalf("expected LP bug, got %q", got.Bug)
	}
	if len(got.BlockedBy) != 1 || got.BlockedBy[0] != "keystone" {
		t.Fatalf("expected blocked-by keystone, got %#v", got.BlockedBy)
	}
}

func TestParseDebianExcusesYAML(t *testing.T) {
	data := `
generated-date: 2026-03-06 17:13:07.261525+00:00
sources:
  - excuses:
      - "Migration status for biomaj3-cli (3.1.11-5 to -): Will attempt migration"
      - "Additional info (not blocking):"
      - "∙ ∙ Removal request by auto-removals"
    hints:
      - hint-from: auto-removals
        hint-type: remove
    is-candidate: true
    item-name: -biomaj3-cli
    migration-policy-verdict: PASS
    new-version: "-"
    old-version: 3.1.11-5
    reason: []
    source: biomaj3-cli
    maintainer: Debian Python Team
`
	results, err := parseExcusesYAML([]byte(data), dto.ExcusesSource{Tracker: dto.ExcusesTrackerDebian})
	if err != nil {
		t.Fatalf("parseExcusesYAML() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	got := results[0]
	if got.Package != "biomaj3-cli" {
		t.Fatalf("expected package biomaj3-cli, got %q", got.Package)
	}
	if got.Maintainer != "Debian Python Team" {
		t.Fatalf("expected maintainer, got %q", got.Maintainer)
	}
	if !got.Candidate {
		t.Fatal("expected candidate to be true")
	}
	if got.PrimaryReason != "removal" {
		t.Fatalf("expected primary reason removal, got %q", got.PrimaryReason)
	}
	if len(got.Messages) == 0 || !strings.Contains(got.Messages[0], "Migration status") {
		t.Fatalf("expected parsed messages, got %#v", got.Messages)
	}
}

func TestParseUbuntuExcusesByTeamYAML(t *testing.T) {
	data := `
debcrafters-packages:
- kind: package-in-proposed
  package_in_proposed: libimagequant
  regressions: []
  waiting: []
  data: !!python/object/apply:collections.defaultdict
    args:
    - &id001 !!python/name:builtins.dict ''
    dictitems:
      item-name: libimagequant
      new-version: 4.4.1-1
      old-version: 2.18.0-1build1
      source: libimagequant
ubuntu-desktop:
- kind: package-in-proposed
  package_in_proposed: gnome-shell
  regressions: []
  waiting: []
  data: !!python/object/apply:collections.defaultdict
    args:
    - *id001
    dictitems:
      item-name: gnome-shell
      new-version: 49.0-1ubuntu1
      source: gnome-shell
`
	exactTeams, packageTeams, err := parseUbuntuExcusesByTeamYAML([]byte(data))
	if err != nil {
		t.Fatalf("parseUbuntuExcusesByTeamYAML() error = %v", err)
	}
	if got := exactTeams[recordKey("libimagequant", "4.4.1-1")]; got != "debcrafters-packages" {
		t.Fatalf("expected exact libimagequant team, got %q", got)
	}
	if got := packageTeams["gnome-shell"]; got != "ubuntu-desktop" {
		t.Fatalf("expected gnome-shell team, got %q", got)
	}
}

func TestApplyExcuseTeams(t *testing.T) {
	excuses := []dto.PackageExcuse{
		{PackageExcuseSummary: dto.PackageExcuseSummary{Package: "libimagequant", Version: "4.4.1-1"}},
		{PackageExcuseSummary: dto.PackageExcuseSummary{Package: "gnome-shell", Version: "49.0-1ubuntu1"}},
	}

	applyExcuseTeams(excuses,
		map[string]string{recordKey("libimagequant", "4.4.1-1"): "debcrafters-packages"},
		map[string]string{"gnome-shell": "ubuntu-desktop"},
	)

	if excuses[0].Team != "debcrafters-packages" {
		t.Fatalf("expected exact team on libimagequant, got %q", excuses[0].Team)
	}
	if excuses[1].Team != "ubuntu-desktop" {
		t.Fatalf("expected package fallback team on gnome-shell, got %q", excuses[1].Team)
	}
}
