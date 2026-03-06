// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"gopkg.in/yaml.v3"
)

var _ port.UpstreamProvider = (*Provider)(nil)

// Provider implements port.UpstreamProvider by reading data from locally
// cloned bare git repos (openstack/releases and openstack/requirements).
type Provider struct {
	releasesDir     string
	requirementsDir string
	mapping         map[string]string
}

// NewProvider creates an OpenStack upstream provider that reads from the given
// bare repo paths.
func NewProvider(releasesDir, requirementsDir string) *Provider {
	return &Provider{
		releasesDir:     releasesDir,
		requirementsDir: requirementsDir,
		mapping:         defaultMapping,
	}
}

// deliverableFile represents the YAML structure of a deliverables file in the
// openstack/releases repository.
type deliverableFile struct {
	Team     string               `yaml:"team"`
	Type     string               `yaml:"type"`
	Releases []deliverableRelease `yaml:"releases"`
}

type deliverableRelease struct {
	Version string `yaml:"version"`
}

// ListDeliverables returns known deliverables for the given release by reading
// YAML files from deliverables/<release>/ in the releases repo.
func (p *Provider) ListDeliverables(ctx context.Context, release string) ([]port.Deliverable, error) {
	treePath := fmt.Sprintf("HEAD:deliverables/%s/", release)
	listing, err := gitShow(ctx, p.releasesDir, treePath)
	if err != nil {
		return nil, fmt.Errorf("listing deliverables for %s: %w", release, err)
	}

	var result []port.Deliverable
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || !strings.HasSuffix(name, ".yaml") {
			continue
		}

		filePath := fmt.Sprintf("HEAD:deliverables/%s/%s", release, name)
		content, err := gitShow(ctx, p.releasesDir, filePath)
		if err != nil {
			continue
		}

		d, err := parseDeliverable(name, []byte(content))
		if err != nil {
			continue
		}
		result = append(result, d)
	}
	return result, nil
}

// parseDeliverable parses a single deliverable YAML file and returns a
// Deliverable. The fileName is used to derive the deliverable name (without
// the .yaml suffix).
func parseDeliverable(fileName string, data []byte) (port.Deliverable, error) {
	var df deliverableFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return port.Deliverable{}, fmt.Errorf("parsing %s: %w", fileName, err)
	}

	name := strings.TrimSuffix(fileName, ".yaml")
	dtype := mapDeliverableType(df.Type)
	version := latestVersion(df.Releases)

	return port.Deliverable{
		Name:    name,
		Type:    dtype,
		Version: version,
		Team:    df.Team,
	}, nil
}

// latestVersion returns the latest non-lifecycle version from the releases
// list, iterating backwards. A "lifecycle" version ends in -eol or -eom.
func latestVersion(releases []deliverableRelease) string {
	for i := len(releases) - 1; i >= 0; i-- {
		v := releases[i].Version
		if isLifecycleVersion(v) {
			continue
		}
		return v
	}
	return ""
}

// isLifecycleVersion returns true if the version string represents an
// end-of-life or end-of-maintenance marker.
func isLifecycleVersion(v string) bool {
	return strings.HasSuffix(v, "-eol") || strings.HasSuffix(v, "-eom")
}

// mapDeliverableType converts the string type from the releases YAML to a
// port.DeliverableType.
func mapDeliverableType(t string) port.DeliverableType {
	switch strings.ToLower(t) {
	case "service":
		return port.DeliverableService
	case "library":
		return port.DeliverableLibrary
	case "client-library":
		return port.DeliverableClient
	default:
		return port.DeliverableOther
	}
}

// GetConstraints returns upper version constraints for the given release from
// the requirements repo. It tries the stable/<release> branch first, then
// falls back to HEAD.
func (p *Provider) GetConstraints(ctx context.Context, release string) (map[string]string, error) {
	ref := fmt.Sprintf("origin/stable/%s:upper-constraints.txt", release)
	content, err := gitShow(ctx, p.requirementsDir, ref)
	if err != nil {
		// Fallback to HEAD (e.g. for master/main).
		content, err = gitShow(ctx, p.requirementsDir, "HEAD:upper-constraints.txt")
		if err != nil {
			return nil, fmt.Errorf("reading upper-constraints.txt: %w", err)
		}
	}
	return parseConstraints([]byte(content))
}

// parseConstraints parses an upper-constraints.txt file. Each line has the
// format name===version (possibly followed by ;env-markers).
func parseConstraints(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip environment markers (everything after ';').
		if idx := strings.Index(line, ";"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		parts := strings.SplitN(line, "===", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])
		if name != "" && version != "" {
			result[name] = version
		}
	}
	return result, scanner.Err()
}

// MapPackageName maps an upstream deliverable name to a distro source package
// name. It first checks the override mapping, then applies a type-based
// heuristic.
func (p *Provider) MapPackageName(deliverable string, dtype port.DeliverableType) string {
	// Normalise: dots → hyphens.
	name := strings.ReplaceAll(deliverable, ".", "-")

	// Check explicit mapping first.
	if mapped, ok := p.mapping[deliverable]; ok {
		return mapped
	}

	// oslo.* libraries → python-oslo-*
	if strings.HasPrefix(name, "oslo-") {
		return "python-" + name
	}

	switch dtype {
	case port.DeliverableLibrary:
		return "python-" + name
	case port.DeliverableService, port.DeliverableClient:
		return name
	default:
		return name
	}
}

// gitShow runs `git -C <dir> show <ref>` and returns the output as a string.
func gitShow(ctx context.Context, dir, ref string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "show", ref)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git show %s in %s: %w", ref, dir, err)
	}
	return string(out), nil
}
