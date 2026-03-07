// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package operation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// Func runs one long-running operation and can emit progress events.
type Func func(context.Context, *Reporter) (string, error)

// Service exposes reusable async/progress/event primitives.
type Service struct {
	store  port.OperationStore
	logger *slog.Logger
	now    func() time.Time
	newID  func() (string, error)

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// Reporter emits progress snapshots and events for a running job.
type Reporter struct {
	service *Service
	jobID   string
}

// NewService creates an operation service.
func NewService(store port.OperationStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Service{
		store:   store,
		logger:  logger,
		now:     time.Now,
		newID:   randomID,
		cancels: make(map[string]context.CancelFunc),
	}
}

// Start runs an operation asynchronously and records its lifecycle.
func (s *Service) Start(ctx context.Context, kind dto.OperationKind, attributes map[string]string, run Func) (*dto.OperationJob, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if run == nil {
		return nil, errors.New("operation runner is required")
	}

	jobID, err := s.newID()
	if err != nil {
		return nil, fmt.Errorf("generating operation ID: %w", err)
	}

	createdAt := s.now()
	job := dto.OperationJob{
		ID:          jobID,
		Kind:        kind,
		State:       dto.OperationStateQueued,
		CreatedAt:   createdAt,
		Cancellable: true,
		Attributes:  cloneAttributes(attributes),
	}

	if err := s.store.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("creating operation job: %w", err)
	}
	if err := s.store.AppendEvent(ctx, jobID, dto.OperationEvent{
		Time:    createdAt,
		Type:    "queued",
		Message: "operation queued",
	}); err != nil {
		return nil, fmt.Errorf("recording queued event: %w", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancels[jobID] = cancel
	s.mu.Unlock()

	reporter := &Reporter{service: s, jobID: jobID}
	go s.runJob(runCtx, jobID, kind, reporter, run)

	return s.store.Get(ctx, jobID)
}

// Get returns the latest snapshot for one operation job.
func (s *Service) Get(ctx context.Context, id string) (*dto.OperationJob, error) {
	return s.store.Get(ctx, id)
}

// List returns all tracked operation jobs.
func (s *Service) List(ctx context.Context) ([]dto.OperationJob, error) {
	return s.store.List(ctx)
}

// Events returns the event history for one operation job.
func (s *Service) Events(ctx context.Context, id string) ([]dto.OperationEvent, error) {
	return s.store.Events(ctx, id)
}

// Cancel requests cancellation for a running job.
func (s *Service) Cancel(ctx context.Context, id string) error {
	s.mu.Lock()
	cancel, ok := s.cancels[id]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("operation job %q is not running", id)
	}

	if err := s.store.AppendEvent(ctx, id, dto.OperationEvent{
		Time:    s.now(),
		Type:    "cancel_requested",
		Message: "cancellation requested",
	}); err != nil {
		return fmt.Errorf("recording cancellation request: %w", err)
	}

	cancel()
	return nil
}

// Event records a human-readable event for the current job.
func (r *Reporter) Event(message string) {
	if err := r.service.store.AppendEvent(context.Background(), r.jobID, dto.OperationEvent{
		Time:    r.service.now(),
		Type:    "event",
		Message: message,
	}); err != nil {
		r.service.logger.Warn("failed to append operation event", "job_id", r.jobID, "error", err)
	}
}

// Progress records a progress snapshot for the current job.
func (r *Reporter) Progress(progress dto.OperationProgress) {
	ctx := context.Background()

	job, err := r.service.store.Get(ctx, r.jobID)
	if err != nil {
		r.service.logger.Warn("failed to load operation job for progress", "job_id", r.jobID, "error", err)
		return
	}
	if job == nil {
		return
	}

	job.Progress = cloneProgress(&progress)
	if err := r.service.store.Update(ctx, *job); err != nil {
		r.service.logger.Warn("failed to update operation progress", "job_id", r.jobID, "error", err)
		return
	}

	if err := r.service.store.AppendEvent(ctx, r.jobID, dto.OperationEvent{
		Time:     r.service.now(),
		Type:     "progress",
		Message:  progress.Message,
		Progress: cloneProgress(&progress),
	}); err != nil {
		r.service.logger.Warn("failed to append operation progress event", "job_id", r.jobID, "error", err)
	}
}

func (s *Service) runJob(
	ctx context.Context,
	jobID string,
	kind dto.OperationKind,
	reporter *Reporter,
	run Func,
) {
	startedAt := s.now()
	if err := s.updateJob(context.Background(), jobID, func(job *dto.OperationJob) {
		job.State = dto.OperationStateRunning
		job.StartedAt = startedAt
	}); err != nil {
		s.logger.Warn("failed to mark operation running", "job_id", jobID, "kind", kind, "error", err)
		return
	}

	if err := s.store.AppendEvent(context.Background(), jobID, dto.OperationEvent{
		Time:    startedAt,
		Type:    "started",
		Message: "operation started",
	}); err != nil {
		s.logger.Warn("failed to append operation start event", "job_id", jobID, "kind", kind, "error", err)
	}

	summary, err := run(ctx, reporter)
	s.finishJob(jobID, kind, summary, err)
}

func (s *Service) finishJob(jobID string, kind dto.OperationKind, summary string, runErr error) {
	s.mu.Lock()
	delete(s.cancels, jobID)
	s.mu.Unlock()

	finishedAt := s.now()
	event := dto.OperationEvent{
		Time:    finishedAt,
		Type:    "finished",
		Message: "operation finished",
	}

	state := dto.OperationStateSucceeded
	errorMessage := ""
	switch {
	case runErr == nil:
		event.Type = "succeeded"
		event.Message = summary
	case errors.Is(runErr, context.Canceled):
		state = dto.OperationStateCancelled
		errorMessage = runErr.Error()
		event.Type = "cancelled"
		event.Message = "operation cancelled"
		event.Error = errorMessage
	default:
		state = dto.OperationStateFailed
		errorMessage = runErr.Error()
		event.Type = "failed"
		event.Message = "operation failed"
		event.Error = errorMessage
	}

	if err := s.updateJob(context.Background(), jobID, func(job *dto.OperationJob) {
		job.State = state
		job.FinishedAt = finishedAt
		job.Cancellable = false
		job.Summary = summary
		job.Error = errorMessage
	}); err != nil {
		s.logger.Warn("failed to finish operation job", "job_id", jobID, "kind", kind, "error", err)
		return
	}

	if err := s.store.AppendEvent(context.Background(), jobID, event); err != nil {
		s.logger.Warn("failed to append operation finish event", "job_id", jobID, "kind", kind, "error", err)
	}
}

func (s *Service) updateJob(ctx context.Context, id string, mutate func(*dto.OperationJob)) error {
	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("operation job %q not found", id)
	}

	mutate(job)
	return s.store.Update(ctx, *job)
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func cloneAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(attributes))
	for key, value := range attributes {
		cloned[key] = value
	}
	return cloned
}

func cloneProgress(progress *dto.OperationProgress) *dto.OperationProgress {
	if progress == nil {
		return nil
	}

	cloned := *progress
	return &cloned
}
