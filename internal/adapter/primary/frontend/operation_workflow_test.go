// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"testing"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/operationstore"
	opsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/operation"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestOperationWorkflowStartListGetCancel(t *testing.T) {
	service := opsvc.NewService(operationstore.NewMemoryStore(), discardFrontendLogger())
	workflow := NewOperationWorkflowFromService(service)

	blocked := make(chan struct{})
	job, err := workflow.Start(context.Background(), dto.OperationKindProjectSync, map[string]string{"scope": "test"}, func(ctx context.Context, reporter *opsvc.Reporter) (string, error) {
		reporter.Event("entered runner")
		reporter.Progress(dto.OperationProgress{Phase: "running", Message: "working"})
		close(blocked)
		<-ctx.Done()
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	<-blocked

	jobs, err := workflow.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("List() = %+v, want one job %q", jobs, job.ID)
	}

	got, err := workflow.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil || got.ID != job.ID {
		t.Fatalf("Get() = %+v, want job %q", got, job.ID)
	}

	events, err := workflow.Events(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("Events() returned no events")
	}

	if err := workflow.Cancel(context.Background(), job.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, err = workflow.Get(context.Background(), job.ID)
		if err != nil {
			t.Fatalf("Get() after cancel error = %v", err)
		}
		if got != nil && got.State == dto.OperationStateCancelled {
			if got.Error != context.Canceled.Error() {
				t.Fatalf("cancelled job error = %q, want %q", got.Error, context.Canceled.Error())
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got == nil || got.State != dto.OperationStateCancelled {
		t.Fatalf("job state after cancel = %+v, want cancelled", got)
	}

	events, err = workflow.Events(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Events() after cancel error = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("Events() after cancel = %+v, want recorded lifecycle events", events)
	}
}
