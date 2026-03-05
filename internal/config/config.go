package config

import (
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
	Owner          string   `mapstructure:"owner" yaml:"owner"`
	Recipes        []string `mapstructure:"recipes" yaml:"recipes,omitempty"`
	PrepareCommand string   `mapstructure:"prepare_command" yaml:"prepare_command,omitempty"`
}

// ProjectConfig defines a project tracked across forges.
type ProjectConfig struct {
	Name             string              `mapstructure:"name" yaml:"name"`
	ArtifactType     string              `mapstructure:"artifact_type" yaml:"artifact_type,omitempty"`
	Code             CodeConfig          `mapstructure:"code" yaml:"code"`
	Bugs             []BugTrackerConfig  `mapstructure:"bugs" yaml:"bugs,omitempty"`
	Build            *ProjectBuildConfig `mapstructure:"build" yaml:"build,omitempty"`
	Series           []string            `mapstructure:"series" yaml:"series,omitempty"`
	DevelopmentFocus string              `mapstructure:"development_focus" yaml:"development_focus,omitempty"`
}

// BuildConfig holds build pipeline settings.
type BuildConfig struct {
	DefaultPrefix  string `mapstructure:"default_prefix" yaml:"default_prefix"`
	TimeoutMinutes int    `mapstructure:"timeout_minutes" yaml:"timeout_minutes"`
	ArtifactsDir   string `mapstructure:"artifacts_dir" yaml:"artifacts_dir"`
}

// Config is the top-level configuration.
type Config struct {
	Launchpad LaunchpadConfig `mapstructure:"launchpad" yaml:"launchpad"`
	GitHub    GitHubConfig    `mapstructure:"github" yaml:"github"`
	Gerrit    GerritConfig    `mapstructure:"gerrit" yaml:"gerrit"`
	Projects  []ProjectConfig `mapstructure:"projects" yaml:"projects"`
	Build     BuildConfig     `mapstructure:"build" yaml:"build"`
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
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
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
			if p.Build.Owner == "" {
				return fmt.Errorf("projects[%d] (%s): build.owner is required", i, p.Name)
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
