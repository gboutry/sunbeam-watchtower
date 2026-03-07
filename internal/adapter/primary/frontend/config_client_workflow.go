// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ConfigClientWorkflow exposes reusable client-side config workflows for CLI/TUI/MCP frontends.
type ConfigClientWorkflow struct {
	client *client.Client
}

// NewConfigClientWorkflow creates a client-side config workflow.
func NewConfigClientWorkflow(apiClient *client.Client) *ConfigClientWorkflow {
	return &ConfigClientWorkflow{client: apiClient}
}

// Show returns the loaded remote configuration.
func (w *ConfigClientWorkflow) Show(ctx context.Context) (*dto.Config, error) {
	if w.client == nil {
		return nil, errors.New("config client workflow requires an API client")
	}
	return w.client.ConfigShow(ctx)
}
