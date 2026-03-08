// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOTelAndPrometheusImportsStayConfinedToTelemetryAdapter(t *testing.T) {
	fset := token.NewFileSet()
	repoRoot := filepath.Join("..", "..", "..", "..")
	for _, root := range []string{
		filepath.Join(repoRoot, "internal"),
		filepath.Join(repoRoot, "pkg"),
		filepath.Join(repoRoot, "cmd"),
	} {
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(path)
			if strings.Contains(rel, "/internal/adapter/secondary/otel/") || strings.HasSuffix(rel, "/internal/adapter/secondary/otel") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if parseErr != nil {
				return parseErr
			}
			for _, imported := range file.Imports {
				pathValue := strings.Trim(imported.Path.Value, "\"")
				if strings.HasPrefix(pathValue, "go.opentelemetry.io/") || strings.HasPrefix(pathValue, "github.com/prometheus/client_golang/") {
					t.Fatalf("%s imports observability dependency %q outside internal/adapter/secondary/otel", rel, pathValue)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
