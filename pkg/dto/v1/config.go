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
	Group   string `json:"group,omitempty" yaml:"group,omitempty"`
}

type BugGroupConfig struct {
	CommonProject string `json:"common_project" yaml:"common_project"`
}

type ProjectBuildConfig struct {
	Owner          string   `json:"owner" yaml:"owner"`
	Artifacts      []string `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	PrepareCommand string   `json:"prepare_command,omitempty" yaml:"prepare_command,omitempty"`
}

type ProjectReleaseBranchConfig struct {
	Series string   `json:"series,omitempty" yaml:"series,omitempty"`
	Track  string   `json:"track,omitempty" yaml:"track,omitempty"`
	Branch string   `json:"branch" yaml:"branch"`
	Risks  []string `json:"risks,omitempty" yaml:"risks,omitempty"`
}

type ProjectReleaseConfig struct {
	Tracks                 []string                     `json:"tracks,omitempty" yaml:"tracks,omitempty"`
	TrackMap               map[string]string            `json:"track_map,omitempty" yaml:"track_map,omitempty"`
	Branches               []ProjectReleaseBranchConfig `json:"branches,omitempty" yaml:"branches,omitempty"`
	SkipArtifacts          []string                     `json:"skip_artifacts,omitempty" yaml:"skip_artifacts,omitempty"`
	TargetProfile          string                       `json:"target_profile,omitempty" yaml:"target_profile,omitempty"`
	TargetProfileOverrides *ReleaseTargetProfileConfig  `json:"target_profile_overrides,omitempty" yaml:"target_profile_overrides,omitempty"`
}

type ReleaseTargetMatcherConfig struct {
	BaseNames      []string `json:"base_names,omitempty" yaml:"base_names,omitempty"`
	BaseChannels   []string `json:"base_channels,omitempty" yaml:"base_channels,omitempty"`
	MinBaseChannel string   `json:"min_base_channel,omitempty" yaml:"min_base_channel,omitempty"`
	Architectures  []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`
}

type ReleaseTargetProfileConfig struct {
	Include []ReleaseTargetMatcherConfig `json:"include,omitempty" yaml:"include,omitempty"`
	Exclude []ReleaseTargetMatcherConfig `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type ReleasesConfig struct {
	DefaultTargetProfile string                                `json:"default_target_profile,omitempty" yaml:"default_target_profile,omitempty"`
	TargetProfiles       map[string]ReleaseTargetProfileConfig `json:"target_profiles,omitempty" yaml:"target_profiles,omitempty"`
}

type ProjectConfig struct {
	Name             string                `json:"name" yaml:"name"`
	ArtifactType     string                `json:"artifact_type,omitempty" yaml:"artifact_type,omitempty"`
	Code             CodeConfig            `json:"code" yaml:"code"`
	Bugs             []BugTrackerConfig    `json:"bugs,omitempty" yaml:"bugs,omitempty"`
	Build            *ProjectBuildConfig   `json:"build,omitempty" yaml:"build,omitempty"`
	Release          *ProjectReleaseConfig `json:"release,omitempty" yaml:"release,omitempty"`
	Series           []string              `json:"series,omitempty" yaml:"series,omitempty"`
	DevelopmentFocus string                `json:"development_focus,omitempty" yaml:"development_focus,omitempty"`
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

type ExcusesConfig struct {
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
	URL      string `json:"url" yaml:"url"`
	TeamURL  string `json:"team_url,omitempty" yaml:"team_url,omitempty"`
}

type DistroConfig struct {
	Mirror     string                   `json:"mirror" yaml:"mirror"`
	Components []string                 `json:"components" yaml:"components"`
	Releases   map[string]ReleaseConfig `json:"releases" yaml:"releases"`
	Excuses    *ExcusesConfig           `json:"excuses,omitempty" yaml:"excuses,omitempty"`
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

type OTelSignalConfig struct {
	Enabled       bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Endpoint      string            `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Protocol      string            `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Insecure      bool              `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	Headers       map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	SamplingRatio float64           `json:"sampling_ratio,omitempty" yaml:"sampling_ratio,omitempty"`
	MinLevel      string            `json:"min_level,omitempty" yaml:"min_level,omitempty"`
	MirrorStderr  bool              `json:"mirror_stderr,omitempty" yaml:"mirror_stderr,omitempty"`
}

type OTelMetricsListenerConfig struct {
	Enabled                bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ListenAddr             string   `json:"listen_addr,omitempty" yaml:"listen_addr,omitempty"`
	Path                   string   `json:"path,omitempty" yaml:"path,omitempty"`
	Runtime                bool     `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Process                bool     `json:"process,omitempty" yaml:"process,omitempty"`
	DefaultRefreshInterval string   `json:"default_refresh_interval,omitempty" yaml:"default_refresh_interval,omitempty"`
	LiveSystems            []string `json:"live_systems,omitempty" yaml:"live_systems,omitempty"`
}

type OTelCollectorConfig struct {
	Enabled         bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	RefreshInterval string `json:"refresh_interval,omitempty" yaml:"refresh_interval,omitempty"`
}

type OTelDomainCollectorsConfig struct {
	Auth       OTelCollectorConfig `json:"auth,omitempty" yaml:"auth,omitempty"`
	Operations OTelCollectorConfig `json:"operations,omitempty" yaml:"operations,omitempty"`
	Projects   OTelCollectorConfig `json:"projects,omitempty" yaml:"projects,omitempty"`
	Builds     OTelCollectorConfig `json:"builds,omitempty" yaml:"builds,omitempty"`
	Releases   OTelCollectorConfig `json:"releases,omitempty" yaml:"releases,omitempty"`
	Reviews    OTelCollectorConfig `json:"reviews,omitempty" yaml:"reviews,omitempty"`
	Commits    OTelCollectorConfig `json:"commits,omitempty" yaml:"commits,omitempty"`
	Bugs       OTelCollectorConfig `json:"bugs,omitempty" yaml:"bugs,omitempty"`
	Packages   OTelCollectorConfig `json:"packages,omitempty" yaml:"packages,omitempty"`
	Excuses    OTelCollectorConfig `json:"excuses,omitempty" yaml:"excuses,omitempty"`
	Cache      OTelCollectorConfig `json:"cache,omitempty" yaml:"cache,omitempty"`
}

type OTelMetricsConfig struct {
	Self       OTelMetricsListenerConfig  `json:"self,omitempty" yaml:"self,omitempty"`
	Domain     OTelMetricsListenerConfig  `json:"domain,omitempty" yaml:"domain,omitempty"`
	Collectors OTelDomainCollectorsConfig `json:"collectors,omitempty" yaml:"collectors,omitempty"`
}

type OTelConfig struct {
	ServiceName        string            `json:"service_name,omitempty" yaml:"service_name,omitempty"`
	ServiceNamespace   string            `json:"service_namespace,omitempty" yaml:"service_namespace,omitempty"`
	ResourceAttributes map[string]string `json:"resource_attributes,omitempty" yaml:"resource_attributes,omitempty"`
	Metrics            OTelMetricsConfig `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Traces             OTelSignalConfig  `json:"traces,omitempty" yaml:"traces,omitempty"`
	Logs               OTelSignalConfig  `json:"logs,omitempty" yaml:"logs,omitempty"`
}

type Config struct {
	Launchpad LaunchpadConfig           `json:"launchpad" yaml:"launchpad"`
	GitHub    GitHubConfig              `json:"github" yaml:"github"`
	Gerrit    GerritConfig              `json:"gerrit" yaml:"gerrit"`
	BugGroups map[string]BugGroupConfig `json:"bug_groups,omitempty" yaml:"bug_groups,omitempty"`
	Projects  []ProjectConfig           `json:"projects" yaml:"projects"`
	Build     BuildConfig               `json:"build" yaml:"build"`
	Releases  ReleasesConfig            `json:"releases,omitempty" yaml:"releases,omitempty"`
	Packages  PackagesConfig            `json:"packages,omitempty" yaml:"packages,omitempty"`
	OTel      OTelConfig                `json:"otel,omitempty" yaml:"otel,omitempty"`
}
