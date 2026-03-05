package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newConfigCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(newConfigShowCmd(opts))
	return cmd
}

func newConfigShowCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display the current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Config == nil {
				return fmt.Errorf("no configuration loaded")
			}

			switch opts.Output {
			case "json":
				return renderJSON(opts.Out, opts.Config)
			default:
				return renderYAML(opts.Out, opts.Config)
			}
		},
	}
}
