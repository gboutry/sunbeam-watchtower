// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

const clientImportPath = "github.com/gboutry/sunbeam-watchtower/pkg/client"

var cliBootstrapFiles = map[string]bool{
	"root.go":    true,
	"runtime.go": true,
}

func TestCommandFilesDoNotImportOrCallPkgClientDirectly(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || cliBootstrapFiles[base] {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v", path, err)
		}

		for _, imported := range file.Imports {
			if strings.Trim(imported.Path.Value, "\"") == clientImportPath {
				t.Fatalf("%s imports %q directly; route command logic through internal/adapter/primary/frontend instead", path, clientImportPath)
			}
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			clientSelector, ok := selector.X.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			optsIdent, ok := clientSelector.X.(*ast.Ident)
			if !ok {
				return true
			}

			if optsIdent.Name == "opts" && clientSelector.Sel.Name == "Client" {
				t.Fatalf("%s calls opts.Client.%s directly; command handlers must delegate to frontend workflows", path, selector.Sel.Name)
			}
			return true
		})
	}
}

func TestCommandFilesDoNotInstantiateFrontendWorkflowsDirectly(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || cliBootstrapFiles[base] {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v", path, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkgIdent, ok := selector.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "frontend" {
				return true
			}

			if strings.HasPrefix(selector.Sel.Name, "New") &&
				(strings.HasSuffix(selector.Sel.Name, "Workflow") || selector.Sel.Name == "NewClientFacade") {
				t.Fatalf("%s calls frontend.%s directly; command handlers must use opts.Frontend()", path, selector.Sel.Name)
			}
			return true
		})
	}
}
