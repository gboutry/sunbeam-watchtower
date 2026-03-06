// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

type LaunchpadConfig struct {
	DefaultOwner     string   `json:"default_owner" yaml:"default_owner"`
	UseKeyring       bool     `json:"use_keyring" yaml:"use_keyring"`
	Series           []string `json:"series,omitempty" yaml:"series,omitempty"`
	DevelopmentFocus string   `json:"development_focus,omitempty" yaml:"development_focus,omitempty"`
}

type GitHubConfig struct {
	UseKeyring bool `json:"use_keyring" yaml:"use_keyring"`
}

type GerritHost struct {
	URL string `json:"url" yaml:"url"`
}

type GerritConfig struct {
	Hosts []GerritHost `json:"hosts" yaml:"hosts"`
}

type CodeConfig struct {
	Forge   string `json:"forge" yaml:"forge"`
	Owner   string `json:"owner,omitempty" yaml:"owner,omitempty"`
	Host    string `json:"host,omitempty" yaml:"host,omitempty"`
	Project string `json:"project" yaml:"project"`
	GitURL  string `json:"git_url,omitempty" yaml:"git_url,omitempty"`
}

type BugTrackerConfig struct {
	Forge   string `json:"forge" yaml:"forge"`
	Owner   string `json:"owner,omitempty" yaml:"owner,omitempty"`
	Host    string `json:"host,omitempty" yaml:"host,omitempty"`
	Project string `json:"project" yaml:"project"`
}

type ProjectBuildConfig struct {
	Owner          string   `json:"owner" yaml:"owner"`
	Artifacts      []string `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	PrepareCommand string   `json:"prepare_command,omitempty" yaml:"prepare_command,omitempty"`
}

type ProjectConfig struct {
	Name             string              `json:"name" yaml:"name"`
	ArtifactType     string              `json:"artifact_type,omitempty" yaml:"artifact_type,omitempty"`
	Code             CodeConfig          `json:"code" yaml:"code"`
	Bugs             []BugTrackerConfig  `json:"bugs,omitempty" yaml:"bugs,omitempty"`
	Build            *ProjectBuildConfig `json:"build,omitempty" yaml:"build,omitempty"`
	Series           []string            `json:"series,omitempty" yaml:"series,omitempty"`
	DevelopmentFocus string              `json:"development_focus,omitempty" yaml:"development_focus,omitempty"`
}

type BuildConfig struct {
	DefaultPrefix  string `json:"default_prefix" yaml:"default_prefix"`
	TimeoutMinutes int    `json:"timeout_minutes" yaml:"timeout_minutes"`
	ArtifactsDir   string `json:"artifacts_dir" yaml:"artifacts_dir"`
}

type DistroSourceConfig struct {
	Mirror     string   `json:"mirror" yaml:"mirror"`
	Suites     []string `json:"suites" yaml:"suites"`
	Components []string `json:"components" yaml:"components"`
}

type ReleaseConfig struct {
	Suites    []string                  `json:"suites" yaml:"suites"`
	Backports map[string]BackportConfig `json:"backports,omitempty" yaml:"backports,omitempty"`
}

type DistroConfig struct {
	Mirror     string                   `json:"mirror" yaml:"mirror"`
	Components []string                 `json:"components" yaml:"components"`
	Releases   map[string]ReleaseConfig `json:"releases" yaml:"releases"`
}

type BackportConfig struct {
	ParentRelease string               `json:"parent_release,omitempty" yaml:"parent_release,omitempty"`
	Sources       []DistroSourceConfig `json:"sources" yaml:"sources"`
}

type UpstreamConfig struct {
	Provider         string `json:"provider" yaml:"provider"`
	ReleasesRepo     string `json:"releases_repo,omitempty" yaml:"releases_repo,omitempty"`
	RequirementsRepo string `json:"requirements_repo,omitempty" yaml:"requirements_repo,omitempty"`
}

type PackagesConfig struct {
	Distros  map[string]DistroConfig `json:"distros,omitempty" yaml:"distros,omitempty"`
	Sets     map[string][]string     `json:"sets,omitempty" yaml:"sets,omitempty"`
	Upstream *UpstreamConfig         `json:"upstream,omitempty" yaml:"upstream,omitempty"`
}

type Config struct {
	Launchpad LaunchpadConfig `json:"launchpad" yaml:"launchpad"`
	GitHub    GitHubConfig    `json:"github" yaml:"github"`
	Gerrit    GerritConfig    `json:"gerrit" yaml:"gerrit"`
	Projects  []ProjectConfig `json:"projects" yaml:"projects"`
	Build     BuildConfig     `json:"build" yaml:"build"`
	Packages  PackagesConfig  `json:"packages,omitempty" yaml:"packages,omitempty"`
}
