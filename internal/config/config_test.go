package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FullConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	yaml := `
launchpad:
  default_owner: my-team
  use_keyring: true
github:
  use_keyring: false
gerrit:
  hosts:
    - url: https://review.opendev.org
projects:
  - name: snap-openstack
    code:
      forge: github
      owner: canonical
      project: snap-openstack
    bugs:
      - forge: launchpad
        project: snap-openstack
  - name: sunbeam-charms
    code:
      forge: gerrit
      host: https://review.opendev.org
      project: openstack/sunbeam-charms
    bugs:
      - forge: launchpad
        project: snap-openstack
      - forge: launchpad
        project: sunbeam-charms
  - name: charm-keystone
    code:
      forge: launchpad
      project: charm-keystone-k8s
build:
  default_prefix: sunbeam
  timeout_minutes: 45
  artifacts_dir: /tmp/artifacts
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Launchpad.DefaultOwner != "my-team" {
		t.Errorf("Launchpad.DefaultOwner = %q, want %q", cfg.Launchpad.DefaultOwner, "my-team")
	}
	if !cfg.Launchpad.UseKeyring {
		t.Error("Launchpad.UseKeyring = false, want true")
	}
	if cfg.GitHub.UseKeyring {
		t.Error("GitHub.UseKeyring = true, want false")
	}
	if len(cfg.Gerrit.Hosts) != 1 || cfg.Gerrit.Hosts[0].URL != "https://review.opendev.org" {
		t.Errorf("Gerrit.Hosts = %+v, want one host with URL https://review.opendev.org", cfg.Gerrit.Hosts)
	}
	if len(cfg.Projects) != 3 {
		t.Fatalf("len(Projects) = %d, want 3", len(cfg.Projects))
	}

	// snap-openstack: github code + 1 LP bug tracker
	p0 := cfg.Projects[0]
	if p0.Name != "snap-openstack" {
		t.Errorf("Projects[0].Name = %q, want %q", p0.Name, "snap-openstack")
	}
	if p0.Code.Forge != "github" || p0.Code.Owner != "canonical" || p0.Code.Project != "snap-openstack" {
		t.Errorf("Projects[0].Code = %+v", p0.Code)
	}
	if len(p0.Bugs) != 1 || p0.Bugs[0].Forge != "launchpad" || p0.Bugs[0].Project != "snap-openstack" {
		t.Errorf("Projects[0].Bugs = %+v", p0.Bugs)
	}

	// sunbeam-charms: gerrit code + 2 LP bug trackers
	p1 := cfg.Projects[1]
	if p1.Code.Forge != "gerrit" || p1.Code.Host != "https://review.opendev.org" {
		t.Errorf("Projects[1].Code = %+v", p1.Code)
	}
	if len(p1.Bugs) != 2 {
		t.Fatalf("len(Projects[1].Bugs) = %d, want 2", len(p1.Bugs))
	}

	// charm-keystone: launchpad code, no bug trackers
	p2 := cfg.Projects[2]
	if p2.Code.Forge != "launchpad" || p2.Code.Project != "charm-keystone-k8s" {
		t.Errorf("Projects[2].Code = %+v", p2.Code)
	}
	if len(p2.Bugs) != 0 {
		t.Errorf("Projects[2].Bugs should be empty, got %+v", p2.Bugs)
	}

	if cfg.Build.TimeoutMinutes != 45 {
		t.Errorf("Build.TimeoutMinutes = %d, want 45", cfg.Build.TimeoutMinutes)
	}
	if cfg.Build.ArtifactsDir != "/tmp/artifacts" {
		t.Errorf("Build.ArtifactsDir = %q, want %q", cfg.Build.ArtifactsDir, "/tmp/artifacts")
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Build.TimeoutMinutes != 30 {
		t.Errorf("default TimeoutMinutes = %d, want 30", cfg.Build.TimeoutMinutes)
	}
	if cfg.Build.ArtifactsDir != "artifacts" {
		t.Errorf("default ArtifactsDir = %q, want %q", cfg.Build.ArtifactsDir, "artifacts")
	}
}

func TestLoad_MissingExplicitFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("Load() should error for missing explicit config path")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("key:\n\t- bad indent"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Fatal("Load() should error for invalid YAML")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name: "p1",
				Code: CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Bugs: []BugTrackerConfig{
					{Forge: "launchpad", Project: "p1"},
				},
			},
			{
				Name: "p2",
				Code: CodeConfig{Forge: "gerrit", Host: "https://review.opendev.org", Project: "openstack/nova"},
			},
			{
				Name: "p3",
				Code: CodeConfig{Forge: "launchpad", Project: "proj"},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestValidate_MissingName(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{Code: CodeConfig{Forge: "github", Owner: "org", Project: "repo"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for missing name")
	}
}

func TestValidate_InvalidCodeForge(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{Name: "p1", Code: CodeConfig{Forge: "gitlab", Project: "repo"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for invalid code forge")
	}
}

func TestValidate_GitHubMissingOwner(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{Name: "p1", Code: CodeConfig{Forge: "github", Project: "repo"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for github code missing owner")
	}
}

func TestValidate_GerritMissingHost(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{Name: "p1", Code: CodeConfig{Forge: "gerrit", Project: "openstack/nova"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for gerrit code missing host")
	}
}

func TestValidate_InvalidBugForge(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name: "p1",
				Code: CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Bugs: []BugTrackerConfig{{Forge: "jira", Project: "proj"}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for invalid bug tracker forge")
	}
}

func TestValidate_BugMissingProject(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name: "p1",
				Code: CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Bugs: []BugTrackerConfig{{Forge: "launchpad"}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for bug tracker missing project")
	}
}

func TestValidate_ValidArtifactTypeAndBuild(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name:         "p1",
				ArtifactType: "rock",
				Code:         CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Build:        &ProjectBuildConfig{Owner: "team", Recipes: []string{"recipe1"}},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestValidate_InvalidArtifactType(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name:         "p1",
				ArtifactType: "docker",
				Code:         CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for invalid artifact_type")
	}
}

func TestValidate_BuildWithoutArtifactType(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name:  "p1",
				Code:  CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Build: &ProjectBuildConfig{Owner: "team"},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for build without artifact_type")
	}
}

func TestValidate_BuildWithoutOwner(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name:         "p1",
				ArtifactType: "charm",
				Code:         CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Build:        &ProjectBuildConfig{},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for build without owner")
	}
}

func TestValidate_BuildWithPrepareCommand(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name:         "p1",
				ArtifactType: "snap",
				Code:         CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Build:        &ProjectBuildConfig{Owner: "team", PrepareCommand: "make prepare"},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestValidate_DevelopmentFocusNotInSeries(t *testing.T) {
	cfg := &Config{
		Launchpad: LaunchpadConfig{
			Series:           []string{"2024.1", "2024.2"},
			DevelopmentFocus: "2025.1",
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error when development_focus is not in series")
	}
}

func TestValidate_DevelopmentFocusInSeries(t *testing.T) {
	cfg := &Config{
		Launchpad: LaunchpadConfig{
			Series:           []string{"2024.1", "2024.2", "2025.1"},
			DevelopmentFocus: "2025.1",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestValidate_DevelopmentFocusWithoutSeries(t *testing.T) {
	cfg := &Config{
		Launchpad: LaunchpadConfig{
			DevelopmentFocus: "2025.1",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should pass when series is empty: %v", err)
	}
}

func TestValidate_ProjectDevelopmentFocusNotInSeries(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name:             "p1",
				Code:             CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Series:           []string{"1.0", "2.0"},
				DevelopmentFocus: "3.0",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error when project development_focus is not in project series")
	}
}

func TestValidate_ProjectDevelopmentFocusInSeries(t *testing.T) {
	cfg := &Config{
		Projects: []ProjectConfig{
			{
				Name:             "p1",
				Code:             CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
				Series:           []string{"1.0", "2.0"},
				DevelopmentFocus: "2.0",
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestLoad_UpstreamConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	yaml := `
packages:
  upstream:
    provider: openstack
    releases_repo: https://opendev.org/openstack/releases
    requirements_repo: https://opendev.org/openstack/requirements
  distros:
    ubuntu:
      mirror: http://archive.ubuntu.com/ubuntu
      suites: [noble]
      components: [main]
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Packages.Upstream == nil {
		t.Fatal("Packages.Upstream should not be nil")
	}
	if cfg.Packages.Upstream.Provider != "openstack" {
		t.Errorf("Upstream.Provider = %q, want %q", cfg.Packages.Upstream.Provider, "openstack")
	}
	if cfg.Packages.Upstream.ReleasesRepo != "https://opendev.org/openstack/releases" {
		t.Errorf("Upstream.ReleasesRepo = %q, want %q", cfg.Packages.Upstream.ReleasesRepo, "https://opendev.org/openstack/releases")
	}
	if cfg.Packages.Upstream.RequirementsRepo != "https://opendev.org/openstack/requirements" {
		t.Errorf("Upstream.RequirementsRepo = %q, want %q", cfg.Packages.Upstream.RequirementsRepo, "https://opendev.org/openstack/requirements")
	}
	if len(cfg.Packages.Distros) != 1 {
		t.Errorf("len(Packages.Distros) = %d, want 1", len(cfg.Packages.Distros))
	}
}

func TestValidate_UpstreamMissingProvider(t *testing.T) {
	cfg := &Config{
		Packages: PackagesConfig{
			Upstream: &UpstreamConfig{},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for upstream missing provider")
	}
}

func TestValidate_UpstreamOpenstackMissingReleasesRepo(t *testing.T) {
	cfg := &Config{
		Packages: PackagesConfig{
			Upstream: &UpstreamConfig{Provider: "openstack"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should error for openstack provider missing releases_repo")
	}
}

func TestValidate_UpstreamValid(t *testing.T) {
	cfg := &Config{
		Packages: PackagesConfig{
			Upstream: &UpstreamConfig{
				Provider:     "openstack",
				ReleasesRepo: "https://opendev.org/openstack/releases",
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestValidate_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should pass for empty config: %v", err)
	}
}
