// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"sync"
	"testing"
)

func TestLogBufferRetainsNewestLines(t *testing.T) {
	buf := newLogBuffer(3)
	for i := range 5 {
		if _, err := fmt.Fprintf(buf, "line-%d\n", i); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	got := buf.Snapshot()
	want := []string{"line-2", "line-3", "line-4"}
	if len(got) != len(want) {
		t.Fatalf("len(Snapshot()) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Snapshot()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLogBufferPreservesPartialLineInSnapshot(t *testing.T) {
	buf := newLogBuffer(10)
	if _, err := buf.Write([]byte("one\ntwo")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got := buf.Snapshot()
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("Snapshot() = %v, want [one two]", got)
	}
}

func TestLogBufferConcurrentWrites(t *testing.T) {
	buf := newLogBuffer(100)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = fmt.Fprintf(buf, "line-%d\n", i)
		}(i)
	}
	wg.Wait()
	got := buf.Snapshot()
	if len(got) != 10 {
		t.Fatalf("len(Snapshot()) = %d, want 10 (%v)", len(got), got)
	}
}
