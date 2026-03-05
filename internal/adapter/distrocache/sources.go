// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package distrocache

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	distro "github.com/gboutry/sunbeam-watchtower/internal/pkg/distro/v1"
	"github.com/ulikunitz/xz"
)

// ParseSources reads an RFC822-format Sources file and yields SourcePackage entries.
// The suite and component are set from the caller (known from the download path).
func ParseSources(r io.Reader, suite, component string) ([]distro.SourcePackage, error) {
	var results []distro.SourcePackage
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var pkg, ver string
	for scanner.Scan() {
		line := scanner.Text()

		// Blank line = end of paragraph.
		if line == "" {
			if pkg != "" && ver != "" {
				results = append(results, distro.SourcePackage{
					Package:   pkg,
					Version:   ver,
					Suite:     suite,
					Component: component,
				})
			}
			pkg, ver = "", ""
			continue
		}

		// Skip continuation lines (start with space/tab).
		if line[0] == ' ' || line[0] == '\t' {
			continue
		}

		if strings.HasPrefix(line, "Package:") {
			pkg = strings.TrimSpace(line[len("Package:"):])
		} else if strings.HasPrefix(line, "Version:") {
			ver = strings.TrimSpace(line[len("Version:"):])
		}
	}

	// Handle last paragraph if file doesn't end with a blank line.
	if pkg != "" && ver != "" {
		results = append(results, distro.SourcePackage{
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

// ParseSourcesDetailed reads an RFC822-format Sources file and yields SourcePackageDetail entries
// including parsed Build-Depends and Build-Depends-Indep.
func ParseSourcesDetailed(r io.Reader, suite, component string) ([]distro.SourcePackageDetail, error) {
	var results []distro.SourcePackageDetail
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var pkg, ver, bdRaw, bdiRaw string
	inBuildDepends := false
	inBuildDependsIndep := false

	flush := func() {
		if pkg == "" || ver == "" {
			return
		}
		detail := distro.SourcePackageDetail{
			SourcePackage: distro.SourcePackage{
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

		// Continuation line.
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

// parseBuildDepends extracts package names from a Build-Depends value string.
func parseBuildDepends(raw string) []string {
	parts := strings.Split(raw, ",")
	var deps []string
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		// Take only the package name (before any space, paren, bracket).
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

// parseSourcesFileDetailed opens and parses a compressed Sources file returning detailed entries.
func parseSourcesFileDetailed(path, format, suite, component string) ([]distro.SourcePackageDetail, error) {
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

// ParseSourcesWithFiles reads an RFC822-format Sources file and yields SourcePackageFiles entries
// including Directory and Files fields needed for .dsc URL construction.
func ParseSourcesWithFiles(r io.Reader, suite, component string) ([]distro.SourcePackageFiles, error) {
	var results []distro.SourcePackageFiles
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var pkg, ver, dir string
	var files []string
	inFiles := false

	flush := func() {
		if pkg != "" && ver != "" {
			results = append(results, distro.SourcePackageFiles{
				SourcePackage: distro.SourcePackage{
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

		// Continuation line (starts with space/tab).
		if line[0] == ' ' || line[0] == '\t' {
			if inFiles {
				// Format: " <checksum> <size> <filename>"
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					files = append(files, fields[len(fields)-1])
				}
			}
			continue
		}

		// New field — no longer in Files continuation.
		inFiles = false

		if strings.HasPrefix(line, "Package:") {
			pkg = strings.TrimSpace(line[len("Package:"):])
		} else if strings.HasPrefix(line, "Version:") {
			ver = strings.TrimSpace(line[len("Version:"):])
		} else if strings.HasPrefix(line, "Directory:") {
			dir = strings.TrimSpace(line[len("Directory:"):])
		} else if strings.HasPrefix(line, "Files:") {
			inFiles = true
			// The value after "Files:" on the same line is usually empty.
		}
	}

	// Handle last paragraph if file doesn't end with a blank line.
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning sources with files: %w", err)
	}
	return results, nil
}

// ParseSourcesFileWithFiles opens and parses a compressed Sources file from disk,
// returning entries with Directory and Files information.
func ParseSourcesFileWithFiles(path, format, suite, component string) ([]distro.SourcePackageFiles, error) {
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

// decompressReader wraps a reader with the appropriate decompressor based on format.
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

// parseSourcesFile opens and parses a compressed Sources file from disk.
func parseSourcesFile(path, format, suite, component string) ([]distro.SourcePackage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening sources file: %w", err)
	}
	defer f.Close()

	reader, err := decompressReader(f, format)
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}
	return ParseSources(reader, suite, component)
}

// sourcesURL builds the URL for a Sources index file.
func sourcesURL(mirror, suite, component, format string) string {
	return fmt.Sprintf("%s/dists/%s/%s/source/Sources.%s",
		strings.TrimRight(mirror, "/"), suite, component, format)
}

// downloadSourcesFile downloads a Sources index file to the given path.
// It tries .xz first, then falls back to .gz.
// Returns the format that succeeded ("xz" or "gz").
func downloadSourcesFile(ctx context.Context, client *http.Client, mirror, suite, component, destPath string) (string, error) {
	for _, format := range []string{"xz", "gz"} {
		url := sourcesURL(mirror, suite, component, format)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		out, err := os.Create(destPath)
		if err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("creating file %s: %w", destPath, err)
		}

		_, copyErr := io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()

		if copyErr != nil {
			os.Remove(destPath)
			return "", fmt.Errorf("writing file %s: %w", destPath, copyErr)
		}

		return format, nil
	}

	return "", fmt.Errorf("no Sources index found for %s %s/%s", mirror, suite, component)
}
