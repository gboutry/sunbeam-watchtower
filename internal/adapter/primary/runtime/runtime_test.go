// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"context"
	"testing"
)

func TestNewSession_DefaultsToEmbedded(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	session, err := NewSession(context.Background(), Options{LogWriter: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer session.Close()

	if got := session.Target().Kind; got != TargetKindEmbedded {
		t.Fatalf("Target().Kind = %q, want %q", got, TargetKindEmbedded)
	}
	if session.Client == nil {
		t.Fatal("session.Client = nil")
	}
	if session.Frontend == nil {
		t.Fatal("session.Frontend = nil")
	}
}

func TestNewSession_UsesRemoteTargetWhenProvided(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	session, err := NewSession(context.Background(), Options{
		ServerAddr: "http://127.0.0.1:9999",
		LogWriter:  &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer session.Close()

	if got := session.Target().Kind; got != TargetKindRemote {
		t.Fatalf("Target().Kind = %q, want %q", got, TargetKindRemote)
	}
	if got := session.Target().Address; got != "http://127.0.0.1:9999" {
		t.Fatalf("Target().Address = %q", got)
	}
}
