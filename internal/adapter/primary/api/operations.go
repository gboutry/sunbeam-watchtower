package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// OperationInput identifies one operation resource.
type OperationInput struct {
	ID string `path:"id" doc:"Operation job ID"`
}

// OperationOutput is the response for one operation snapshot.
type OperationOutput struct {
	Body dto.OperationJob
}

// OperationsListOutput is the response for listing operations.
type OperationsListOutput struct {
	Body struct {
		Jobs []dto.OperationJob `json:"jobs" doc:"Known long-running operations"`
	}
}

// OperationEventsOutput is the response for listing operation events.
type OperationEventsOutput struct {
	Body struct {
		Events []dto.OperationEvent `json:"events" doc:"Recorded operation events"`
	}
}

// RegisterOperationsAPI registers the /api/v1/operations endpoints.
func RegisterOperationsAPI(api huma.API, application *app.App) {
	operations := frontend.NewOperationWorkflow(application)

	huma.Register(api, huma.Operation{
		OperationID: "list-operations",
		Method:      http.MethodGet,
		Path:        "/api/v1/operations",
		Summary:     "List long-running operations",
		Tags:        []string{"operations"},
	}, func(ctx context.Context, _ *struct{}) (*OperationsListOutput, error) {
		jobs, err := operations.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to list operations: %v", err))
		}

		out := &OperationsListOutput{}
		out.Body.Jobs = jobs
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-operation",
		Method:      http.MethodGet,
		Path:        "/api/v1/operations/{id}",
		Summary:     "Get one operation",
		Tags:        []string{"operations"},
	}, func(ctx context.Context, input *OperationInput) (*OperationOutput, error) {
		job, err := operations.Get(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to load operation: %v", err))
		}
		if job == nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("operation %q not found", input.ID))
		}

		out := &OperationOutput{}
		out.Body = *job
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-operation-events",
		Method:      http.MethodGet,
		Path:        "/api/v1/operations/{id}/events",
		Summary:     "List operation events",
		Tags:        []string{"operations"},
	}, func(ctx context.Context, input *OperationInput) (*OperationEventsOutput, error) {
		job, err := operations.Get(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to load operation: %v", err))
		}
		if job == nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("operation %q not found", input.ID))
		}

		events, err := operations.Events(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to load operation events: %v", err))
		}

		out := &OperationEventsOutput{}
		out.Body.Events = events
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "cancel-operation",
		Method:      http.MethodPost,
		Path:        "/api/v1/operations/{id}/cancel",
		Summary:     "Cancel one operation",
		Tags:        []string{"operations"},
	}, func(ctx context.Context, input *OperationInput) (*OperationOutput, error) {
		job, err := operations.Get(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to load operation: %v", err))
		}
		if job == nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("operation %q not found", input.ID))
		}

		if err := operations.Cancel(ctx, input.ID); err != nil {
			return nil, huma.Error409Conflict(fmt.Sprintf("failed to cancel operation: %v", err))
		}

		job, err = operations.Get(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to reload operation: %v", err))
		}

		out := &OperationOutput{}
		out.Body = *job
		return out, nil
	})
}
