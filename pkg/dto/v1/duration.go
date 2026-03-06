// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// durationPattern matches strings like "2d", "3w", "1w2d", "1w2d3h30m".
var durationPattern = regexp.MustCompile(`^(?:(\d+)w)?(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)

// ParseHumanDuration parses a human-friendly duration string that extends
// Go's time.ParseDuration with support for days (d) and weeks (w).
//
// Accepted formats:
//   - Standard Go durations: "30m", "2h", "1h30m", "90s"
//   - Extended: "2d", "3w", "1w2d", "1w2d3h30m"
//   - Also accepts an ISO 8601 / RFC 3339 timestamp directly
//
// Days are treated as 24h and weeks as 7×24h.
func ParseHumanDuration(s string) (time.Duration, error) {
	// Try standard Go duration first (handles "30m", "2h30m", "90s", etc.)
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	m := durationPattern.FindStringSubmatch(s)
	if m == nil || m[0] == "" {
		return 0, fmt.Errorf("invalid duration %q: use e.g. 30m, 2h, 3d, 1w, 1w2d3h", s)
	}

	var total time.Duration
	if m[1] != "" {
		w, _ := strconv.Atoi(m[1])
		total += time.Duration(w) * 7 * 24 * time.Hour
	}
	if m[2] != "" {
		d, _ := strconv.Atoi(m[2])
		total += time.Duration(d) * 24 * time.Hour
	}
	if m[3] != "" {
		h, _ := strconv.Atoi(m[3])
		total += time.Duration(h) * time.Hour
	}
	if m[4] != "" {
		mins, _ := strconv.Atoi(m[4])
		total += time.Duration(mins) * time.Minute
	}
	if m[5] != "" {
		sec, _ := strconv.Atoi(m[5])
		total += time.Duration(sec) * time.Second
	}

	if total == 0 {
		return 0, fmt.Errorf("invalid duration %q: must be > 0", s)
	}
	return total, nil
}

// ResolveSince takes a --since flag value (human duration like "2d" or ISO 8601
// timestamp) and returns an RFC 3339 timestamp string. If the input parses as a
// duration, the returned time is now minus that duration.
func ResolveSince(s string) (string, error) {
	if s == "" {
		return "", nil
	}

	// Try as an absolute timestamp first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	// Also try date-only format.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}

	// Try as a human duration.
	d, err := ParseHumanDuration(s)
	if err != nil {
		return "", fmt.Errorf("cannot parse --since %q: expected a duration (e.g. 2d, 1w, 30m) or a date (e.g. 2025-01-01)", s)
	}
	return time.Now().Add(-d).UTC().Format(time.RFC3339), nil
}
