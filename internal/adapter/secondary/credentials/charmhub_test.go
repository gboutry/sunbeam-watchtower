// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCharmhubStore_DefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	store := NewCharmhubStore("")

	want := filepath.Join(home, ".config", "sunbeam-watchtower", "charmhub-credentials.json")
	if store.path != want {
		t.Fatalf("path = %q, want %q", store.path, want)
	}
}

func TestCharmhubStoreLoad_PrefersEnvironment(t *testing.T) {
	dir := t.TempDir()
	store := NewCharmhubStore(dir)
	if _, err := store.Save(context.Background(), "file-macaroon"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	t.Setenv(envCharmcraftAuth, "env-macaroon")

	record, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if record == nil {
		t.Fatal("Load() returned nil record")
	}
	if record.Source != storeCredentialSourceEnvironment {
		t.Fatalf("Source = %q, want %q", record.Source, storeCredentialSourceEnvironment)
	}
	if record.Macaroon != "env-macaroon" {
		t.Fatalf("Macaroon = %q, want %q", record.Macaroon, "env-macaroon")
	}
}

func TestCharmhubStoreLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	store := NewCharmhubStore(dir)
	if _, err := store.Save(context.Background(), "file-macaroon"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	record, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if record == nil {
		t.Fatal("Load() returned nil record")
	}
	if record.Source != storeCredentialSourceFile {
		t.Fatalf("Source = %q, want %q", record.Source, storeCredentialSourceFile)
	}
	if record.Macaroon != "file-macaroon" {
		t.Fatalf("Macaroon = %q, want %q", record.Macaroon, "file-macaroon")
	}
}

func TestCharmhubStoreLoad_Missing(t *testing.T) {
	dir := t.TempDir()
	store := NewCharmhubStore(dir)

	record, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if record != nil {
		t.Fatalf("Load() = %+v, want nil", record)
	}
}

func TestCharmhubStoreSaveAndClear(t *testing.T) {
	dir := t.TempDir()
	store := NewCharmhubStore(dir)
	path := filepath.Join(dir, charmhubCredentialFile)

	record, err := store.Save(context.Background(), "saved-macaroon")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if record.Path != path {
		t.Fatalf("Path = %q, want %q", record.Path, path)
	}

	if err := store.Clear(context.Background()); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected credentials file to be removed, got err = %v", err)
	}
}
