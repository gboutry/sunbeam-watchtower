// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import "context"

// DeliverableType classifies upstream project deliverables.
type DeliverableType int

const (
	DeliverableService DeliverableType = iota
	DeliverableLibrary
	DeliverableClient
	DeliverableOther
)

func (d DeliverableType) String() string {
	switch d {
	case DeliverableService:
		return "service"
	case DeliverableLibrary:
		return "library"
	case DeliverableClient:
		return "client"
	default:
		return "other"
	}
}

// Deliverable represents an upstream project with its latest version.
type Deliverable struct {
	Name    string          `json:"name" yaml:"name"`
	Type    DeliverableType `json:"type" yaml:"type"`
	Version string          `json:"version" yaml:"version"`
	Team    string          `json:"team,omitempty" yaml:"team,omitempty"`
}

// UpstreamProvider resolves upstream package versions for a set of packages.
// Implementations may fetch data from git repos, APIs, or other sources.
type UpstreamProvider interface {
	// ListDeliverables returns known deliverables for the given release.
	ListDeliverables(ctx context.Context, release string) ([]Deliverable, error)

	// GetConstraints returns upper version constraints for a release.
	// The map keys are package names, values are version strings.
	GetConstraints(ctx context.Context, release string) (map[string]string, error)

	// MapPackageName maps an upstream deliverable name to a distro source package name.
	MapPackageName(deliverable string, dtype DeliverableType) string
}
