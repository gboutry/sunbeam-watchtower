// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestTrackedReleases(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	snapRemote := createReleaseTestRemote(t, map[string]string{
		"snap/snapcraft.yaml": "name: snap-openstack\n",
	})
	charmRemote := createReleaseTestRemote(t, map[string]string{
		"charms/keystone/charmcraft.yaml": "name: keystone-k8s\nresources:\n  keystone-image:\n    type: oci-image\n",
		"charms/glance/charmcraft.yaml":   "name: glance-k8s\n",
	})

	application := NewApp(&config.Config{Projects: []config.ProjectConfig{{
		Name:         "sunbeam",
		ArtifactType: "snap",
		Series:       []string{"2024.1", "2025.1"},
		Code:         config.CodeConfig{Forge: "github", Owner: "canonical", Project: "snap-openstack", GitURL: snapRemote},
		Release: &config.ProjectReleaseConfig{
			TrackMap: map[string]string{"2025.1": "latest"},
			Branches: []config.ProjectReleaseBranchConfig{{
				Series: "2024.1",
				Branch: "risc-v",
				Risks:  []string{"edge", "stable"},
			}},
		},
	}, {
		Name:         "sunbeam-charms",
		ArtifactType: "charm",
		Series:       []string{"2024.1"},
		Code:         config.CodeConfig{Forge: "gerrit", Host: "https://review.opendev.org", Project: "openstack/sunbeam-charms", GitURL: charmRemote},
	}}}, nil)

	publications, err := application.TrackedReleases(context.Background())
	if err != nil {
		t.Fatalf("TrackedReleases() error = %v", err)
	}
	if len(publications) != 3 {
		t.Fatalf("TrackedReleases() = %+v, want 3 entries", publications)
	}

	byName := make(map[string]dto.TrackedPublication, len(publications))
	for _, publication := range publications {
		byName[publication.Name] = publication
	}

	if got := byName["glance-k8s"]; got.ArtifactType != dto.ArtifactCharm {
		t.Fatalf("TrackedReleases() glance = %+v, want charm publication", got)
	}
	if got := byName["keystone-k8s"]; len(got.Resources) != 1 || got.Resources[0] != "keystone-image" {
		t.Fatalf("TrackedReleases() keystone = %+v, want discovered resources", got)
	}
	if got := byName["snap-openstack"]; len(got.Tracks) != 2 || got.Tracks[1] != "latest" {
		t.Fatalf("TrackedReleases() snap = %+v, want mapped snap tracks", got)
	}
	if got := byName["snap-openstack"]; len(got.Branches) != 1 || got.Branches[0].Branch != "risc-v" || got.Branches[0].Track != "2024.1" {
		t.Fatalf("TrackedReleases() snap branches = %+v, want resolved branch override", got.Branches)
	}
}

func createReleaseTestRemote(t *testing.T, files map[string]string) string {
	t.Helper()

	worktreeDir := filepath.Join(t.TempDir(), "repo")
	repo, err := git.PlainInit(worktreeDir, false)
	if err != nil {
		t.Fatalf("PlainInit() error = %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}
	for name, content := range files {
		fullPath := filepath.Join(worktreeDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", fullPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", fullPath, err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("Add(%q) error = %v", name, err)
		}
	}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com"},
	}); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	return "file://" + worktreeDir
}
