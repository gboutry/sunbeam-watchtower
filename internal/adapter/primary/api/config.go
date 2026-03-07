// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// ConfigOutput is the response for GET /api/v1/config.
type ConfigOutput struct {
	Body *dto.Config
}

// RegisterConfigAPI registers configuration-related endpoints on the given huma API.
func RegisterConfigAPI(api huma.API, application *app.App) {
	facade := frontend.NewServerFacade(application)

	huma.Register(api, huma.Operation{
		OperationID: "config-show",
		Method:      http.MethodGet,
		Path:        "/api/v1/config",
		Summary:     "Return the loaded configuration",
		Tags:        []string{"config"},
	}, func(ctx context.Context, _ *struct{}) (*ConfigOutput, error) {
		result, err := facade.Config().Show(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return &ConfigOutput{Body: result}, nil
	})
}
