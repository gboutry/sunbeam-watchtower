package cli

import "github.com/spf13/cobra"

func newOperationCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "operation",
		Aliases: []string{"operations"},
		Short:   "Inspect and control long-running operations",
	}
	cmd.AddCommand(
		newOperationListCmd(opts),
		newOperationShowCmd(opts),
		newOperationEventsCmd(opts),
		newOperationCancelCmd(opts),
	)
	return cmd
}

func newOperationListCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List long-running operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			jobs, err := opts.Client.OperationsList(cmd.Context())
			if err != nil {
				return err
			}
			return renderOperationJobs(opts.Out, opts.Output, jobs)
		},
	}
}

func newOperationShowCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			job, err := opts.Client.OperationGet(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderOperationJob(opts.Out, opts.Output, job)
		},
	}
}

func newOperationEventsCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "events <id>",
		Short: "Show the event history for one operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := opts.Client.OperationEvents(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderOperationEvents(opts.Out, opts.Output, events)
		},
	}
}

func newOperationCancelCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Request cancellation for one operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			job, err := opts.Client.OperationCancel(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderOperationJob(opts.Out, opts.Output, job)
		},
	}
}
