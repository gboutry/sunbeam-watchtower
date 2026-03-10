// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialsPath(t *testing.T) {
	path := CredentialsPath()
	if path == "" {
		t.Fatal("CredentialsPath() returned empty path")
	}
	if !strings.HasSuffix(path, filepath.Join(".config", "sunbeam-watchtower", credentialFile)) {
		t.Fatalf("CredentialsPath() = %q", path)
	}
}

func TestSaveLoadCredentialsFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github-credentials.json")
	creds := &Credentials{
		AccessToken: "token",
		TokenType:   "bearer",
		Scope:       "repo",
	}

	if err := SaveCredentialsFile(path, creds); err != nil {
		t.Fatalf("SaveCredentialsFile() error = %v", err)
	}
	got, err := LoadCredentialsFile(path)
	if err != nil {
		t.Fatalf("LoadCredentialsFile() error = %v", err)
	}
	if got == nil || got.AccessToken != creds.AccessToken || got.TokenType != creds.TokenType || got.Scope != creds.Scope {
		t.Fatalf("LoadCredentialsFile() = %+v, want %+v", got, creds)
	}
}

func TestLoadCredentialsFileMissingReturnsNil(t *testing.T) {
	got, err := LoadCredentialsFile(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("LoadCredentialsFile() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadCredentialsFile() = %+v, want nil", got)
	}
}

func TestLoadCredentialsFileEmptyTokenReturnsNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github-credentials.json")
	if err := os.WriteFile(path, []byte(`{"token_type":"bearer"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := LoadCredentialsFile(path)
	if err != nil {
		t.Fatalf("LoadCredentialsFile() error = %v", err)
	}
	if got != nil {
		t.Fatalf("LoadCredentialsFile() = %+v, want nil", got)
	}
}
