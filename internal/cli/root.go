package cli

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/spf13/cobra"
)

// Options holds resolved CLI state shared across commands.
type Options struct {
	Config     *config.Config
	ConfigPath string
	Verbose    bool
	Output     string // "table", "json", "yaml"
	NoColor    bool
	Logger     *slog.Logger
	Out        io.Writer
	ErrOut     io.Writer
}

// envDefaults applies WATCHTOWER_* environment variables as defaults.
func envDefaults(opts *Options) {
	if v := os.Getenv("WATCHTOWER_CONFIG"); v != "" && opts.ConfigPath == "" {
		opts.ConfigPath = v
	}
	if v := os.Getenv("WATCHTOWER_VERBOSE"); v != "" && !opts.Verbose {
		opts.Verbose = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("WATCHTOWER_OUTPUT"); v != "" && opts.Output == "table" {
		opts.Output = v
	}
	if v := os.Getenv("WATCHTOWER_NO_COLOR"); v != "" && !opts.NoColor {
		opts.NoColor = strings.EqualFold(v, "true") || v == "1"
	}
}

// NewRootCmd creates the root watchtower command with all subcommands.
func NewRootCmd(opts *Options) *cobra.Command {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.ErrOut == nil {
		opts.ErrOut = os.Stderr
	}

	root := &cobra.Command{
		Use:           "watchtower",
		Short:         "Unified dashboard for Sunbeam across GitHub, Launchpad, and Gerrit",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Apply env var defaults for flags not explicitly set.
			envDefaults(opts)

			// Skip config loading for commands that don't need it.
			if cmd.Name() == "version" {
				return nil
			}

			level := slog.LevelInfo
			if opts.Verbose {
				level = slog.LevelDebug
			}
			opts.Logger = slog.New(slog.NewTextHandler(opts.ErrOut, &slog.HandlerOptions{Level: level}))

			cfg, err := config.Load(opts.ConfigPath)
			if err != nil {
				return err
			}
			opts.Config = cfg
			return nil
		},
	}

	root.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	root.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "enable debug logging")
	root.PersistentFlags().StringVarP(&opts.Output, "output", "o", "table", "output format: table, json, yaml")
	root.PersistentFlags().BoolVar(&opts.NoColor, "no-color", false, "disable colored output")

	root.AddCommand(newVersionCmd(opts))
	root.AddCommand(newConfigCmd(opts))
	root.AddCommand(newAuthCmd(opts))
	root.AddCommand(newReviewCmd(opts))
	root.AddCommand(newCommitCmd(opts))
	root.AddCommand(newBugCmd(opts))
	root.AddCommand(newBuildCmd(opts))
	root.AddCommand(newCacheCmd(opts))
	root.AddCommand(newProjectCmd(opts))
	root.AddCommand(newPackagesCmd(opts))

	return root
}
