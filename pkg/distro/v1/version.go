// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"strings"

	"pault.ag/go/debian/version"
)

// CompareVersions compares two Debian version strings using dpkg semantics.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) int {
	va, err := version.Parse(a)
	if err != nil {
		return strings.Compare(a, b)
	}
	vb, err := version.Parse(b)
	if err != nil {
		return strings.Compare(a, b)
	}
	return version.Compare(va, vb)
}

// StripDebianRevision removes the Debian revision from a version string.
// For example, "3:32.0.0-0ubuntu1" becomes "3:32.0.0".
func StripDebianRevision(v string) string {
	parsed, err := version.Parse(v)
	if err != nil {
		return v
	}
	parsed.Revision = ""
	return parsed.String()
}

// PickHighest returns the source package with the highest version from a slice.
// Returns nil if the slice is empty.
func PickHighest(versions []SourcePackage) *SourcePackage {
	if len(versions) == 0 {
		return nil
	}
	best := &versions[0]
	for i := 1; i < len(versions); i++ {
		if CompareVersions(versions[i].Version, best.Version) > 0 {
			best = &versions[i]
		}
	}
	return best
}
