// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestShellCommandRunner_Run(t *testing.T) {
	dir := t.TempDir()
	runner := &ShellCommandRunner{}

	if err := runner.Run(context.Background(), dir, "echo hello > output.txt"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); got != "hello\n" {
		t.Fatalf("output = %q, want %q", got, "hello\n")
	}
}

func TestShellCommandRunner_RunFailure(t *testing.T) {
	dir := t.TempDir()
	runner := &ShellCommandRunner{}

	err := runner.Run(context.Background(), dir, "false")
	if err == nil {
		t.Fatal("Run() error = nil, want failure")
	}
}

func TestShellCommandRunner_RunContextCancel(t *testing.T) {
	dir := t.TempDir()
	runner := &ShellCommandRunner{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runner.Run(ctx, dir, "sleep 60")
	if err == nil {
		t.Fatal("Run() error = nil, want context cancellation error")
	}
}
