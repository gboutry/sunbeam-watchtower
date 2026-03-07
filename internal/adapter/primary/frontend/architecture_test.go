// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

const frontendClientImportPath = "github.com/gboutry/sunbeam-watchtower/pkg/client"

func TestExportedFrontendAPIsDoNotExposePkgClientTypes(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v", path, err)
		}

		aliases := frontendClientImportAliases(file)
		if len(aliases) == 0 {
			continue
		}

		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if !decl.Name.IsExported() || strings.HasPrefix(decl.Name.Name, "New") {
					continue
				}
				if frontendFuncTypeUsesClient(decl.Type, aliases) {
					t.Fatalf("%s exports %s with a pkg/client type in its signature; keep exported frontend APIs transport-agnostic", path, decl.Name.Name)
				}
			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || !typeSpec.Name.IsExported() {
						continue
					}
					if frontendExportedTypeUsesClient(typeSpec.Type, aliases) {
						t.Fatalf("%s exports type %s with a pkg/client field or alias; keep exported frontend DTOs transport-agnostic", path, typeSpec.Name.Name)
					}
				}
			}
		}
	}
}

func frontendClientImportAliases(file *ast.File) map[string]struct{} {
	aliases := map[string]struct{}{}
	for _, imported := range file.Imports {
		if strings.Trim(imported.Path.Value, "\"") != frontendClientImportPath {
			continue
		}
		if imported.Name != nil {
			aliases[imported.Name.Name] = struct{}{}
			continue
		}
		aliases["client"] = struct{}{}
	}
	return aliases
}

func frontendFuncTypeUsesClient(fn *ast.FuncType, aliases map[string]struct{}) bool {
	return frontendFieldListUsesClient(fn.Params, aliases) || frontendFieldListUsesClient(fn.Results, aliases)
}

func frontendFieldListUsesClient(fields *ast.FieldList, aliases map[string]struct{}) bool {
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		if frontendExprUsesClient(field.Type, aliases) {
			return true
		}
	}
	return false
}

func frontendExportedTypeUsesClient(expr ast.Expr, aliases map[string]struct{}) bool {
	switch expr := expr.(type) {
	case *ast.StructType:
		for _, field := range expr.Fields.List {
			if len(field.Names) == 0 {
				if frontendExprUsesClient(field.Type, aliases) {
					return true
				}
				continue
			}
			exported := false
			for _, name := range field.Names {
				if name.IsExported() {
					exported = true
					break
				}
			}
			if exported && frontendExprUsesClient(field.Type, aliases) {
				return true
			}
		}
		return false
	case *ast.InterfaceType:
		for _, field := range expr.Methods.List {
			if len(field.Names) == 0 {
				if frontendExprUsesClient(field.Type, aliases) {
					return true
				}
				continue
			}
			for _, name := range field.Names {
				if name.IsExported() && frontendExprUsesClient(field.Type, aliases) {
					return true
				}
			}
		}
		return false
	default:
		return frontendExprUsesClient(expr, aliases)
	}
}

func frontendExprUsesClient(expr ast.Expr, aliases map[string]struct{}) bool {
	switch expr := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			if _, exists := aliases[ident.Name]; exists {
				return true
			}
		}
		return frontendExprUsesClient(expr.X, aliases)
	case *ast.StarExpr:
		return frontendExprUsesClient(expr.X, aliases)
	case *ast.ArrayType:
		return frontendExprUsesClient(expr.Elt, aliases)
	case *ast.MapType:
		return frontendExprUsesClient(expr.Key, aliases) || frontendExprUsesClient(expr.Value, aliases)
	case *ast.ChanType:
		return frontendExprUsesClient(expr.Value, aliases)
	case *ast.Ellipsis:
		return frontendExprUsesClient(expr.Elt, aliases)
	case *ast.FuncType:
		return frontendFuncTypeUsesClient(expr, aliases)
	case *ast.StructType:
		return frontendExportedTypeUsesClient(expr, aliases)
	case *ast.InterfaceType:
		return frontendExportedTypeUsesClient(expr, aliases)
	case *ast.ParenExpr:
		return frontendExprUsesClient(expr.X, aliases)
	case *ast.IndexExpr:
		return frontendExprUsesClient(expr.X, aliases) || frontendExprUsesClient(expr.Index, aliases)
	case *ast.IndexListExpr:
		if frontendExprUsesClient(expr.X, aliases) {
			return true
		}
		for _, index := range expr.Indices {
			if frontendExprUsesClient(index, aliases) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
