// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import "regexp"

// bugRefPatterns matches LP bug references in commit messages.
var bugRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`LP: #(\d+)`),
	regexp.MustCompile(`(?i)Closes-Bug:\s*#?(\d+)`),
	regexp.MustCompile(`(?i)Partial-Bug:\s*#?(\d+)`),
	regexp.MustCompile(`(?i)Related-Bug:\s*#?(\d+)`),
}

// ExtractBugRefs parses a commit message for LP bug references.
func ExtractBugRefs(message string) []string {
	seen := make(map[string]bool)
	var refs []string

	for _, pat := range bugRefPatterns {
		matches := pat.FindAllStringSubmatch(message, -1)
		for _, m := range matches {
			if len(m) >= 2 && !seen[m[1]] {
				seen[m[1]] = true
				refs = append(refs, m[1])
			}
		}
	}

	return refs
}
