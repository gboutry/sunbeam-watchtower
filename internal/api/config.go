// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
)

// --- Request / Response types ------------------------------------------------

// ConfigOutput is the response for GET /api/v1/config.
type ConfigOutput struct {
	Body *config.Config
}

// --- Route registration ------------------------------------------------------

// RegisterConfigAPI registers configuration-related endpoints on the given huma API.
func RegisterConfigAPI(api huma.API, application *app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "config-show",
		Method:      http.MethodGet,
		Path:        "/api/v1/config",
		Summary:     "Return the loaded configuration",
		Tags:        []string{"config"},
	}, func(_ context.Context, _ *struct{}) (*ConfigOutput, error) {
		if application.Config == nil {
			return nil, huma.Error500InternalServerError("no configuration loaded")
		}
		return &ConfigOutput{Body: application.Config}, nil
	})
}
