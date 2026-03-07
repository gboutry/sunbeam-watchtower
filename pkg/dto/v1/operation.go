// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import "time"

// OperationKind identifies a long-running frontend-facing workflow.
type OperationKind string

const (
	OperationKindBuildTrigger OperationKind = "build.trigger"
	OperationKindProjectSync  OperationKind = "project.sync"
)

// OperationState describes the lifecycle state of an operation job.
type OperationState string

const (
	OperationStateQueued      OperationState = "queued"
	OperationStateRunning     OperationState = "running"
	OperationStateInterrupted OperationState = "interrupted"
	OperationStateSucceeded   OperationState = "succeeded"
	OperationStateFailed      OperationState = "failed"
	OperationStateCancelled   OperationState = "cancelled"
)

// OperationProgress captures the latest progress snapshot for a job.
type OperationProgress struct {
	Phase         string `json:"phase,omitempty" yaml:"phase,omitempty"`
	Message       string `json:"message,omitempty" yaml:"message,omitempty"`
	Current       int    `json:"current,omitempty" yaml:"current,omitempty"`
	Total         int    `json:"total,omitempty" yaml:"total,omitempty"`
	Indeterminate bool   `json:"indeterminate,omitempty" yaml:"indeterminate,omitempty"`
}

// OperationEvent records a time-ordered event emitted by a running job.
type OperationEvent struct {
	Time     time.Time          `json:"time" yaml:"time"`
	Type     string             `json:"type" yaml:"type"`
	Message  string             `json:"message,omitempty" yaml:"message,omitempty"`
	Error    string             `json:"error,omitempty" yaml:"error,omitempty"`
	Progress *OperationProgress `json:"progress,omitempty" yaml:"progress,omitempty"`
}

// OperationJob is the frontend-facing snapshot of a long-running job.
type OperationJob struct {
	ID          string             `json:"id" yaml:"id"`
	Kind        OperationKind      `json:"kind" yaml:"kind"`
	State       OperationState     `json:"state" yaml:"state"`
	CreatedAt   time.Time          `json:"created_at" yaml:"created_at"`
	StartedAt   time.Time          `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	FinishedAt  time.Time          `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	Summary     string             `json:"summary,omitempty" yaml:"summary,omitempty"`
	Error       string             `json:"error,omitempty" yaml:"error,omitempty"`
	Cancellable bool               `json:"cancellable" yaml:"cancellable"`
	Progress    *OperationProgress `json:"progress,omitempty" yaml:"progress,omitempty"`
	Attributes  map[string]string  `json:"attributes,omitempty" yaml:"attributes,omitempty"`
}
