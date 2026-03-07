// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGoFilesSkipsTestsAndExemptions(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "keep.go"), "package sample\n")
	mustWriteFile(t, filepath.Join(dir, "skip_test.go"), "package sample\n")
	mustWriteFile(t, filepath.Join(dir, "exempt.go"), "package sample\n")

	files, err := LoadGoFiles(filepath.Join(dir, "*.go"), map[string]bool{"exempt.go": true})
	if err != nil {
		t.Fatalf("LoadGoFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].Base != "keep.go" {
		t.Fatalf("LoadGoFiles() = %+v, want only keep.go", files)
	}
}

func TestImportAliasesAndExprUsesImport(t *testing.T) {
	file := parseTestFile(t, `package sample
import alias "example.com/client"
type Exported struct {
	Field *alias.Client
}
`)

	aliases := ImportAliases(file, "example.com/client", "client")
	if _, ok := aliases["alias"]; !ok {
		t.Fatalf("ImportAliases() = %+v, want alias", aliases)
	}

	typeSpec := file.Decls[1].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	if !ExportedTypeUsesImport(typeSpec.Type, aliases) {
		t.Fatal("ExportedTypeUsesImport() = false, want true")
	}
}

func TestVisitCollectsTypedNodes(t *testing.T) {
	file := parseTestFile(t, `package sample
func One() {}
func Two() {}
`)

	var names []string
	Visit[*ast.FuncDecl](file, func(fn *ast.FuncDecl) {
		names = append(names, fn.Name.Name)
	})

	if len(names) != 2 || names[0] != "One" || names[1] != "Two" {
		t.Fatalf("Visit() collected %v, want [One Two]", names)
	}
}

func parseTestFile(t *testing.T, src string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	return file
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
