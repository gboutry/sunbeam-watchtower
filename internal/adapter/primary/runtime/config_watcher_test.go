// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigWatcherTriggersOnFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watchtower.yaml")
	if err := os.WriteFile(path, []byte("initial"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	called := make(chan string, 1)
	cw, err := NewConfigWatcher(path, func(p string) error {
		called <- p
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("NewConfigWatcher() error = %v", err)
	}
	defer cw.Stop()

	if err := os.WriteFile(path, []byte("updated"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	select {
	case got := <-called:
		if got != path {
			t.Fatalf("callback path = %q, want %q", got, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callback was not called within timeout")
	}
}

func TestConfigWatcherCallbackErrorDoesNotCrash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watchtower.yaml")
	if err := os.WriteFile(path, []byte("initial"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	errCalled := make(chan struct{}, 1)
	cw, err := NewConfigWatcher(path, func(p string) error {
		errCalled <- struct{}{}
		return errors.New("reload failed")
	}, nil)
	if err != nil {
		t.Fatalf("NewConfigWatcher() error = %v", err)
	}
	defer cw.Stop()

	if err := os.WriteFile(path, []byte("bad config"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	select {
	case <-errCalled:
		// Watcher is still alive; write again to confirm it didn't crash.
	case <-time.After(2 * time.Second):
		t.Fatal("callback was not called within timeout")
	}

	if err := os.WriteFile(path, []byte("another change"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	select {
	case <-errCalled:
		// Second callback confirms watcher survived the first error.
	case <-time.After(2 * time.Second):
		t.Fatal("second callback was not called — watcher may have crashed")
	}
}
