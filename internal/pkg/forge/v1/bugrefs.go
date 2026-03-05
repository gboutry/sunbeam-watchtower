// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import "regexp"

// BugRefType categorises how a commit references a bug.
type BugRefType int

const (
	BugRefCloses  BugRefType = iota // Closes-Bug or LP: — the commit fixes the bug
	BugRefPartial                   // Partial-Bug — partial fix, work still needed
	BugRefRelated                   // Related-Bug — informational, no status change
)

// BugRef is a typed bug reference extracted from a commit message.
type BugRef struct {
	ID   string
	Type BugRefType
}

// bugRefPattern pairs a regex with its reference type.
type bugRefPattern struct {
	re      *regexp.Regexp
	refType BugRefType
}

// bugRefPatterns matches LP bug references in commit messages.
var bugRefPatterns = []bugRefPattern{
	{regexp.MustCompile(`LP: #(\d+)`), BugRefCloses},
	{regexp.MustCompile(`(?i)Closes-Bug:\s*#?(\d+)`), BugRefCloses},
	{regexp.MustCompile(`(?i)Partial-Bug:\s*#?(\d+)`), BugRefPartial},
	{regexp.MustCompile(`(?i)Related-Bug:\s*#?(\d+)`), BugRefRelated},
}

// ExtractBugRefs parses a commit message for LP bug references.
// When the same bug is referenced multiple times, the strongest type wins
// (Closes > Partial > Related).
func ExtractBugRefs(message string) []BugRef {
	seen := make(map[string]BugRefType)
	var order []string

	for _, pat := range bugRefPatterns {
		matches := pat.re.FindAllStringSubmatch(message, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			id := m[1]
			if existing, ok := seen[id]; ok {
				if pat.refType < existing {
					seen[id] = pat.refType // stronger type wins (lower value = stronger)
				}
			} else {
				seen[id] = pat.refType
				order = append(order, id)
			}
		}
	}

	refs := make([]BugRef, 0, len(order))
	for _, id := range order {
		refs = append(refs, BugRef{ID: id, Type: seen[id]})
	}
	return refs
}
