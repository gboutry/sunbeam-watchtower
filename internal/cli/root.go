package cli

import (
	"io"
	"log/slog"
	"os"

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
	root.AddCommand(newReviewCmd(opts))

	return root
}
