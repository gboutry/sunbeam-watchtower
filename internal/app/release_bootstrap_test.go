// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestTrackedPublications(t *testing.T) {
	application := NewApp(&config.Config{Projects: []config.ProjectConfig{{
		Name: "sunbeam",
		Publications: []config.ProjectPublicationConfig{{
			Name:   "snap-openstack",
			Type:   "snap",
			Tracks: []string{"2024.1", "2025.1"},
		}, {
			Name:      "keystone-k8s",
			Type:      "charm",
			Tracks:    []string{"2024.1"},
			Resources: []string{"keystone-image"},
		}},
	}}}, nil)

	publications, err := application.TrackedPublications()
	if err != nil {
		t.Fatalf("TrackedPublications() error = %v", err)
	}
	if len(publications) != 2 {
		t.Fatalf("TrackedPublications() = %+v, want 2 entries", publications)
	}
	if publications[0].ArtifactType != dto.ArtifactCharm || publications[1].ArtifactType != dto.ArtifactSnap {
		t.Fatalf("TrackedPublications() order/types = %+v", publications)
	}
}
