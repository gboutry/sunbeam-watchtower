// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package snapstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

var infoEndpoint = "https://api.snapcraft.io/v2/snaps/info/"

// Source fetches public snap release information from the Snap Store.
type Source struct {
	client *http.Client
}

// NewSource creates a snap release source.
func NewSource(client *http.Client) *Source {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Source{client: client}
}

func (s *Source) ArtifactType() dto.ArtifactType { return dto.ArtifactSnap }

// Fetch returns the current published snap channel state.
func (s *Source) Fetch(ctx context.Context, publication dto.TrackedPublication) (*dto.PublishedArtifactSnapshot, error) {
	fields := url.Values{}
	fields.Set("fields", "channel-map")
	endpoint := infoEndpoint + url.PathEscape(publication.Name) + "?" + fields.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating snap info request: %w", err)
	}
	req.Header.Set("Snap-Device-Series", "16")
	req.Header.Set("Snap-Device-Architecture", "amd64")
	req.Header.Set("Snap-Classic", "true")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching snap info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching snap info: HTTP %d", resp.StatusCode)
	}

	var payload snapInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding snap info: %w", err)
	}

	byChannel := make(map[string]*dto.ReleaseChannelSnapshot)
	trackSet := make(map[string]bool, len(publication.Tracks))
	for _, track := range publication.Tracks {
		trackSet[track] = true
	}
	for _, entry := range payload.ChannelMap {
		if len(trackSet) > 0 && !trackSet[entry.Channel.Track] {
			continue
		}
		channelName := entry.Channel.Name
		channel := byChannel[channelName]
		if channel == nil {
			channel = &dto.ReleaseChannelSnapshot{
				Track:     entry.Channel.Track,
				Risk:      dto.ReleaseRisk(entry.Channel.Risk),
				Channel:   channelName,
				UpdatedAt: entry.Channel.ReleasedAt,
			}
			byChannel[channelName] = channel
		}
		target := dto.ReleaseTargetSnapshot{
			Architecture: entry.Channel.Architecture,
			Base: dto.ReleaseBase{
				Architecture: entry.Channel.Architecture,
			},
			Revision:   entry.Revision,
			Version:    entry.Version,
			ReleasedAt: entry.Channel.ReleasedAt,
		}
		channel.Targets = append(channel.Targets, target)
		if target.ReleasedAt.After(channel.UpdatedAt) {
			channel.UpdatedAt = target.ReleasedAt
		}
	}

	channels := make([]dto.ReleaseChannelSnapshot, 0, len(byChannel))
	for _, channel := range byChannel {
		channels = append(channels, dto.NormalizeChannel(*channel))
	}
	sort.Slice(channels, func(i, j int) bool {
		if channels[i].Track == channels[j].Track {
			return channels[i].Channel < channels[j].Channel
		}
		return channels[i].Track < channels[j].Track
	})

	tracks := append([]string(nil), publication.Tracks...)
	if len(tracks) == 0 {
		for _, channel := range channels {
			tracks = append(tracks, channel.Track)
		}
	}

	return &dto.PublishedArtifactSnapshot{
		Project:      publication.Project,
		Name:         publication.Name,
		ArtifactType: dto.ArtifactSnap,
		Tracks:       tracks,
		Channels:     channels,
		UpdatedAt:    time.Now().UTC(),
	}, nil
}

type snapInfoResponse struct {
	ChannelMap []struct {
		Channel struct {
			Architecture string    `json:"architecture"`
			Name         string    `json:"name"`
			ReleasedAt   time.Time `json:"released-at"`
			Risk         string    `json:"risk"`
			Track        string    `json:"track"`
		} `json:"channel"`
		Revision int    `json:"revision"`
		Version  string `json:"version"`
	} `json:"channel-map"`
}
