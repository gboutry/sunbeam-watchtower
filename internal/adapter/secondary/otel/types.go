// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"time"
)

type SnapshotSource struct {
	AuthSnapshot      func(context.Context) (*AuthSnapshot, error)
	OperationSnapshot func(context.Context) (*OperationSnapshot, error)
	ProjectSnapshot   func(context.Context) (*ProjectSnapshot, error)
	BuildSnapshot     func(context.Context) (*BuildSnapshot, error)
	ReleaseSnapshot   func(context.Context) (*ReleaseSnapshot, error)
	ReviewSnapshot    func(context.Context) (*ReviewSnapshot, error)
	CommitSnapshot    func(context.Context) (*CommitSnapshot, error)
	BugSnapshot       func(context.Context) (*BugSnapshot, error)
	PackageSnapshot   func(context.Context) (*PackageSnapshot, error)
	ExcusesSnapshot   func(context.Context) (*ExcusesSnapshot, error)
	CacheSnapshot     func(context.Context) (*CacheSnapshot, error)
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
