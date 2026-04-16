// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var coveragePattern = regexp.MustCompile(`coverage:\s+([0-9]+(?:\.[0-9]+)?)% of statements`)

type policy struct {
	DefaultThreshold float64            `yaml:"default_threshold"`
	PackageThreshold map[string]float64 `yaml:"package_thresholds"`
	ExcludePackages  []string           `yaml:"exclude_packages"`
}

type goListPackage struct {
	ImportPath string `json:"ImportPath"`
	Dir        string `json:"Dir"`
}

type packageCoverage struct {
	Package   string
	Coverage  float64
	Threshold float64
}

type runner interface {
	run(context.Context, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, string(output))
	}
	return output, nil
}

func main() {
	os.Exit(run(context.Background(), execRunner{}, os.Getwd, os.Stdout, os.Stderr, os.Args[1:]))
}

func run(ctx context.Context, shell runner, getwd func() (string, error), stdout, stderr io.Writer, args []string) int {
	wd, err := getwd()
	if err != nil {
		fmt.Fprintf(stderr, "coverageguard: resolve working directory: %v\n", err)
		return 1
	}
	return runMain(ctx, shell, wd, stdout, stderr, args)
}

func runMain(ctx context.Context, shell runner, workdir string, stdout, stderr io.Writer, args []string) int {
	configPath := flag.NewFlagSet("coverageguard", flag.ContinueOnError)
	configPath.SetOutput(stderr)

	policyFile := configPath.String("config", ".coverage-policy.yaml", "coverage policy file")
	if err := configPath.Parse(args); err != nil {
		return 2
	}

	p, err := loadPolicy(*policyFile)
	if err != nil {
		fmt.Fprintf(stderr, "coverageguard: %v\n", err)
		return 1
	}

	changedFiles := configPath.Args()
	if len(changedFiles) == 0 {
		changedFiles, err = stagedGoFiles(ctx, shell)
		if err != nil {
			fmt.Fprintf(stderr, "coverageguard: %v\n", err)
			return 1
		}
	}

	results, err := evaluateChangedPackages(ctx, shell, workdir, p, changedFiles)
	if err != nil {
		fmt.Fprintf(stderr, "coverageguard: %v\n", err)
		return 1
	}
	if len(results) == 0 {
		fmt.Fprintln(stdout, "coverageguard: no changed Go packages to evaluate")
		return 0
	}

	failed := false
	for _, result := range results {
		status := "PASS"
		if result.Coverage < result.Threshold {
			status = "FAIL"
			failed = true
		}
		fmt.Fprintf(stdout, "%s %s coverage %.1f%% (threshold %.1f%%)\n", status, result.Package, result.Coverage, result.Threshold)
	}

	if failed {
		return 1
	}
	return 0
}

func loadPolicy(path string) (*policy, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy %s: %w", path, err)
	}

	var p policy
	if err := yaml.Unmarshal(content, &p); err != nil {
		return nil, fmt.Errorf("parse policy %s: %w", path, err)
	}
	if p.PackageThreshold == nil {
		p.PackageThreshold = make(map[string]float64)
	}
	return &p, nil
}

func stagedGoFiles(ctx context.Context, shell runner) ([]string, error) {
	output, err := shell.run(ctx, "git", "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--", "*.go")
	if err != nil {
		return nil, err
	}
	return splitLines(string(output)), nil
}

func evaluateChangedPackages(ctx context.Context, shell runner, workdir string, p *policy, files []string) ([]packageCoverage, error) {
	packages, err := listPackages(ctx, shell, workdir)
	if err != nil {
		return nil, err
	}

	changedPackages, err := changedPackagesForFiles(workdir, packages, files)
	if err != nil {
		return nil, err
	}

	var results []packageCoverage
	for _, pkg := range changedPackages {
		if p.isExcluded(pkg) {
			continue
		}
		threshold := p.thresholdFor(pkg)
		coverage, err := packageCoveragePercent(ctx, shell, pkg)
		if err != nil {
			return nil, err
		}
		results = append(results, packageCoverage{
			Package:   pkg,
			Coverage:  coverage,
			Threshold: threshold,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Package < results[j].Package
	})
	return results, nil
}

func listPackages(ctx context.Context, shell runner, root string) (map[string]string, error) {
	output, err := shell.run(ctx, "go", "list", "-json", "./...")
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	packages := make(map[string]string)
	for {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode go list output: %w", err)
		}

		relDir, err := filepath.Rel(root, pkg.Dir)
		if err != nil {
			return nil, fmt.Errorf("resolve relative package path for %s: %w", pkg.Dir, err)
		}
		packages[filepath.Clean(relDir)] = pkg.ImportPath
	}
	return packages, nil
}

func changedPackagesForFiles(root string, packages map[string]string, files []string) ([]string, error) {
	seen := make(map[string]struct{})

	for _, file := range files {
		if filepath.Ext(file) != ".go" {
			continue
		}

		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, file)
		}

		dir := filepath.Dir(path)
		relDir, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, fmt.Errorf("resolve relative dir for %s: %w", file, err)
		}
		relDir = filepath.Clean(relDir)
		if relDir == "." {
			relDir = "."
		}

		if _, ok := packages[relDir]; !ok {
			continue
		}
		seen[relDir] = struct{}{}
	}

	changedPackages := make([]string, 0, len(seen))
	for pkg := range seen {
		changedPackages = append(changedPackages, pkg)
	}
	sort.Strings(changedPackages)
	return changedPackages, nil
}

func packageCoveragePercent(ctx context.Context, shell runner, pkg string) (float64, error) {
	target := "./" + pkg
	if pkg == "." {
		target = "."
	}
	output, err := shell.run(ctx, "go", "test", "-cover", target)
	if err != nil {
		return 0, err
	}
	return parseCoveragePercent(string(output))
}

func parseCoveragePercent(output string) (float64, error) {
	match := coveragePattern.FindStringSubmatch(output)
	if len(match) == 2 {
		var coverage float64
		if _, err := fmt.Sscanf(match[1], "%f", &coverage); err != nil {
			return 0, fmt.Errorf("parse coverage value %q: %w", match[1], err)
		}
		return coverage, nil
	}
	if strings.Contains(output, "[no test files]") {
		return 0, nil
	}
	return 0, fmt.Errorf("coverage output did not contain a coverage percentage")
}

func splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func (p *policy) isExcluded(pkg string) bool {
	for _, excluded := range p.ExcludePackages {
		if packageMatchesPattern(pkg, excluded) {
			return true
		}
	}
	return false
}

func (p *policy) thresholdFor(pkg string) float64 {
	threshold := p.DefaultThreshold
	longest := 0
	for pattern, value := range p.PackageThreshold {
		if !packageMatchesPattern(pkg, pattern) {
			continue
		}
		if len(pattern) > longest {
			longest = len(pattern)
			threshold = value
		}
	}
	return threshold
}

func packageMatchesPattern(pkg, pattern string) bool {
	pkg = filepath.Clean(pkg)
	pattern = filepath.Clean(pattern)
	return pkg == pattern || strings.HasPrefix(pkg, pattern+string(filepath.Separator))
}
