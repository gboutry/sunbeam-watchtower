// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package artifactdiscovery

import (
	"strings"
	"testing"
)

func TestParseManifestName_Valid(t *testing.T) {
	content := []byte("name: mysql-k8s\nsummary: a charm\n")
	got, err := ParseManifestName(content, "charmcraft.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mysql-k8s" {
		t.Fatalf("got %q, want %q", got, "mysql-k8s")
	}
}

func TestParseManifestName_Missing(t *testing.T) {
	content := []byte("summary: a charm without a name\n")
	got, err := ParseManifestName(content, "charmcraft.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestParseManifestName_Malformed(t *testing.T) {
	content := []byte("name: [unterminated\n")
	_, err := ParseManifestName(content, "charmcraft.yaml")
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), "charmcraft.yaml") {
		t.Fatalf("expected error to include filename, got %v", err)
	}
}

func TestParseManifestName_Unicode(t *testing.T) {
	content := []byte("name: café-øl-日本\n")
	got, err := ParseManifestName(content, "snapcraft.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "café-øl-日本" {
		t.Fatalf("got %q, want %q", got, "café-øl-日本")
	}
}

func TestParseManifestName_EmptyInput(t *testing.T) {
	got, err := ParseManifestName(nil, "rockcraft.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}
