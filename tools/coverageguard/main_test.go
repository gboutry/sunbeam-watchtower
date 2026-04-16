// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestParseCoveragePercent(t *testing.T) {
	t.Parallel()

	t.Run("parses coverage percentage", func(t *testing.T) {
		t.Parallel()
		got, err := parseCoveragePercent("ok\tpkg\t0.123s\tcoverage: 46.5% of statements\n")
		if err != nil {
			t.Fatalf("parseCoveragePercent() error = %v", err)
		}
		if got != 46.5 {
			t.Fatalf("parseCoveragePercent() = %v, want 46.5", got)
		}
	})

	t.Run("treats no test files as zero coverage", func(t *testing.T) {
		t.Parallel()
		got, err := parseCoveragePercent("?   \tpkg\t[no test files]\n")
		if err != nil {
			t.Fatalf("parseCoveragePercent() error = %v", err)
		}
		if got != 0 {
			t.Fatalf("parseCoveragePercent() = %v, want 0", got)
		}
	})

	t.Run("fails on missing coverage marker", func(t *testing.T) {
		t.Parallel()
		if _, err := parseCoveragePercent("ok\tpkg\t0.123s\n"); err == nil {
			t.Fatal("parseCoveragePercent() error = nil, want error")
		}
	})
}

func TestChangedPackagesForFiles(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "repo")
	packages := map[string]string{
		"internal/app": "example/internal/app",
		"pkg/client":   "example/pkg/client",
	}

	got, err := changedPackagesForFiles(root, packages, []string{
		"internal/app/app.go",
		"internal/app/app_test.go",
		"pkg/client/client.go",
		"README.md",
		"internal/missing/file.go",
	})
	if err != nil {
		t.Fatalf("changedPackagesForFiles() error = %v", err)
	}

	want := []string{"internal/app", "pkg/client"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changedPackagesForFiles() = %v, want %v", got, want)
	}
}

func TestPolicyThresholdFor(t *testing.T) {
	t.Parallel()

	p := &policy{
		DefaultThreshold: 40,
		PackageThreshold: map[string]float64{
			"internal":                     30,
			"internal/adapter/primary/cli": 45,
			"internal/core/service":        60,
		},
		ExcludePackages: []string{"cmd/watchtower"},
	}

	if !p.isExcluded("cmd/watchtower") {
		t.Fatal("isExcluded(cmd/watchtower) = false, want true")
	}
	if p.thresholdFor("internal/adapter/primary/cli") != 45 {
		t.Fatalf("thresholdFor(cli) = %v, want 45", p.thresholdFor("internal/adapter/primary/cli"))
	}
	if p.thresholdFor("internal/core/service/build") != 60 {
		t.Fatalf("thresholdFor(build) = %v, want 60", p.thresholdFor("internal/core/service/build"))
	}
	if p.thresholdFor("pkg/client") != 40 {
		t.Fatalf("thresholdFor(pkg/client) = %v, want 40", p.thresholdFor("pkg/client"))
	}
}

func TestSplitLinesAndPackageMatchesPattern(t *testing.T) {
	t.Parallel()

	got := splitLines(" internal/app/app.go \n\npkg/client/client.go\n")
	want := []string{"internal/app/app.go", "pkg/client/client.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitLines() = %v, want %v", got, want)
	}

	if !packageMatchesPattern("internal/adapter/primary/cli", "internal/adapter") {
		t.Fatal("packageMatchesPattern(cli, internal/adapter) = false, want true")
	}
	if packageMatchesPattern("pkg/client", "internal") {
		t.Fatal("packageMatchesPattern(pkg/client, internal) = true, want false")
	}
}

func TestRunMainUsesProvidedChangedFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	packageDir := filepath.Join(tmpDir, "internal/adapter/primary/cli")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(packageDir) error = %v", err)
	}
	policyPath := filepath.Join(tmpDir, "coverage-policy.yaml")
	if err := os.WriteFile(policyPath, []byte(`
default_threshold: 40
package_thresholds:
  internal/adapter/primary/cli: 45
`), 0o600); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}

	shell := &fakeRunner{
		outputs: map[string][]byte{
			"go list -json ./...":                           []byte(`{"ImportPath":"example/internal/adapter/primary/cli","Dir":"` + packageDir + `"}` + "\n"),
			"go test -cover ./internal/adapter/primary/cli": []byte("ok\texample/internal/adapter/primary/cli\t0.1s\tcoverage: 46.5% of statements\n"),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), shell, tmpDir, &stdout, &stderr, []string{
		"--config", policyPath,
		"internal/adapter/primary/cli/runtime.go",
	})
	if exitCode != 0 {
		t.Fatalf("runMain() exitCode = %d, want 0; stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "PASS internal/adapter/primary/cli coverage 46.5% (threshold 45.0%)") {
		t.Fatalf("runMain() stdout = %q", output)
	}
	if len(shell.callsSnapshot()) != 2 {
		t.Fatalf("shell.calls = %v, want 2 calls", shell.callsSnapshot())
	}
}

func TestEvaluateChangedPackagesRespectsExclusions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cliDir := filepath.Join(tmpDir, "internal/adapter/primary/cli")
	cmdDir := filepath.Join(tmpDir, "cmd/watchtower")
	for _, dir := range []string{cliDir, cmdDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	shell := &fakeRunner{
		outputs: map[string][]byte{
			"go list -json ./...": []byte(
				`{"ImportPath":"example/internal/adapter/primary/cli","Dir":"` + cliDir + `"}` + "\n" +
					`{"ImportPath":"example/cmd/watchtower","Dir":"` + cmdDir + `"}` + "\n",
			),
			"go test -cover ./internal/adapter/primary/cli": []byte("ok\texample/internal/adapter/primary/cli\t0.1s\tcoverage: 46.5% of statements\n"),
		},
	}

	results, err := evaluateChangedPackages(context.Background(), shell, tmpDir, &policy{
		DefaultThreshold: 40,
		ExcludePackages:  []string{"cmd/watchtower"},
	}, []string{
		"internal/adapter/primary/cli/runtime.go",
		"cmd/watchtower/main.go",
	})
	if err != nil {
		t.Fatalf("evaluateChangedPackages() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Package != "internal/adapter/primary/cli" {
		t.Fatalf("results[0].Package = %q, want internal/adapter/primary/cli", results[0].Package)
	}
}

func TestStagedGoFiles(t *testing.T) {
	t.Parallel()

	shell := &fakeRunner{
		outputs: map[string][]byte{
			"git diff --cached --name-only --diff-filter=ACMR -- *.go": []byte("internal/app/app.go\npkg/client/client.go\n"),
		},
	}

	got, err := stagedGoFiles(context.Background(), shell)
	if err != nil {
		t.Fatalf("stagedGoFiles() error = %v", err)
	}
	want := []string{"internal/app/app.go", "pkg/client/client.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stagedGoFiles() = %v, want %v", got, want)
	}
}

func TestPackageCoveragePercent(t *testing.T) {
	t.Parallel()

	shell := &fakeRunner{
		outputs: map[string][]byte{
			"go test -cover ./internal/app":   []byte("ok\texample/internal/app\t0.1s\tcoverage: 33.6% of statements\n"),
			"go test -cover ./cmd/watchtower": []byte("?   \texample/cmd/watchtower\t[no test files]\n"),
		},
	}

	got, err := packageCoveragePercent(context.Background(), shell, "internal/app")
	if err != nil {
		t.Fatalf("packageCoveragePercent() error = %v", err)
	}
	if got != 33.6 {
		t.Fatalf("packageCoveragePercent() = %v, want 33.6", got)
	}

	got, err = packageCoveragePercent(context.Background(), shell, "cmd/watchtower")
	if err != nil {
		t.Fatalf("packageCoveragePercent(no test files) error = %v", err)
	}
	if got != 0 {
		t.Fatalf("packageCoveragePercent(no test files) = %v, want 0", got)
	}
}

func TestLoadPolicy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "coverage-policy.yaml")
	if err := os.WriteFile(policyPath, []byte("default_threshold: 50\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}

	got, err := loadPolicy(policyPath)
	if err != nil {
		t.Fatalf("loadPolicy() error = %v", err)
	}
	if got.DefaultThreshold != 50 {
		t.Fatalf("DefaultThreshold = %v, want 50", got.DefaultThreshold)
	}
	if got.PackageThreshold == nil {
		t.Fatal("PackageThreshold = nil, want empty map")
	}
}

func TestRunMainFailsWhenCoverageBelowThreshold(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	packageDir := filepath.Join(tmpDir, "internal/app")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(packageDir) error = %v", err)
	}

	policyPath := filepath.Join(tmpDir, "coverage-policy.yaml")
	if err := os.WriteFile(policyPath, []byte("default_threshold: 40\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}

	shell := &fakeRunner{
		outputs: map[string][]byte{
			"go list -json ./...":           []byte(`{"ImportPath":"example/internal/app","Dir":"` + packageDir + `"}` + "\n"),
			"go test -cover ./internal/app": []byte("ok\texample/internal/app\t0.1s\tcoverage: 33.6% of statements\n"),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), shell, tmpDir, &stdout, &stderr, []string{
		"--config", policyPath,
		"internal/app/app.go",
	})
	if exitCode != 1 {
		t.Fatalf("runMain() exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stdout.String(), "FAIL internal/app coverage 33.6% (threshold 40.0%)") {
		t.Fatalf("runMain() stdout = %q", stdout.String())
	}
}

func TestRunMainReportsPolicyLoadError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runMain(context.Background(), &fakeRunner{}, t.TempDir(), &stdout, &stderr, []string{"--config", "missing.yaml"})
	if exitCode != 1 {
		t.Fatalf("runMain() exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "coverageguard: read policy missing.yaml") {
		t.Fatalf("runMain() stderr = %q", stderr.String())
	}
}

type fakeRunner struct {
	outputs map[string][]byte

	mu    sync.Mutex
	calls []string
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	f.mu.Lock()
	f.calls = append(f.calls, key)
	f.mu.Unlock()
	output, ok := f.outputs[key]
	if !ok {
		return nil, errors.New("unexpected command: " + key)
	}
	return output, nil
}

func (f *fakeRunner) callsSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}
