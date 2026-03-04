package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

			data, err := yaml.Marshal(opts.Config)
			if err != nil {
				return fmt.Errorf("marshalling config: %w", err)
			}

			fmt.Fprint(opts.Out, string(data))
			return nil
		},
	}
}
