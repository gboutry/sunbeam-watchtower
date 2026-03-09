// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTailFileLinesReturnsLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchtower.log")
	content := "one\ntwo\nthree\nfour\nfive\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	lines, err := TailFileLines(path, 2)
	if err != nil {
		t.Fatalf("TailFileLines() error = %v", err)
	}
	if len(lines) != 2 || lines[0] != "four" || lines[1] != "five" {
		t.Fatalf("TailFileLines() = %v, want [four five]", lines)
	}
}

func TestTailFileLinesMissingFile(t *testing.T) {
	if _, err := TailFileLines(filepath.Join(t.TempDir(), "missing.log"), 10); err == nil {
		t.Fatal("TailFileLines() error = nil, want missing file error")
	}
}
