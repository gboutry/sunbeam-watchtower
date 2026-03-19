// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"os"
	"path/filepath"
	"testing"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestDiscoverTargets_SnapAtRoot(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snap")
	os.MkdirAll(snapDir, 0o755)
	os.WriteFile(filepath.Join(snapDir, "snapcraft.yaml"), []byte("name: my-snap\n"), 0o644)

	targets, err := DiscoverTargets(dir, dto.ArtifactSnap)
	if err != nil {
		t.Fatalf("DiscoverTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0] != "my-snap" {
		t.Fatalf("DiscoverTargets() = %v, want [my-snap]", targets)
	}
}

func TestDiscoverTargets_CharmMonorepo(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"keystone-k8s", "nova-k8s"} {
		d := filepath.Join(dir, "charms", name)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "charmcraft.yaml"), []byte("name: "+name+"\n"), 0o644)
	}

	targets, err := DiscoverTargets(dir, dto.ArtifactCharm)
	if err != nil {
		t.Fatalf("DiscoverTargets() error = %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("DiscoverTargets() = %v, want 2 targets", targets)
	}
}

func TestDiscoverTargets_CharmAtRoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "charmcraft.yaml"), []byte("name: single-charm\n"), 0o644)

	targets, err := DiscoverTargets(dir, dto.ArtifactCharm)
	if err != nil {
		t.Fatalf("DiscoverTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0] != "single-charm" {
		t.Fatalf("DiscoverTargets() = %v, want [single-charm]", targets)
	}
}

func TestDiscoverTargets_SnapMonorepo(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, "snaps", "my-snap", "snap")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, "snapcraft.yaml"), []byte("name: my-snap\n"), 0o644)

	targets, err := DiscoverTargets(dir, dto.ArtifactSnap)
	if err != nil {
		t.Fatalf("DiscoverTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0] != "my-snap" {
		t.Fatalf("DiscoverTargets() = %v, want [my-snap]", targets)
	}
}

func TestDiscoverTargets_NoManifests(t *testing.T) {
	dir := t.TempDir()
	targets, err := DiscoverTargets(dir, dto.ArtifactSnap)
	if err != nil {
		t.Fatalf("DiscoverTargets() error = %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("DiscoverTargets() = %v, want empty", targets)
	}
}

func TestDiscoverTargets_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snap")
	os.MkdirAll(snapDir, 0o755)
	os.WriteFile(filepath.Join(snapDir, "snapcraft.yaml"), []byte("not:\n\tvalid yaml"), 0o644)

	targets, err := DiscoverTargets(dir, dto.ArtifactSnap)
	if err != nil {
		t.Fatalf("DiscoverTargets() error = %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("DiscoverTargets() = %v, want empty (malformed skipped)", targets)
	}
}
