// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// GoFile holds one parsed Go source file.
type GoFile struct {
	Path string
	Base string
	AST  *ast.File
}

// LoadGoFiles parses non-test Go files matching the glob, excluding exempt base names.
func LoadGoFiles(glob string, exempt map[string]bool) ([]GoFile, error) {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	files := make([]GoFile, 0, len(matches))
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || exempt[base] {
			continue
		}

		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, err
		}
		files = append(files, GoFile{
			Path: path,
			Base: base,
			AST:  parsed,
		})
	}

	return files, nil
}

// ImportAliases returns the local aliases used for one import path.
func ImportAliases(file *ast.File, importPath, defaultAlias string) map[string]struct{} {
	aliases := map[string]struct{}{}
	for _, imported := range file.Imports {
		if strings.Trim(imported.Path.Value, "\"") != importPath {
			continue
		}
		if imported.Name != nil {
			aliases[imported.Name.Name] = struct{}{}
			continue
		}
		aliases[defaultAlias] = struct{}{}
	}
	return aliases
}

// Visit walks one file AST and invokes visit for each node of type T.
func Visit[T ast.Node](file *ast.File, visit func(T)) {
	ast.Inspect(file, func(node ast.Node) bool {
		typed, ok := node.(T)
		if ok {
			visit(typed)
		}
		return true
	})
}

// FuncTypeUsesImport reports whether a function signature references an imported package alias.
func FuncTypeUsesImport(fn *ast.FuncType, aliases map[string]struct{}) bool {
	return fieldListUsesImport(fn.Params, aliases) || fieldListUsesImport(fn.Results, aliases)
}

// ExportedTypeUsesImport reports whether the exported surface of one type references an imported package alias.
func ExportedTypeUsesImport(expr ast.Expr, aliases map[string]struct{}) bool {
	switch expr := expr.(type) {
	case *ast.StructType:
		for _, field := range expr.Fields.List {
			if len(field.Names) == 0 {
				if ExprUsesImport(field.Type, aliases) {
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
			if exported && ExprUsesImport(field.Type, aliases) {
				return true
			}
		}
		return false
	case *ast.InterfaceType:
		for _, field := range expr.Methods.List {
			if len(field.Names) == 0 {
				if ExprUsesImport(field.Type, aliases) {
					return true
				}
				continue
			}
			for _, name := range field.Names {
				if name.IsExported() && ExprUsesImport(field.Type, aliases) {
					return true
				}
			}
		}
		return false
	default:
		return ExprUsesImport(expr, aliases)
	}
}

func fieldListUsesImport(fields *ast.FieldList, aliases map[string]struct{}) bool {
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		if ExprUsesImport(field.Type, aliases) {
			return true
		}
	}
	return false
}

// ExprUsesImport reports whether expr references an imported package alias.
func ExprUsesImport(expr ast.Expr, aliases map[string]struct{}) bool {
	switch expr := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			if _, exists := aliases[ident.Name]; exists {
				return true
			}
		}
		return ExprUsesImport(expr.X, aliases)
	case *ast.StarExpr:
		return ExprUsesImport(expr.X, aliases)
	case *ast.ArrayType:
		return ExprUsesImport(expr.Elt, aliases)
	case *ast.MapType:
		return ExprUsesImport(expr.Key, aliases) || ExprUsesImport(expr.Value, aliases)
	case *ast.ChanType:
		return ExprUsesImport(expr.Value, aliases)
	case *ast.Ellipsis:
		return ExprUsesImport(expr.Elt, aliases)
	case *ast.FuncType:
		return FuncTypeUsesImport(expr, aliases)
	case *ast.StructType:
		return ExportedTypeUsesImport(expr, aliases)
	case *ast.InterfaceType:
		return ExportedTypeUsesImport(expr, aliases)
	case *ast.ParenExpr:
		return ExprUsesImport(expr.X, aliases)
	case *ast.IndexExpr:
		return ExprUsesImport(expr.X, aliases) || ExprUsesImport(expr.Index, aliases)
	case *ast.IndexListExpr:
		if ExprUsesImport(expr.X, aliases) {
			return true
		}
		for _, index := range expr.Indices {
			if ExprUsesImport(index, aliases) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
