// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package distrocache

import (
	"strings"
	"testing"
)

const sampleSources = `Package: nova
Binary: nova-api, nova-compute, python3-nova
Version: 1:29.1.0-0ubuntu1
Architecture: any all
Format: 3.0 (quilt)

Package: keystone
Binary: keystone, python3-keystone
Version: 2:27.0.1-0ubuntu1
Architecture: any all
Format: 3.0 (quilt)
`

func TestParseSources(t *testing.T) {
	r := strings.NewReader(sampleSources)
	pkgs, err := ParseSources(r, "noble-updates", "main")
	if err != nil {
		t.Fatal(err)
	}

	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}

	if pkgs[0].Package != "nova" || pkgs[0].Version != "1:29.1.0-0ubuntu1" {
		t.Errorf("unexpected first package: %+v", pkgs[0])
	}
	if pkgs[0].Suite != "noble-updates" || pkgs[0].Component != "main" {
		t.Errorf("unexpected suite/component: %+v", pkgs[0])
	}

	if pkgs[1].Package != "keystone" || pkgs[1].Version != "2:27.0.1-0ubuntu1" {
		t.Errorf("unexpected second package: %+v", pkgs[1])
	}
}

func TestParseSourcesNoTrailingNewline(t *testing.T) {
	src := "Package: glance\nVersion: 1:28.0.0-1\n"
	pkgs, err := ParseSources(strings.NewReader(src), "trixie", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Package != "glance" {
		t.Errorf("expected glance, got %q", pkgs[0].Package)
	}
}

func TestParseSourcesContinuationLines(t *testing.T) {
	src := `Package: neutron
Version: 2:24.0.0-1
Description: OpenStack Networking
 This is a long description
 that spans multiple lines.

`
	pkgs, err := ParseSources(strings.NewReader(src), "noble", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Package != "neutron" || pkgs[0].Version != "2:24.0.0-1" {
		t.Errorf("unexpected package: %+v", pkgs[0])
	}
}

const sampleSourcesWithBuildDeps = `Package: nova
Version: 1:29.1.0-0ubuntu1
Build-Depends: debhelper-compat (= 13),
               dh-python,
               python3-all,
               python3-setuptools

Package: keystone
Version: 2:27.0.1-0ubuntu1
Build-Depends: debhelper-compat (= 13), python3-oslo.config (>= 1:9.0.0)
`

func TestParseSourcesDetailed(t *testing.T) {
	r := strings.NewReader(sampleSourcesWithBuildDeps)
	pkgs, err := ParseSourcesDetailed(r, "noble", "main")
	if err != nil {
		t.Fatal(err)
	}

	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}

	// First package: nova with multi-line Build-Depends.
	if pkgs[0].Package != "nova" || pkgs[0].Version != "1:29.1.0-0ubuntu1" {
		t.Errorf("unexpected first package: %+v", pkgs[0])
	}
	expectedDeps := []string{"debhelper-compat", "dh-python", "python3-all", "python3-setuptools"}
	if len(pkgs[0].BuildDepends) != len(expectedDeps) {
		t.Fatalf("expected %d build deps for nova, got %d: %v", len(expectedDeps), len(pkgs[0].BuildDepends), pkgs[0].BuildDepends)
	}
	for i, dep := range expectedDeps {
		if pkgs[0].BuildDepends[i] != dep {
			t.Errorf("build dep %d: expected %q, got %q", i, dep, pkgs[0].BuildDepends[i])
		}
	}

	// Second package: keystone with single-line Build-Depends.
	if pkgs[1].Package != "keystone" {
		t.Errorf("expected keystone, got %q", pkgs[1].Package)
	}
	if len(pkgs[1].BuildDepends) != 2 {
		t.Fatalf("expected 2 build deps for keystone, got %d: %v", len(pkgs[1].BuildDepends), pkgs[1].BuildDepends)
	}
	if pkgs[1].BuildDepends[0] != "debhelper-compat" || pkgs[1].BuildDepends[1] != "python3-oslo.config" {
		t.Errorf("unexpected keystone build deps: %v", pkgs[1].BuildDepends)
	}
}

const sampleSourcesWithFiles = `Package: nova
Version: 1:29.1.0-0ubuntu1
Directory: pool/main/n/nova
Files:
 abc123 1234 nova_29.1.0-0ubuntu1.dsc
 def456 5678 nova_29.1.0.orig.tar.gz
 ghi789 9012 nova_29.1.0-0ubuntu1.debian.tar.xz

Package: keystone
Version: 2:27.0.1-0ubuntu1
Directory: pool/main/k/keystone
Files:
 aaa111 100 keystone_27.0.1-0ubuntu1.dsc
 bbb222 200 keystone_27.0.1.orig.tar.gz
`

func TestParseSourcesWithFiles(t *testing.T) {
	r := strings.NewReader(sampleSourcesWithFiles)
	pkgs, err := ParseSourcesWithFiles(r, "noble-updates", "main")
	if err != nil {
		t.Fatal(err)
	}

	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}

	// First package: nova.
	if pkgs[0].Package != "nova" || pkgs[0].Version != "1:29.1.0-0ubuntu1" {
		t.Errorf("unexpected first package: %+v", pkgs[0])
	}
	if pkgs[0].Directory != "pool/main/n/nova" {
		t.Errorf("unexpected directory: %q", pkgs[0].Directory)
	}
	if len(pkgs[0].Files) != 3 {
		t.Fatalf("expected 3 files for nova, got %d: %v", len(pkgs[0].Files), pkgs[0].Files)
	}
	expectedFiles := []string{
		"nova_29.1.0-0ubuntu1.dsc",
		"nova_29.1.0.orig.tar.gz",
		"nova_29.1.0-0ubuntu1.debian.tar.xz",
	}
	for i, f := range expectedFiles {
		if pkgs[0].Files[i] != f {
			t.Errorf("file %d: expected %q, got %q", i, f, pkgs[0].Files[i])
		}
	}

	// Second package: keystone.
	if pkgs[1].Package != "keystone" || pkgs[1].Version != "2:27.0.1-0ubuntu1" {
		t.Errorf("unexpected second package: %+v", pkgs[1])
	}
	if pkgs[1].Directory != "pool/main/k/keystone" {
		t.Errorf("unexpected directory: %q", pkgs[1].Directory)
	}
	if len(pkgs[1].Files) != 2 {
		t.Fatalf("expected 2 files for keystone, got %d: %v", len(pkgs[1].Files), pkgs[1].Files)
	}
}

func TestParseSourcesWithFilesNoTrailingNewline(t *testing.T) {
	src := "Package: glance\nVersion: 1:28.0.0-1\nDirectory: pool/main/g/glance\nFiles:\n abc 100 glance_28.0.0-1.dsc\n"
	pkgs, err := ParseSourcesWithFiles(strings.NewReader(src), "trixie", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].Directory != "pool/main/g/glance" {
		t.Errorf("unexpected directory: %q", pkgs[0].Directory)
	}
	if len(pkgs[0].Files) != 1 || pkgs[0].Files[0] != "glance_28.0.0-1.dsc" {
		t.Errorf("unexpected files: %v", pkgs[0].Files)
	}
}
