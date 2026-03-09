// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"
	"sync"
)

const sessionLogLineLimit = 500

type logBuffer struct {
	mu      sync.Mutex
	lines   []string
	partial string
	limit   int
}

func newLogBuffer(limit int) *logBuffer {
	if limit <= 0 {
		limit = sessionLogLineLimit
	}
	return &logBuffer{limit: limit}
}

func (b *logBuffer) Write(p []byte) (int, error) {
	if b == nil {
		return len(p), nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	text := b.partial + strings.ReplaceAll(string(p), "\r\n", "\n")
	parts := strings.Split(text, "\n")
	if len(parts) == 0 {
		return len(p), nil
	}
	b.partial = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		b.appendLine(line)
	}
	return len(p), nil
}

func (b *logBuffer) Snapshot() []string {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	lines := append([]string(nil), b.lines...)
	if strings.TrimSpace(b.partial) != "" {
		lines = append(lines, b.partial)
	}
	return lines
}

func (b *logBuffer) appendLine(line string) {
	b.lines = append(b.lines, line)
	if extra := len(b.lines) - b.limit; extra > 0 {
		b.lines = append([]string(nil), b.lines[extra:]...)
	}
}
