// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"bytes"
	"compress/gzip"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"os"

	"github.com/ulikunitz/xz"
)

// sampleSources is a two-paragraph Sources fixture exercising continuation
// lines, Build-Depends, Build-Depends-Indep, Directory, and Files.
const sampleSources = `Package: nova
Version: 1:29.0.0-0ubuntu1
Build-Depends: debhelper-compat (= 13),
 python3-all,
 dh-python [amd64] <!nocheck>
Build-Depends-Indep: python3-sphinx,
 python3-openstackdocstheme
Directory: pool/main/n/nova
Files:
 abc123 1234 nova_29.0.0-0ubuntu1.dsc
 def456 5678 nova_29.0.0.orig.tar.gz

Package: keystone
Version: 2:25.0.0-0ubuntu1
Build-Depends: debhelper-compat (= 13)
Directory: pool/main/k/keystone
Files:
 111aaa 100 keystone_25.0.0-0ubuntu1.dsc
`

func TestSourcesFileName(t *testing.T) {
	tests := []struct {
		suite, component, format, want string
	}{
		{"noble", "main", "gz", "noble_main_Sources.gz"},
		{"noble-updates", "universe", "xz", "noble-updates_universe_Sources.xz"},
		{"noble/extra", "main", "gz", "noble_extra_main_Sources.gz"},
	}
	for _, tt := range tests {
		got := SourcesFileName(tt.suite, tt.component, tt.format)
		if got != tt.want {
			t.Errorf("SourcesFileName(%q,%q,%q) = %q, want %q", tt.suite, tt.component, tt.format, got, tt.want)
		}
	}
}

func TestParseSources(t *testing.T) {
	pkgs, err := ParseSources(strings.NewReader(sampleSources), "noble", "main")
	if err != nil {
		t.Fatalf("ParseSources() error = %v", err)
	}
	want := []SourcePackage{
		{Package: "nova", Version: "1:29.0.0-0ubuntu1", Suite: "noble", Component: "main"},
		{Package: "keystone", Version: "2:25.0.0-0ubuntu1", Suite: "noble", Component: "main"},
	}
	if !reflect.DeepEqual(pkgs, want) {
		t.Errorf("ParseSources() = %+v, want %+v", pkgs, want)
	}
}

func TestParseSourcesSkipsIncompleteParagraphs(t *testing.T) {
	// Paragraph with no Version must be dropped.
	input := `Package: orphan

Package: good
Version: 1.0-1
`
	pkgs, err := ParseSources(strings.NewReader(input), "s", "c")
	if err != nil {
		t.Fatalf("ParseSources() error = %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].Package != "good" {
		t.Fatalf("expected only 'good' package, got %+v", pkgs)
	}
}

func TestParseSourcesNoTrailingNewline(t *testing.T) {
	// Final paragraph without a trailing blank line must still flush.
	input := "Package: tail\nVersion: 1.0-1"
	pkgs, err := ParseSources(strings.NewReader(input), "s", "c")
	if err != nil {
		t.Fatalf("ParseSources() error = %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].Package != "tail" {
		t.Fatalf("expected 'tail' package, got %+v", pkgs)
	}
}

func TestParseSourcesDetailed(t *testing.T) {
	details, err := ParseSourcesDetailed(strings.NewReader(sampleSources), "noble", "main")
	if err != nil {
		t.Fatalf("ParseSourcesDetailed() error = %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("got %d details, want 2", len(details))
	}

	// nova: Build-Depends and Build-Depends-Indep merged, arch/profile markers stripped.
	wantNova := []string{"debhelper-compat", "python3-all", "dh-python", "python3-sphinx", "python3-openstackdocstheme"}
	if !reflect.DeepEqual(details[0].BuildDepends, wantNova) {
		t.Errorf("nova BuildDepends = %v, want %v", details[0].BuildDepends, wantNova)
	}

	// keystone: single-line Build-Depends, no Indep.
	wantKeystone := []string{"debhelper-compat"}
	if !reflect.DeepEqual(details[1].BuildDepends, wantKeystone) {
		t.Errorf("keystone BuildDepends = %v, want %v", details[1].BuildDepends, wantKeystone)
	}
}

func TestParseSourcesDetailedIndepOnly(t *testing.T) {
	input := `Package: docs
Version: 1.0-1
Build-Depends-Indep: python3-sphinx,
 python3-openstackdocstheme
`
	details, err := ParseSourcesDetailed(strings.NewReader(input), "s", "c")
	if err != nil {
		t.Fatalf("ParseSourcesDetailed() error = %v", err)
	}
	want := []string{"python3-sphinx", "python3-openstackdocstheme"}
	if !reflect.DeepEqual(details[0].BuildDepends, want) {
		t.Errorf("BuildDepends = %v, want %v", details[0].BuildDepends, want)
	}
}

func TestParseSourcesDetailedNoBuildDepends(t *testing.T) {
	input := `Package: bare
Version: 1.0-1
`
	details, err := ParseSourcesDetailed(strings.NewReader(input), "s", "c")
	if err != nil {
		t.Fatalf("ParseSourcesDetailed() error = %v", err)
	}
	if details[0].BuildDepends != nil {
		t.Errorf("BuildDepends = %v, want nil", details[0].BuildDepends)
	}
}

func TestParseSourcesWithFiles(t *testing.T) {
	got, err := ParseSourcesWithFiles(strings.NewReader(sampleSources), "noble", "main")
	if err != nil {
		t.Fatalf("ParseSourcesWithFiles() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if got[0].Directory != "pool/main/n/nova" {
		t.Errorf("nova Directory = %q", got[0].Directory)
	}
	wantFiles := []string{"nova_29.0.0-0ubuntu1.dsc", "nova_29.0.0.orig.tar.gz"}
	if !reflect.DeepEqual(got[0].Files, wantFiles) {
		t.Errorf("nova Files = %v, want %v", got[0].Files, wantFiles)
	}
	if len(got[1].Files) != 1 || got[1].Files[0] != "keystone_25.0.0-0ubuntu1.dsc" {
		t.Errorf("keystone Files = %v", got[1].Files)
	}
}

func TestParseSourcesFull(t *testing.T) {
	// Paragraph with a multi-line continuation (Description) must preserve it.
	input := `Package: nova
Version: 1:29.0.0-0ubuntu1
Maintainer: Ubuntu Developers
Description: OpenStack Compute
 Nova is the OpenStack compute service.
 It manages virtual machines.
Architecture: all
`
	entries, err := ParseSourcesFull(strings.NewReader(input), "noble", "main")
	if err != nil {
		t.Fatalf("ParseSourcesFull() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	byKey := map[string]string{}
	for _, f := range entries[0].Fields {
		byKey[f.Key] = f.Value
	}
	if byKey["Package"] != "nova" || byKey["Maintainer"] != "Ubuntu Developers" {
		t.Errorf("fields = %+v", byKey)
	}
	if !strings.Contains(byKey["Description"], "OpenStack compute service") {
		t.Errorf("Description continuation lost: %q", byKey["Description"])
	}
	if !strings.Contains(byKey["Description"], "manages virtual machines") {
		t.Errorf("Description continuation lost: %q", byKey["Description"])
	}
}

func TestParseSourcesFullSkipsMalformedLines(t *testing.T) {
	// A line with no ':' must be skipped without error.
	input := "Package: ok\nVersion: 1.0-1\nstray line without colon\n"
	entries, err := ParseSourcesFull(strings.NewReader(input), "s", "c")
	if err != nil {
		t.Fatalf("ParseSourcesFull() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Package != "ok" {
		t.Fatalf("got %+v, want single ok paragraph", entries)
	}
}

func TestDecompressReaderPlain(t *testing.T) {
	r, err := decompressReader(strings.NewReader("hello"), "")
	if err != nil {
		t.Fatalf("decompressReader(plain) error = %v", err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != "hello" {
		t.Errorf("plain = %q, want %q", got, "hello")
	}
}

func TestDecompressReaderGzip(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write([]byte("payload")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	gw.Close()

	r, err := decompressReader(&buf, "gz")
	if err != nil {
		t.Fatalf("decompressReader(gz) error = %v", err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != "payload" {
		t.Errorf("gz = %q", got)
	}
}

func TestDecompressReaderXZ(t *testing.T) {
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatalf("xz writer: %v", err)
	}
	if _, err := xw.Write([]byte("payload")); err != nil {
		t.Fatalf("xz write: %v", err)
	}
	xw.Close()

	r, err := decompressReader(&buf, "xz")
	if err != nil {
		t.Fatalf("decompressReader(xz) error = %v", err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != "payload" {
		t.Errorf("xz = %q", got)
	}
}

func writeGzipFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte(content)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return path
}

func TestParseSourcesFileDetailed(t *testing.T) {
	dir := t.TempDir()
	path := writeGzipFixture(t, dir, "Sources.gz", sampleSources)

	details, err := ParseSourcesFileDetailed(path, "gz", "noble", "main")
	if err != nil {
		t.Fatalf("ParseSourcesFileDetailed() error = %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("got %d details, want 2", len(details))
	}
}

func TestParseSourcesFileWithFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeGzipFixture(t, dir, "Sources.gz", sampleSources)

	entries, err := ParseSourcesFileWithFiles(path, "gz", "noble", "main")
	if err != nil {
		t.Fatalf("ParseSourcesFileWithFiles() error = %v", err)
	}
	if len(entries) != 2 || len(entries[0].Files) != 2 {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestParseSourcesFileFull(t *testing.T) {
	dir := t.TempDir()
	path := writeGzipFixture(t, dir, "Sources.gz", sampleSources)

	entries, err := ParseSourcesFileFull(path, "gz", "noble", "main")
	if err != nil {
		t.Fatalf("ParseSourcesFileFull() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestParseSourcesFileOpenError(t *testing.T) {
	_, err := ParseSourcesFileDetailed(filepath.Join(t.TempDir(), "missing.gz"), "gz", "s", "c")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "opening sources file") {
		t.Errorf("error = %v, want wrapped open error", err)
	}
}

func TestParseSourcesFileDecompressError(t *testing.T) {
	// Plain text under a "gz" format claim must fail to decompress.
	dir := t.TempDir()
	path := filepath.Join(dir, "Sources.gz")
	if err := os.WriteFile(path, []byte("not gzipped"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ParseSourcesFileWithFiles(path, "gz", "s", "c")
	if err == nil {
		t.Fatal("expected decompress error")
	}
	if !strings.Contains(err.Error(), "decompressing") {
		t.Errorf("error = %v, want wrapped decompress error", err)
	}
}
