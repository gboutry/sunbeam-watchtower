// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package appclient

import (
	"context"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
)

// ConfigShow returns the loaded configuration.
func (c *Client) ConfigShow(ctx context.Context) (*config.Config, error) {
	var result config.Config
	err := c.get(ctx, "/api/v1/config", nil, &result)
	return &result, err
}
