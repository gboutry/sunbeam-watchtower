// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

var apiFacadeExemptFiles = map[string]bool{
	"api_architecture_helpers.go": true,
	"cache.go":                    true,
	"packages.go":                 true,
	"packages_excuses.go":         true,
	"server.go":                   true,
}

func TestAPIHandlersUseServerFacadeInsteadOfAppSelectors(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || apiFacadeExemptFiles[base] {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v", path, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := selector.X.(*ast.Ident)
			if ok && ident.Name == "application" {
				t.Fatalf("%s uses application.%s directly; route API handler logic through frontend.NewServerFacade()", path, selector.Sel.Name)
			}
			return true
		})
	}
}

func TestAPIHandlersDoNotInstantiateWorkflowsDirectly(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || apiFacadeExemptFiles[base] {
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

			if strings.HasPrefix(selector.Sel.Name, "New") && selector.Sel.Name != "NewServerFacade" {
				t.Fatalf("%s calls frontend.%s directly; API handlers must use frontend.NewServerFacade()", path, selector.Sel.Name)
			}
			return true
		})
	}
}
