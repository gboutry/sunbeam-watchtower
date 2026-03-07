// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package releasecache

import (
	"context"
	"testing"
	"time"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestCacheStoreListStatusAndRemove(t *testing.T) {
	cache, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	defer cache.Close()

	snapshot := dto.PublishedArtifactSnapshot{
		Project:      "sunbeam",
		Name:         "keystone-k8s",
		ArtifactType: dto.ArtifactCharm,
		Tracks:       []string{"2024.1"},
		Channels: []dto.ReleaseChannelSnapshot{{
			Track:   "2024.1",
			Risk:    dto.ReleaseRiskStable,
			Channel: "2024.1/stable",
		}},
		UpdatedAt: time.Now().UTC(),
	}
	if err := cache.Store(context.Background(), snapshot); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	list, err := cache.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Name != "keystone-k8s" {
		t.Fatalf("List() = %+v, want stored snapshot", list)
	}

	status, err := cache.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(status) != 1 || status[0].ChannelCount != 1 || status[0].TrackCount != 1 {
		t.Fatalf("Status() = %+v, want one cached channel", status)
	}

	if err := cache.Remove("sunbeam", "keystone-k8s", dto.ArtifactCharm); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	list, err = cache.List(context.Background())
	if err != nil {
		t.Fatalf("List() after remove error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() after remove = %+v, want empty", list)
	}
}
