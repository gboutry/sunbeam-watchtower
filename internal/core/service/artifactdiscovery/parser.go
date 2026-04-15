// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package artifactdiscovery provides the canonical primitives for identifying
// artifact manifests (charmcraft.yaml, snapcraft.yaml, rockcraft.yaml) and
// extracting declared metadata from them. It is the single source of truth
// for "what is an artifact manifest" across build preparation and server-side
// discovery.
package artifactdiscovery

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// manifestHeader is the minimal subset of an artifact manifest that this
// package inspects. Other fields are deliberately ignored so the parser stays
// tolerant of craft-tool schema additions.
type manifestHeader struct {
	Name string `yaml:"name"`
}

// ParseManifestName parses a charmcraft/snapcraft/rockcraft YAML payload and
// returns the declared `name` field. It returns an empty string (and no
// error) when the manifest omits a name, so callers can apply their own
// fallback policy (for example, build prepare falls back to the directory
// base name). Malformed YAML yields a non-nil error whose message includes
// filename for context.
func ParseManifestName(content []byte, filename string) (string, error) {
	var m manifestHeader
	if err := yaml.Unmarshal(content, &m); err != nil {
		return "", fmt.Errorf("parse %s: %w", filename, err)
	}
	return m.Name, nil
}
