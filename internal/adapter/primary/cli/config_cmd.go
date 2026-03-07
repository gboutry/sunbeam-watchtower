package cli

import "github.com/spf13/cobra"

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
			cfg, err := opts.Frontend().Config().Show(cmd.Context())
			if err != nil {
				return err
			}

			switch opts.Output {
			case "json":
				return renderJSON(opts.Out, cfg)
			default:
				return renderYAML(opts.Out, cfg)
			}
		},
	}
}
