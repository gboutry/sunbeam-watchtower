// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ConfigShow returns the loaded configuration.
func (c *Client) ConfigShow(ctx context.Context) (*dto.Config, error) {
	var result dto.Config
	err := c.get(ctx, "/api/v1/config", nil, &result)
	return &result, err
}
