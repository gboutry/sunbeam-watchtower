// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"sort"
	"testing"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// helper to sort and compare string slices
func sortedEqual(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// --- RockStrategy tests ---

func TestRockStrategy_ArtifactType(t *testing.T) {
	s := &RockStrategy{}
	if s.ArtifactType() != dto.ArtifactRock {
		t.Fatalf("expected ArtifactRock")
	}
}

func TestRockStrategy_MetadataFileName(t *testing.T) {
	s := &RockStrategy{}
	if s.MetadataFileName() != "rockcraft.yaml" {
		t.Fatalf("expected rockcraft.yaml")
	}
}

func TestRockStrategy_BuildPath(t *testing.T) {
	s := &RockStrategy{}
	if got := s.BuildPath("keystone"); got != "rocks/keystone" {
		t.Fatalf("got %q, want %q", got, "rocks/keystone")
	}
}

func TestRockStrategy_ParsePlatforms_Simple(t *testing.T) {
	yaml := []byte(`
platforms:
  amd64:
  arm64:
`)
	s := &RockStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64", "arm64"})
}

func TestRockStrategy_ParsePlatforms_Complex(t *testing.T) {
	yaml := []byte(`
platforms:
  multi-arch:
    build-on: [arm64]
    build-for: [amd64]
  native:
    build-on: [s390x]
    build-for: [s390x]
`)
	s := &RockStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64", "s390x"})
}

func TestRockStrategy_ParsePlatforms_Empty(t *testing.T) {
	yaml := []byte(`name: test`)
	s := &RockStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64"})
}

// --- CharmStrategy tests ---

func TestCharmStrategy_ArtifactType(t *testing.T) {
	s := &CharmStrategy{}
	if s.ArtifactType() != dto.ArtifactCharm {
		t.Fatalf("expected ArtifactCharm")
	}
}

func TestCharmStrategy_MetadataFileName(t *testing.T) {
	s := &CharmStrategy{}
	if s.MetadataFileName() != "charmcraft.yaml" {
		t.Fatalf("expected charmcraft.yaml")
	}
}

func TestCharmStrategy_BuildPath(t *testing.T) {
	s := &CharmStrategy{}
	if got := s.BuildPath("mysql-k8s"); got != "charms/mysql-k8s" {
		t.Fatalf("got %q, want %q", got, "charms/mysql-k8s")
	}
}

func TestCharmStrategy_ParsePlatforms_NewSyntax(t *testing.T) {
	yaml := []byte(`
platforms:
  amd64:
  arm64:
`)
	s := &CharmStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64", "arm64"})
}

func TestCharmStrategy_ParsePlatforms_OldBases(t *testing.T) {
	yaml := []byte(`
bases:
  - build-on:
      - name: ubuntu
        channel: "22.04"
        architectures: [amd64]
    run-on:
      - name: ubuntu
        channel: "22.04"
        architectures: [amd64, arm64]
`)
	s := &CharmStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64", "arm64"})
}

func TestCharmStrategy_ParsePlatforms_OldBasesBuildOnFallback(t *testing.T) {
	yaml := []byte(`
bases:
  - build-on:
      - name: ubuntu
        channel: "22.04"
        architectures: [amd64, s390x]
`)
	s := &CharmStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64", "s390x"})
}

func TestCharmStrategy_ParsePlatforms_Empty(t *testing.T) {
	yaml := []byte(`name: test`)
	s := &CharmStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64"})
}

// --- SnapStrategy tests ---

func TestSnapStrategy_ArtifactType(t *testing.T) {
	s := &SnapStrategy{}
	if s.ArtifactType() != dto.ArtifactSnap {
		t.Fatalf("expected ArtifactSnap")
	}
}

func TestSnapStrategy_MetadataFileName(t *testing.T) {
	s := &SnapStrategy{}
	if s.MetadataFileName() != "snapcraft.yaml" {
		t.Fatalf("expected snapcraft.yaml")
	}
}

func TestSnapStrategy_BuildPath(t *testing.T) {
	s := &SnapStrategy{}
	if got := s.BuildPath("mysnap"); got != "" {
		t.Fatalf("got %q, want %q", got, "")
	}
}

func TestSnapStrategy_ParsePlatforms_Architectures(t *testing.T) {
	yaml := []byte(`
architectures:
  - build-on: [amd64]
    build-for: [amd64]
  - build-on: [arm64]
    build-for: [arm64]
`)
	s := &SnapStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64", "arm64"})
}

func TestSnapStrategy_ParsePlatforms_Platforms(t *testing.T) {
	yaml := []byte(`
platforms:
  amd64:
  arm64:
`)
	s := &SnapStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64", "arm64"})
}

func TestSnapStrategy_ParsePlatforms_Default(t *testing.T) {
	yaml := []byte(`name: test`)
	s := &SnapStrategy{}
	got, err := s.ParsePlatforms(yaml)
	if err != nil {
		t.Fatal(err)
	}
	sortedEqual(t, got, []string{"amd64"})
}

// --- TempRecipeName tests ---

func TestTempRecipeName(t *testing.T) {
	tests := []struct {
		strategy ArtifactStrategy
		name     string
		sha      string
		prefix   string
		want     string
	}{
		{&RockStrategy{}, "keystone", "abcdef1234567890", "temp", "temp-abcdef12-keystone"},
		{&CharmStrategy{}, "mysql-k8s", "12345678", "pr", "pr-12345678-mysql-k8s"},
		{&SnapStrategy{}, "mysnap", "short", "test", "test-short-mysnap"},
	}
	for _, tt := range tests {
		got := tt.strategy.TempRecipeName(tt.name, tt.sha, tt.prefix)
		if got != tt.want {
			t.Errorf("TempRecipeName(%q, %q, %q) = %q, want %q", tt.name, tt.sha, tt.prefix, got, tt.want)
		}
	}
}
