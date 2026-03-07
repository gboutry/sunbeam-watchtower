package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	UseKeyring bool `mapstructure:"use_keyring" yaml:"use_keyring"`
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
	Tracks   []string                     `mapstructure:"tracks" yaml:"tracks,omitempty"`
	TrackMap map[string]string            `mapstructure:"track_map" yaml:"track_map,omitempty"`
	Branches []ProjectReleaseBranchConfig `mapstructure:"branches" yaml:"branches,omitempty"`
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

// Config is the top-level configuration.
type Config struct {
	Launchpad LaunchpadConfig `mapstructure:"launchpad" yaml:"launchpad"`
	GitHub    GitHubConfig    `mapstructure:"github" yaml:"github"`
	Gerrit    GerritConfig    `mapstructure:"gerrit" yaml:"gerrit"`
	Projects  []ProjectConfig `mapstructure:"projects" yaml:"projects"`
	Build     BuildConfig     `mapstructure:"build" yaml:"build"`
	Packages  PackagesConfig  `mapstructure:"packages" yaml:"packages,omitempty"`
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
