package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LaunchpadConfig holds Launchpad-specific settings.
type LaunchpadConfig struct {
	DefaultOwner     string   `mapstructure:"default_owner" yaml:"default_owner"`
	UseKeyring       bool     `mapstructure:"use_keyring" yaml:"use_keyring"`
	Series           []string `mapstructure:"series" yaml:"series,omitempty"`
	DevelopmentFocus string   `mapstructure:"development_focus" yaml:"development_focus,omitempty"`
}

// GitHubConfig holds GitHub-specific settings.
type GitHubConfig struct {
	UseKeyring bool   `mapstructure:"use_keyring" yaml:"use_keyring"`
	ClientID   string `mapstructure:"client_id" yaml:"client_id,omitempty"`
}

// GerritHost represents a single Gerrit instance.
type GerritHost struct {
	URL string `mapstructure:"url" yaml:"url"`
}

// GerritConfig holds Gerrit-specific settings.
type GerritConfig struct {
	Hosts []GerritHost `mapstructure:"hosts" yaml:"hosts"`
}

// CodeConfig specifies where a project's code is hosted (exactly one forge).
type CodeConfig struct {
	Forge   string `mapstructure:"forge" yaml:"forge"`
	Owner   string `mapstructure:"owner" yaml:"owner,omitempty"`
	Host    string `mapstructure:"host" yaml:"host,omitempty"`
	Project string `mapstructure:"project" yaml:"project"`
	GitURL  string `mapstructure:"git_url" yaml:"git_url,omitempty"` // explicit clone URL override
}

// BugTrackerConfig specifies a bug tracker for a project.
type BugTrackerConfig struct {
	Forge   string `mapstructure:"forge" yaml:"forge"`
	Owner   string `mapstructure:"owner" yaml:"owner,omitempty"`
	Host    string `mapstructure:"host" yaml:"host,omitempty"`
	Project string `mapstructure:"project" yaml:"project"`
	Group   string `mapstructure:"group" yaml:"group,omitempty"`
}

// BugGroupConfig defines one shared bug tracker group.
type BugGroupConfig struct {
	CommonProject string `mapstructure:"common_project" yaml:"common_project"`
}

// ProjectBuildConfig holds per-project build settings.
type ProjectBuildConfig struct {
	Owner               string   `mapstructure:"owner" yaml:"owner,omitempty"`
	Artifacts           []string `mapstructure:"artifacts" yaml:"artifacts,omitempty"`
	PrepareCommand      string   `mapstructure:"prepare_command" yaml:"prepare_command,omitempty"`
	OfficialCodehosting bool     `mapstructure:"official_codehosting" yaml:"official_codehosting,omitempty"`
	LPProject           string   `mapstructure:"lp_project" yaml:"lp_project,omitempty"`
}

// ProjectReleaseBranchConfig declares one explicitly managed release branch.
type ProjectReleaseBranchConfig struct {
	Series string   `mapstructure:"series" yaml:"series,omitempty"`
	Track  string   `mapstructure:"track" yaml:"track,omitempty"`
	Branch string   `mapstructure:"branch" yaml:"branch"`
	Risks  []string `mapstructure:"risks" yaml:"risks,omitempty"`
}

// ProjectReleaseConfig holds per-project release tracking overrides.
type ProjectReleaseConfig struct {
	Tracks                 []string                     `mapstructure:"tracks" yaml:"tracks,omitempty"`
	TrackMap               map[string]string            `mapstructure:"track_map" yaml:"track_map,omitempty"`
	Branches               []ProjectReleaseBranchConfig `mapstructure:"branches" yaml:"branches,omitempty"`
	SkipArtifacts          []string                     `mapstructure:"skip_artifacts" yaml:"skip_artifacts,omitempty"`
	TargetProfile          string                       `mapstructure:"target_profile" yaml:"target_profile,omitempty"`
	TargetProfileOverrides *ReleaseTargetProfileConfig  `mapstructure:"target_profile_overrides" yaml:"target_profile_overrides,omitempty"`
}

// ReleaseTargetMatcherConfig defines one release target visibility matcher.
type ReleaseTargetMatcherConfig struct {
	BaseNames      []string `mapstructure:"base_names" yaml:"base_names,omitempty"`
	BaseChannels   []string `mapstructure:"base_channels" yaml:"base_channels,omitempty"`
	MinBaseChannel string   `mapstructure:"min_base_channel" yaml:"min_base_channel,omitempty"`
	Architectures  []string `mapstructure:"architectures" yaml:"architectures,omitempty"`
}

// ReleaseTargetProfileConfig defines include/exclude rules for release targets.
type ReleaseTargetProfileConfig struct {
	Include []ReleaseTargetMatcherConfig `mapstructure:"include" yaml:"include,omitempty"`
	Exclude []ReleaseTargetMatcherConfig `mapstructure:"exclude" yaml:"exclude,omitempty"`
}

// ReleasesConfig holds frontend-side release presentation defaults.
type ReleasesConfig struct {
	DefaultTargetProfile string                                `mapstructure:"default_target_profile" yaml:"default_target_profile,omitempty"`
	TargetProfiles       map[string]ReleaseTargetProfileConfig `mapstructure:"target_profiles" yaml:"target_profiles,omitempty"`
}

// ProjectConfig defines a project tracked across forges.
type ProjectConfig struct {
	Name             string                `mapstructure:"name" yaml:"name"`
	ArtifactType     string                `mapstructure:"artifact_type" yaml:"artifact_type,omitempty"`
	Code             CodeConfig            `mapstructure:"code" yaml:"code"`
	Bugs             []BugTrackerConfig    `mapstructure:"bugs" yaml:"bugs,omitempty"`
	Build            *ProjectBuildConfig   `mapstructure:"build" yaml:"build,omitempty"`
	Release          *ProjectReleaseConfig `mapstructure:"release" yaml:"release,omitempty"`
	Series           []string              `mapstructure:"series" yaml:"series,omitempty"`
	DevelopmentFocus string                `mapstructure:"development_focus" yaml:"development_focus,omitempty"`
}

// BuildConfig holds build pipeline settings.
type BuildConfig struct {
	DefaultPrefix  string `mapstructure:"default_prefix" yaml:"default_prefix"`
	TimeoutMinutes int    `mapstructure:"timeout_minutes" yaml:"timeout_minutes"`
	ArtifactsDir   string `mapstructure:"artifacts_dir" yaml:"artifacts_dir"`
}

// DistroSourceConfig represents an APT mirror with suites and components.
type DistroSourceConfig struct {
	Mirror     string   `mapstructure:"mirror" yaml:"mirror"`
	Suites     []string `mapstructure:"suites" yaml:"suites"`
	Components []string `mapstructure:"components" yaml:"components"`
}

// ReleaseConfig defines a distro release (e.g. noble, jammy, trixie).
// Suites are suite type names (release, updates, proposed, backports) that get
// expanded to full names by prepending the release name (e.g. "updates" → "noble-updates").
// The special type "release" expands to just the release name itself (e.g. "noble").
type ReleaseConfig struct {
	Suites    []string                  `mapstructure:"suites" yaml:"suites"`
	Backports map[string]BackportConfig `mapstructure:"backports" yaml:"backports,omitempty"`
}

// DistroConfig defines an APT distribution (e.g. Ubuntu, Debian).
type DistroConfig struct {
	Mirror     string                   `mapstructure:"mirror" yaml:"mirror"`
	Components []string                 `mapstructure:"components" yaml:"components"`
	Releases   map[string]ReleaseConfig `mapstructure:"releases" yaml:"releases"`
	Excuses    *ExcusesConfig           `mapstructure:"excuses" yaml:"excuses,omitempty"`
}

// BackportConfig defines a backport source group (e.g. UCA, OSBPO).
type BackportConfig struct {
	ParentRelease string               `mapstructure:"parent_release" yaml:"parent_release,omitempty"`
	Sources       []DistroSourceConfig `mapstructure:"sources" yaml:"sources"`
}

// ExcusesConfig configures the proposed-migration excuses feed for a distro.
type ExcusesConfig struct {
	Provider string `mapstructure:"provider" yaml:"provider,omitempty"`
	URL      string `mapstructure:"url" yaml:"url"`
	TeamURL  string `mapstructure:"team_url" yaml:"team_url,omitempty"`
}

// ExpandSuiteType expands a suite type name to its full APT suite name for a given release.
// "release" → releaseName, anything else → releaseName + "-" + suiteType.
func ExpandSuiteType(releaseName, suiteType string) string {
	if suiteType == "release" {
		return releaseName
	}
	return releaseName + "-" + suiteType
}

// ExpandBackportSuiteType expands a suite type name for a backport source.
// Known types are expanded relative to the parent release and backport name:
//   - "release" → releaseName (e.g. "noble")
//   - "updates" → releaseName-updates/backportName (e.g. "noble-updates/gazpacho")
//   - "proposed" → releaseName-proposed/backportName (e.g. "noble-proposed/gazpacho")
//
// Any other value is treated as a literal suite name and returned as-is
// (e.g. "trixie-gazpacho-backports" stays unchanged).
func ExpandBackportSuiteType(releaseName, backportName, suiteType string) string {
	switch suiteType {
	case "release":
		return releaseName
	case "updates":
		return releaseName + "-updates/" + backportName
	case "proposed":
		return releaseName + "-proposed/" + backportName
	default:
		return suiteType
	}
}

// UpstreamConfig configures an upstream version provider.
type UpstreamConfig struct {
	Provider         string `mapstructure:"provider" yaml:"provider"`
	ReleasesRepo     string `mapstructure:"releases_repo" yaml:"releases_repo,omitempty"`
	RequirementsRepo string `mapstructure:"requirements_repo" yaml:"requirements_repo,omitempty"`
}

// PackagesConfig holds configuration for the packages subcommand.
type PackagesConfig struct {
	Distros  map[string]DistroConfig `mapstructure:"distros" yaml:"distros,omitempty"`
	Sets     map[string][]string     `mapstructure:"sets" yaml:"sets,omitempty"`
	Upstream *UpstreamConfig         `mapstructure:"upstream" yaml:"upstream,omitempty"`
}

// OTelSignalConfig configures one OTLP-exported signal.
type OTelSignalConfig struct {
	Enabled       bool              `mapstructure:"enabled" yaml:"enabled,omitempty"`
	Endpoint      string            `mapstructure:"endpoint" yaml:"endpoint,omitempty"`
	Protocol      string            `mapstructure:"protocol" yaml:"protocol,omitempty"`
	Insecure      bool              `mapstructure:"insecure" yaml:"insecure,omitempty"`
	Headers       map[string]string `mapstructure:"headers" yaml:"headers,omitempty"`
	SamplingRatio float64           `mapstructure:"sampling_ratio" yaml:"sampling_ratio,omitempty"`
	MinLevel      string            `mapstructure:"min_level" yaml:"min_level,omitempty"`
	MirrorStderr  bool              `mapstructure:"mirror_stderr" yaml:"mirror_stderr,omitempty"`
}

// OTelMetricsListenerConfig configures one metrics listener.
type OTelMetricsListenerConfig struct {
	Enabled                bool     `mapstructure:"enabled" yaml:"enabled,omitempty"`
	ListenAddr             string   `mapstructure:"listen_addr" yaml:"listen_addr,omitempty"`
	Path                   string   `mapstructure:"path" yaml:"path,omitempty"`
	Runtime                bool     `mapstructure:"runtime" yaml:"runtime,omitempty"`
	Process                bool     `mapstructure:"process" yaml:"process,omitempty"`
	DefaultRefreshInterval string   `mapstructure:"default_refresh_interval" yaml:"default_refresh_interval,omitempty"`
	LiveSystems            []string `mapstructure:"live_systems" yaml:"live_systems,omitempty"`
}

// OTelCollectorConfig configures one domain metrics collector.
type OTelCollectorConfig struct {
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled,omitempty"`
	RefreshInterval string `mapstructure:"refresh_interval" yaml:"refresh_interval,omitempty"`
}

// OTelDomainCollectorsConfig groups supported domain metrics collectors.
type OTelDomainCollectorsConfig struct {
	Auth       OTelCollectorConfig `mapstructure:"auth" yaml:"auth,omitempty"`
	Operations OTelCollectorConfig `mapstructure:"operations" yaml:"operations,omitempty"`
	Projects   OTelCollectorConfig `mapstructure:"projects" yaml:"projects,omitempty"`
	Builds     OTelCollectorConfig `mapstructure:"builds" yaml:"builds,omitempty"`
	Releases   OTelCollectorConfig `mapstructure:"releases" yaml:"releases,omitempty"`
	Reviews    OTelCollectorConfig `mapstructure:"reviews" yaml:"reviews,omitempty"`
	Commits    OTelCollectorConfig `mapstructure:"commits" yaml:"commits,omitempty"`
	Bugs       OTelCollectorConfig `mapstructure:"bugs" yaml:"bugs,omitempty"`
	Packages   OTelCollectorConfig `mapstructure:"packages" yaml:"packages,omitempty"`
	Excuses    OTelCollectorConfig `mapstructure:"excuses" yaml:"excuses,omitempty"`
	Cache      OTelCollectorConfig `mapstructure:"cache" yaml:"cache,omitempty"`
}

// OTelMetricsConfig groups self and domain metrics configuration.
type OTelMetricsConfig struct {
	Self       OTelMetricsListenerConfig  `mapstructure:"self" yaml:"self,omitempty"`
	Domain     OTelMetricsListenerConfig  `mapstructure:"domain" yaml:"domain,omitempty"`
	Collectors OTelDomainCollectorsConfig `mapstructure:"collectors" yaml:"collectors,omitempty"`
}

// OTelConfig configures OpenTelemetry exporters and metrics listeners.
type OTelConfig struct {
	ServiceName        string            `mapstructure:"service_name" yaml:"service_name,omitempty"`
	ServiceNamespace   string            `mapstructure:"service_namespace" yaml:"service_namespace,omitempty"`
	ResourceAttributes map[string]string `mapstructure:"resource_attributes" yaml:"resource_attributes,omitempty"`
	Metrics            OTelMetricsConfig `mapstructure:"metrics" yaml:"metrics,omitempty"`
	Traces             OTelSignalConfig  `mapstructure:"traces" yaml:"traces,omitempty"`
	Logs               OTelSignalConfig  `mapstructure:"logs" yaml:"logs,omitempty"`
}

// Config is the top-level configuration.
type Config struct {
	Launchpad LaunchpadConfig           `mapstructure:"launchpad" yaml:"launchpad"`
	GitHub    GitHubConfig              `mapstructure:"github" yaml:"github"`
	Gerrit    GerritConfig              `mapstructure:"gerrit" yaml:"gerrit"`
	BugGroups map[string]BugGroupConfig `mapstructure:"bug_groups" yaml:"bug_groups,omitempty"`
	Projects  []ProjectConfig           `mapstructure:"projects" yaml:"projects"`
	Build     BuildConfig               `mapstructure:"build" yaml:"build"`
	Releases  ReleasesConfig            `mapstructure:"releases" yaml:"releases,omitempty"`
	Packages  PackagesConfig            `mapstructure:"packages" yaml:"packages,omitempty"`
	OTel      OTelConfig                `mapstructure:"otel" yaml:"otel,omitempty"`
}

// Load reads configuration from the given path. If configPath is empty,
// it searches ~/.config/sunbeam-watchtower/config.yaml. A missing file
// returns defaults (no error).
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	// Defaults
	v.SetDefault("build.timeout_minutes", 30)
	v.SetDefault("build.artifacts_dir", "artifacts")
	v.SetDefault("otel.metrics.self.path", "/metrics")
	v.SetDefault("otel.metrics.domain.path", "/metrics")
	v.SetDefault("otel.metrics.domain.default_refresh_interval", "5m")
	v.SetDefault("otel.metrics.domain.live_systems", []string{})
	v.SetDefault("otel.metrics.self.runtime", true)
	v.SetDefault("otel.metrics.self.process", true)
	v.SetDefault("otel.traces.protocol", "grpc")
	v.SetDefault("otel.logs.protocol", "grpc")
	v.SetDefault("otel.logs.min_level", "info")
	v.SetDefault("otel.logs.mirror_stderr", true)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return defaults(v)
		}
		v.AddConfigPath(filepath.Join(home, ".config", "sunbeam-watchtower"))
		v.SetConfigName("config")
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return defaults(v)
		}
		// If an explicit path was given and it doesn't exist, that's an error.
		if configPath != "" {
			if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
				return nil, fmt.Errorf("config file not found: %s", configPath)
			}
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return defaults(v)
}

func defaults(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	return &cfg, nil
}

// Validate checks that the configuration is consistent.
func (c *Config) Validate() error {
	for name, group := range c.BugGroups {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("bug_groups cannot contain an empty group name")
		}
		if strings.TrimSpace(group.CommonProject) == "" {
			return fmt.Errorf("bug_groups.%s.common_project is required", name)
		}
	}

	groupProjects := make(map[string]map[string]bool, len(c.BugGroups))
	groupForges := make(map[string]string, len(c.BugGroups))
	if c.Launchpad.DevelopmentFocus != "" && len(c.Launchpad.Series) > 0 {
		found := false
		for _, s := range c.Launchpad.Series {
			if s == c.Launchpad.DevelopmentFocus {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("launchpad: development_focus %q must be one of the declared series", c.Launchpad.DevelopmentFocus)
		}
	}
	if c.Releases.DefaultTargetProfile != "" {
		if _, ok := c.Releases.TargetProfiles[c.Releases.DefaultTargetProfile]; !ok {
			return fmt.Errorf("releases.default_target_profile %q must match a configured releases.target_profiles entry", c.Releases.DefaultTargetProfile)
		}
	}
	for name, profile := range c.Releases.TargetProfiles {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("releases.target_profiles cannot contain an empty profile name")
		}
		if err := validateReleaseTargetProfileConfig(fmt.Sprintf("releases.target_profiles.%s", name), profile); err != nil {
			return err
		}
	}
	for i, p := range c.Projects {
		if p.Name == "" {
			return fmt.Errorf("projects[%d]: name is required", i)
		}
		if err := validateForgeRef(fmt.Sprintf("projects[%d] (%s) code", i, p.Name), p.Code.Forge, p.Code.Owner, p.Code.Host, p.Code.Project); err != nil {
			return err
		}
		for j, bug := range p.Bugs {
			if err := validateForgeRef(fmt.Sprintf("projects[%d] (%s) bugs[%d]", i, p.Name, j), bug.Forge, bug.Owner, bug.Host, bug.Project); err != nil {
				return err
			}
			if bug.Group != "" {
				_, ok := c.BugGroups[bug.Group]
				if !ok {
					return fmt.Errorf("projects[%d] (%s) bugs[%d]: unknown bug group %q", i, p.Name, j, bug.Group)
				}
				if groupProjects[bug.Group] == nil {
					groupProjects[bug.Group] = make(map[string]bool)
				}
				groupProjects[bug.Group][bug.Project] = true
				if existing, ok := groupForges[bug.Group]; ok && existing != bug.Forge {
					return fmt.Errorf("bug_groups.%s must use a single forge, got %q and %q", bug.Group, existing, bug.Forge)
				}
				groupForges[bug.Group] = bug.Forge
			}
		}

		validArtifactTypes := map[string]bool{"rock": true, "charm": true, "snap": true}
		if p.ArtifactType != "" && !validArtifactTypes[p.ArtifactType] {
			return fmt.Errorf("projects[%d] (%s): invalid artifact_type %q (must be rock, charm, or snap)", i, p.Name, p.ArtifactType)
		}
		if p.Build != nil {
			if p.ArtifactType == "" {
				return fmt.Errorf("projects[%d] (%s): artifact_type is required when build is set", i, p.Name)
			}
			if p.Build.OfficialCodehosting && p.Build.Owner == "" {
				return fmt.Errorf("projects[%d] (%s): build.owner is required when official_codehosting is true", i, p.Name)
			}
		}
		if p.Release != nil {
			if p.ArtifactType != "snap" && p.ArtifactType != "charm" {
				return fmt.Errorf("projects[%d] (%s): release overrides require artifact_type snap or charm", i, p.Name)
			}
			effectiveSeries := p.Series
			if len(effectiveSeries) == 0 {
				effectiveSeries = c.Launchpad.Series
			}
			if len(p.Release.Tracks) > 0 && len(p.Release.TrackMap) > 0 {
				return fmt.Errorf("projects[%d] (%s): release.tracks and release.track_map are mutually exclusive", i, p.Name)
			}
			seenTracks := make(map[string]bool, len(p.Release.Tracks))
			for _, track := range p.Release.Tracks {
				if track == "" {
					return fmt.Errorf("projects[%d] (%s): release.tracks cannot contain empty values", i, p.Name)
				}
				if seenTracks[track] {
					return fmt.Errorf("projects[%d] (%s): release.tracks contains duplicate %q", i, p.Name, track)
				}
				seenTracks[track] = true
			}
			seenArtifacts := make(map[string]bool, len(p.Release.SkipArtifacts))
			for _, name := range p.Release.SkipArtifacts {
				if name == "" {
					return fmt.Errorf("projects[%d] (%s): release.skip_artifacts cannot contain empty values", i, p.Name)
				}
				if seenArtifacts[name] {
					return fmt.Errorf("projects[%d] (%s): release.skip_artifacts contains duplicate %q", i, p.Name, name)
				}
				seenArtifacts[name] = true
			}
			for series, track := range p.Release.TrackMap {
				if series == "" {
					return fmt.Errorf("projects[%d] (%s): release.track_map cannot contain an empty series key", i, p.Name)
				}
				if track == "" {
					return fmt.Errorf("projects[%d] (%s): release.track_map[%q] cannot map to an empty track", i, p.Name, series)
				}
				if len(effectiveSeries) > 0 {
					found := false
					for _, knownSeries := range effectiveSeries {
						if knownSeries == series {
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("projects[%d] (%s): release.track_map key %q must match one of the declared series", i, p.Name, series)
					}
				}
			}
			for j, branch := range p.Release.Branches {
				if branch.Branch == "" {
					return fmt.Errorf("projects[%d] (%s): release.branches[%d].branch is required", i, p.Name, j)
				}
				if (branch.Series == "" && branch.Track == "") || (branch.Series != "" && branch.Track != "") {
					return fmt.Errorf("projects[%d] (%s): release.branches[%d] must set exactly one of series or track", i, p.Name, j)
				}
				if branch.Series != "" && len(effectiveSeries) > 0 {
					found := false
					for _, knownSeries := range effectiveSeries {
						if knownSeries == branch.Series {
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("projects[%d] (%s): release.branches[%d].series %q must match one of the declared series", i, p.Name, j, branch.Series)
					}
				}
				seenRisks := make(map[string]bool, len(branch.Risks))
				for _, risk := range branch.Risks {
					switch risk {
					case "edge", "beta", "candidate", "stable":
					default:
						return fmt.Errorf("projects[%d] (%s): release.branches[%d].risks contains invalid risk %q", i, p.Name, j, risk)
					}
					if seenRisks[risk] {
						return fmt.Errorf("projects[%d] (%s): release.branches[%d].risks contains duplicate %q", i, p.Name, j, risk)
					}
					seenRisks[risk] = true
				}
			}
			if p.Release.TargetProfile != "" {
				if _, ok := c.Releases.TargetProfiles[p.Release.TargetProfile]; !ok {
					return fmt.Errorf("projects[%d] (%s): release.target_profile %q must match a configured releases.target_profiles entry", i, p.Name, p.Release.TargetProfile)
				}
			}
			if p.Release.TargetProfileOverrides != nil {
				if err := validateReleaseTargetProfileConfig(fmt.Sprintf("projects[%d] (%s): release.target_profile_overrides", i, p.Name), *p.Release.TargetProfileOverrides); err != nil {
					return err
				}
			}
		}

		if p.DevelopmentFocus != "" && len(p.Series) > 0 {
			found := false
			for _, s := range p.Series {
				if s == p.DevelopmentFocus {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("projects[%d] (%s): development_focus %q must be one of the declared series", i, p.Name, p.DevelopmentFocus)
			}
		}
	}
	if c.Packages.Upstream != nil {
		if c.Packages.Upstream.Provider == "" {
			return fmt.Errorf("packages.upstream: provider is required")
		}
		if c.Packages.Upstream.Provider == "openstack" && c.Packages.Upstream.ReleasesRepo == "" {
			return fmt.Errorf("packages.upstream: releases_repo is required for openstack provider")
		}
	}
	for distroName, distro := range c.Packages.Distros {
		if distro.Excuses == nil {
			continue
		}
		if distro.Excuses.URL == "" {
			return fmt.Errorf("packages.distros.%s.excuses: url is required", distroName)
		}
	}
	if err := validateOTelConfig(c.OTel); err != nil {
		return err
	}

	for groupName, group := range c.BugGroups {
		projects := groupProjects[groupName]
		if len(projects) == 0 {
			return fmt.Errorf("bug_groups.%s is not referenced by any bug tracker entry", groupName)
		}
		if !projects[group.CommonProject] {
			return fmt.Errorf("bug_groups.%s.common_project %q must match one of the grouped tracker projects", groupName, group.CommonProject)
		}
	}
	return nil
}

func validateReleaseTargetProfileConfig(prefix string, profile ReleaseTargetProfileConfig) error {
	for idx, matcher := range profile.Include {
		if err := validateReleaseTargetMatcherConfig(fmt.Sprintf("%s.include[%d]", prefix, idx), matcher); err != nil {
			return err
		}
	}
	for idx, matcher := range profile.Exclude {
		if err := validateReleaseTargetMatcherConfig(fmt.Sprintf("%s.exclude[%d]", prefix, idx), matcher); err != nil {
			return err
		}
	}
	return nil
}

func validateReleaseTargetMatcherConfig(prefix string, matcher ReleaseTargetMatcherConfig) error {
	if len(matcher.BaseNames) == 0 && len(matcher.BaseChannels) == 0 && matcher.MinBaseChannel == "" && len(matcher.Architectures) == 0 {
		return fmt.Errorf("%s must set at least one of base_names, base_channels, min_base_channel, or architectures", prefix)
	}
	if matcher.MinBaseChannel != "" {
		if _, err := parseReleaseBaseChannelVersion(matcher.MinBaseChannel); err != nil {
			return fmt.Errorf("%s.min_base_channel: %w", prefix, err)
		}
	}
	return nil
}

func parseReleaseBaseChannelVersion(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("value cannot be empty")
	}
	parts := strings.Split(raw, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid dotted numeric version %q", raw)
	}
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("invalid dotted numeric version %q", raw)
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid dotted numeric version %q", raw)
		}
		values = append(values, value)
	}
	return values, nil
}

func validateOTelConfig(cfg OTelConfig) error {
	if err := validateMetricsListener("otel.metrics.self", cfg.Metrics.Self); err != nil {
		return err
	}
	if err := validateMetricsListener("otel.metrics.domain", cfg.Metrics.Domain); err != nil {
		return err
	}
	if cfg.Metrics.Self.Enabled && cfg.Metrics.Domain.Enabled && cfg.Metrics.Self.ListenAddr == cfg.Metrics.Domain.ListenAddr && cfg.Metrics.Self.ListenAddr != "" {
		return fmt.Errorf("otel.metrics.self.listen_addr and otel.metrics.domain.listen_addr must differ")
	}
	if err := validateSignalConfig("otel.traces", cfg.Traces, true); err != nil {
		return err
	}
	if err := validateSignalConfig("otel.logs", cfg.Logs, false); err != nil {
		return err
	}
	validLiveSystems := map[string]bool{
		"reviews": true,
		"builds":  true,
		"bugs":    true,
		"commits": true,
	}
	seenLiveSystems := make(map[string]bool, len(cfg.Metrics.Domain.LiveSystems))
	for _, system := range cfg.Metrics.Domain.LiveSystems {
		if !validLiveSystems[system] {
			return fmt.Errorf("otel.metrics.domain.live_systems contains unknown system %q", system)
		}
		if seenLiveSystems[system] {
			return fmt.Errorf("otel.metrics.domain.live_systems contains duplicate %q", system)
		}
		seenLiveSystems[system] = true
	}
	for name, collector := range map[string]OTelCollectorConfig{
		"auth":       cfg.Metrics.Collectors.Auth,
		"operations": cfg.Metrics.Collectors.Operations,
		"projects":   cfg.Metrics.Collectors.Projects,
		"builds":     cfg.Metrics.Collectors.Builds,
		"releases":   cfg.Metrics.Collectors.Releases,
		"reviews":    cfg.Metrics.Collectors.Reviews,
		"commits":    cfg.Metrics.Collectors.Commits,
		"bugs":       cfg.Metrics.Collectors.Bugs,
		"packages":   cfg.Metrics.Collectors.Packages,
		"excuses":    cfg.Metrics.Collectors.Excuses,
		"cache":      cfg.Metrics.Collectors.Cache,
	} {
		if collector.RefreshInterval == "" {
			continue
		}
		if _, err := time.ParseDuration(collector.RefreshInterval); err != nil {
			return fmt.Errorf("otel.metrics.collectors.%s.refresh_interval: %w", name, err)
		}
	}
	return nil
}

func validateMetricsListener(prefix string, cfg OTelMetricsListenerConfig) error {
	if cfg.Path != "" && cfg.Path[0] != '/' {
		return fmt.Errorf("%s.path must start with /", prefix)
	}
	if cfg.Enabled && cfg.ListenAddr == "" {
		return fmt.Errorf("%s.listen_addr is required when enabled", prefix)
	}
	if cfg.DefaultRefreshInterval != "" {
		if _, err := time.ParseDuration(cfg.DefaultRefreshInterval); err != nil {
			return fmt.Errorf("%s.default_refresh_interval: %w", prefix, err)
		}
	}
	if len(cfg.LiveSystems) > 0 && prefix != "otel.metrics.domain" {
		return fmt.Errorf("%s.live_systems is only supported on otel.metrics.domain", prefix)
	}
	return nil
}

func validateSignalConfig(prefix string, cfg OTelSignalConfig, allowSampling bool) error {
	if cfg.Enabled && cfg.Endpoint == "" {
		return fmt.Errorf("%s.endpoint is required when enabled", prefix)
	}
	if cfg.Protocol != "" && cfg.Protocol != "grpc" && cfg.Protocol != "http" {
		return fmt.Errorf("%s.protocol must be grpc or http", prefix)
	}
	if allowSampling && cfg.SamplingRatio < 0 || cfg.SamplingRatio > 1 {
		return fmt.Errorf("%s.sampling_ratio must be between 0 and 1", prefix)
	}
	return nil
}

func validateForgeRef(prefix, forge, owner, host, project string) error {
	validForges := map[string]bool{
		"github":    true,
		"launchpad": true,
		"gerrit":    true,
	}

	if forge == "" {
		return fmt.Errorf("%s: forge is required", prefix)
	}
	if !validForges[forge] {
		return fmt.Errorf("%s: invalid forge %q (must be github, launchpad, or gerrit)", prefix, forge)
	}
	if project == "" {
		return fmt.Errorf("%s: project is required", prefix)
	}

	switch forge {
	case "github":
		if owner == "" {
			return fmt.Errorf("%s: owner is required for github", prefix)
		}
	case "gerrit":
		if host == "" {
			return fmt.Errorf("%s: host is required for gerrit", prefix)
		}
	}

	return nil
}
