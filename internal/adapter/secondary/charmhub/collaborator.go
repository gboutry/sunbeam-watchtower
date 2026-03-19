// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package charmhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

var charmhubBaseURL = "https://api.charmhub.io"

// Compile-time interface compliance check.
var _ port.StoreCollaboratorManager = (*CollaboratorManager)(nil)

// CollaboratorManager implements port.StoreCollaboratorManager for Charmhub.
type CollaboratorManager struct {
	baseURL string
	auth    string // macaroon auth header value
	client  *http.Client
}

// NewCollaboratorManager creates a CollaboratorManager for Charmhub.
// An optional *http.Client may be provided; if omitted a default client is used.
func NewCollaboratorManager(auth string, clients ...*http.Client) *CollaboratorManager {
	var client *http.Client
	if len(clients) > 0 && clients[0] != nil {
		client = clients[0]
	} else {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &CollaboratorManager{
		baseURL: charmhubBaseURL,
		auth:    auth,
		client:  client,
	}
}

// ListCollaborators returns all collaborators for the named charm.
func (m *CollaboratorManager) ListCollaborators(ctx context.Context, storeName string) ([]dto.StoreCollaborator, error) {
	endpoint := m.baseURL + "/v1/charm/" + url.PathEscape(storeName) + "/collaborators"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating list collaborators request: %w", err)
	}
	req.Header.Set("Authorization", "Macaroon "+m.auth)
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing collaborators: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing collaborators: HTTP %d", resp.StatusCode)
	}

	var payload charmCollaboratorsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding collaborators response: %w", err)
	}

	result := make([]dto.StoreCollaborator, 0, len(payload.Collaborators))
	for _, c := range payload.Collaborators {
		result = append(result, dto.StoreCollaborator{
			Username:    c.Username,
			Email:       c.Email,
			DisplayName: c.DisplayName,
			Status:      c.Status,
		})
	}
	return result, nil
}

// InviteCollaborator sends a collaborator invitation for the named charm.
func (m *CollaboratorManager) InviteCollaborator(ctx context.Context, storeName string, email string) error {
	endpoint := m.baseURL + "/v1/charm/" + url.PathEscape(storeName) + "/collaborators"

	body, err := json.Marshal(map[string]string{"email": email})
	if err != nil {
		return fmt.Errorf("encoding invite request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating invite collaborator request: %w", err)
	}
	req.Header.Set("Authorization", "Macaroon "+m.auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("inviting collaborator: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("inviting collaborator: HTTP %d", resp.StatusCode)
	}
	return nil
}

type charmCollaboratorsResponse struct {
	Collaborators []charmCollaborator `json:"collaborators"`
}

type charmCollaborator struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display-name"`
	Status      string `json:"status"`
}
