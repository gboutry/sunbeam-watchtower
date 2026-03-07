// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOptionalHumaRequestFieldsDeclareRequiredTags(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v", path, err)
		}

		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(typeSpec.Name.Name, "Input") {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				assertHumaRequiredTags(t, path, typeSpec.Name.Name, structType)
			}
		}
	}
}

func assertHumaRequiredTags(t *testing.T, path, typeName string, st *ast.StructType) {
	t.Helper()

	for _, field := range st.Fields.List {
		if nested, ok := field.Type.(*ast.StructType); ok {
			assertHumaRequiredTags(t, path, typeName, nested)
		}

		if field.Tag == nil {
			continue
		}
		tagValue := strings.Trim(field.Tag.Value, "`")
		tag := reflect.StructTag(tagValue)
		if tag.Get("query") == "" && tag.Get("json") == "" && tag.Get("header") == "" {
			continue
		}
		if !needsExplicitRequired(field.Type) {
			continue
		}
		if tag.Get("required") == "" {
			name := "<embedded>"
			if len(field.Names) > 0 {
				name = field.Names[0].Name
			}
			t.Fatalf("%s %s.%s is a slice/map/bool request field without required tag; add required:\"false\" or required:\"true\"", path, typeName, name)
		}
	}
}

func needsExplicitRequired(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.ArrayType:
		return true
	case *ast.MapType:
		return true
	case *ast.Ident:
		return expr.Name == "bool"
	default:
		return false
	}
}
