// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"time"
)

type SnapshotSource interface {
	AuthSnapshot(context.Context) (*AuthSnapshot, error)
	OperationSnapshot(context.Context) (*OperationSnapshot, error)
	ProjectSnapshot(context.Context) (*ProjectSnapshot, error)
	BuildSnapshot(context.Context) (*BuildSnapshot, error)
	ReleaseSnapshot(context.Context) (*ReleaseSnapshot, error)
	ReviewSnapshot(context.Context) (*ReviewSnapshot, error)
	CommitSnapshot(context.Context) (*CommitSnapshot, error)
	BugSnapshot(context.Context) (*BugSnapshot, error)
	PackageSnapshot(context.Context) (*PackageSnapshot, error)
	ExcusesSnapshot(context.Context) (*ExcusesSnapshot, error)
	CacheSnapshot(context.Context) (*CacheSnapshot, error)
}

type AuthMetric struct {
	Provider      string
	Authenticated bool
}

type AuthSnapshot struct {
	Providers []AuthMetric
}

type OperationMetric struct {
	Kind      string
	State     string
	Count     int
	OldestAge time.Duration
}

type OperationSnapshot struct {
	Operations []OperationMetric
}

type ProjectMetric struct {
	Project      string
	Forge        string
	ArtifactType string
	RepoCached   bool
}

type ProjectSnapshot struct {
	Projects []ProjectMetric
}

type BuildMetric struct {
	Project      string
	ArtifactType string
	Backend      string
	State        string
	Count        int
	OldestAge    time.Duration
}

type BuildSnapshot struct {
	Builds []BuildMetric
}

type ReleaseTargetMetric struct {
	Project      string
	ArtifactType string
	Artifact     string
	Track        string
	Risk         string
	Branch       string
	Architecture string
	Revision     int
	ReleasedAt   time.Time
}

type ReleaseResourceMetric struct {
	Project      string
	ArtifactType string
	Artifact     string
	Track        string
	Risk         string
	Branch       string
	Resource     string
	Revision     int
}

type ReleaseSnapshot struct {
	Targets   []ReleaseTargetMetric
	Resources []ReleaseResourceMetric
}

type ReviewMetric struct {
	Project   string
	Forge     string
	State     string
	Count     int
	OldestAge time.Duration
}

type ReviewSnapshot struct {
	Reviews []ReviewMetric
}

type CommitMetric struct {
	Project           string
	MergeRequestState string
	HasBugRef         string
	Count             int
}

type CommitSnapshot struct {
	Commits []CommitMetric
}

type BugMetric struct {
	Project  string
	Forge    string
	Assigned string
	Count    int
}

type BugSnapshot struct {
	Bugs []BugMetric
}

type PackageMetric struct {
	Source    string
	Distro    string
	Release   string
	Component string
	Count     int
}

type PackageSnapshot struct {
	Packages []PackageMetric
}

type ExcusesMetric struct {
	Tracker string
	Count   int
}

type ExcusesSnapshot struct {
	Trackers []ExcusesMetric
}

type CacheMetric struct {
	Kind        string
	Scope       string
	Entries     int
	LastUpdated time.Time
}

type CacheSnapshot struct {
	Caches []CacheMetric
}
