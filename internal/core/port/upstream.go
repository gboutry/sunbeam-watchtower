// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package port

import (
	"context"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// UpstreamProvider resolves upstream package versions for a set of packages.
type UpstreamProvider interface {
	ListDeliverables(ctx context.Context, release string) ([]dto.Deliverable, error)
	GetConstraints(ctx context.Context, release string) (map[string]string, error)
	MapPackageName(deliverable string, dtype dto.DeliverableType) string
}
