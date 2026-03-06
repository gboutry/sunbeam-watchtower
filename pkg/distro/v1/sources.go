// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ulikunitz/xz"
)

// SourcesFileName returns the filename for a Sources file.
func SourcesFileName(suite, component, format string) string {
	return fmt.Sprintf("%s_%s_Sources.%s",
		strings.ReplaceAll(suite, "/", "_"),
		component,
		format)
}

// ParseSources reads an RFC822-format Sources file and yields SourcePackage entries.
// The suite and component are set from the caller.
func ParseSources(r io.Reader, suite, component string) ([]SourcePackage, error) {
	var results []SourcePackage
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var pkg, ver string
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if pkg != "" && ver != "" {
				results = append(results, SourcePackage{
					Package:   pkg,
					Version:   ver,
					Suite:     suite,
					Component: component,
				})
			}
			pkg, ver = "", ""
			continue
		}

		if line[0] == ' ' || line[0] == '\t' {
			continue
		}

		if strings.HasPrefix(line, "Package:") {
			pkg = strings.TrimSpace(line[len("Package:"):])
		} else if strings.HasPrefix(line, "Version:") {
			ver = strings.TrimSpace(line[len("Version:"):])
		}
	}

	if pkg != "" && ver != "" {
		results = append(results, SourcePackage{
			Package:   pkg,
			Version:   ver,
			Suite:     suite,
			Component: component,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning sources: %w", err)
	}
	return results, nil
}

// ParseSourcesDetailed reads an RFC822-format Sources file and yields SourcePackageDetail entries.
func ParseSourcesDetailed(r io.Reader, suite, component string) ([]SourcePackageDetail, error) {
	var results []SourcePackageDetail
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var pkg, ver, bdRaw, bdiRaw string
	inBuildDepends := false
	inBuildDependsIndep := false

	flush := func() {
		if pkg == "" || ver == "" {
			return
		}
		detail := SourcePackageDetail{
			SourcePackage: SourcePackage{
				Package:   pkg,
				Version:   ver,
				Suite:     suite,
				Component: component,
			},
		}
		combined := bdRaw
		if bdiRaw != "" {
			if combined != "" {
				combined += ", " + bdiRaw
			} else {
				combined = bdiRaw
			}
		}
		if combined != "" {
			detail.BuildDepends = parseBuildDepends(combined)
		}
		results = append(results, detail)
	}

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			flush()
			pkg, ver, bdRaw, bdiRaw = "", "", "", ""
			inBuildDepends = false
			inBuildDependsIndep = false
			continue
		}

		if line[0] == ' ' || line[0] == '\t' {
			if inBuildDepends {
				bdRaw += " " + strings.TrimSpace(line)
			} else if inBuildDependsIndep {
				bdiRaw += " " + strings.TrimSpace(line)
			}
			continue
		}

		inBuildDepends = false
		inBuildDependsIndep = false

		if strings.HasPrefix(line, "Package:") {
			pkg = strings.TrimSpace(line[len("Package:"):])
		} else if strings.HasPrefix(line, "Version:") {
			ver = strings.TrimSpace(line[len("Version:"):])
		} else if strings.HasPrefix(line, "Build-Depends-Indep:") {
			bdiRaw = strings.TrimSpace(line[len("Build-Depends-Indep:"):])
			inBuildDependsIndep = true
		} else if strings.HasPrefix(line, "Build-Depends:") {
			bdRaw = strings.TrimSpace(line[len("Build-Depends:"):])
			inBuildDepends = true
		}
	}

	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning sources: %w", err)
	}
	return results, nil
}

// ParseSourcesFileDetailed opens and parses a compressed Sources file returning detailed entries.
func ParseSourcesFileDetailed(path, format, suite, component string) ([]SourcePackageDetail, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening sources file: %w", err)
	}
	defer f.Close()

	reader, err := decompressReader(f, format)
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}
	return ParseSourcesDetailed(reader, suite, component)
}

// ParseSourcesWithFiles reads an RFC822-format Sources file and yields SourcePackageFiles entries.
func ParseSourcesWithFiles(r io.Reader, suite, component string) ([]SourcePackageFiles, error) {
	var results []SourcePackageFiles
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var pkg, ver, dir string
	var files []string
	inFiles := false

	flush := func() {
		if pkg != "" && ver != "" {
			results = append(results, SourcePackageFiles{
				SourcePackage: SourcePackage{
					Package:   pkg,
					Version:   ver,
					Suite:     suite,
					Component: component,
				},
				Directory: dir,
				Files:     files,
			})
		}
		pkg, ver, dir = "", "", ""
		files = nil
		inFiles = false
	}

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			flush()
			continue
		}

		if line[0] == ' ' || line[0] == '\t' {
			if inFiles {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					files = append(files, fields[len(fields)-1])
				}
			}
			continue
		}

		inFiles = false

		if strings.HasPrefix(line, "Package:") {
			pkg = strings.TrimSpace(line[len("Package:"):])
		} else if strings.HasPrefix(line, "Version:") {
			ver = strings.TrimSpace(line[len("Version:"):])
		} else if strings.HasPrefix(line, "Directory:") {
			dir = strings.TrimSpace(line[len("Directory:"):])
		} else if strings.HasPrefix(line, "Files:") {
			inFiles = true
		}
	}

	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning sources with files: %w", err)
	}
	return results, nil
}

// ParseSourcesFileWithFiles opens and parses a compressed Sources file from disk.
func ParseSourcesFileWithFiles(path, format, suite, component string) ([]SourcePackageFiles, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening sources file: %w", err)
	}
	defer f.Close()

	reader, err := decompressReader(f, format)
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}
	return ParseSourcesWithFiles(reader, suite, component)
}

func parseBuildDepends(raw string) []string {
	parts := strings.Split(raw, ",")
	var deps []string
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		for i, ch := range name {
			if ch == ' ' || ch == '(' || ch == '[' || ch == '<' {
				name = name[:i]
				break
			}
		}
		if name != "" {
			deps = append(deps, name)
		}
	}
	return deps
}

func decompressReader(r io.Reader, format string) (io.Reader, error) {
	switch format {
	case "xz":
		return xz.NewReader(r)
	case "gz":
		return gzip.NewReader(r)
	default:
		return r, nil
	}
}
