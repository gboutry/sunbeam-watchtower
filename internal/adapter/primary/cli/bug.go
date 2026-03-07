package cli

import (
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/spf13/cobra"
)

func newBugCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bug",
		Short: "Manage bugs across trackers",
	}

	cmd.AddCommand(newBugListCmd(opts))
	cmd.AddCommand(newBugShowCmd(opts))
	cmd.AddCommand(newBugSyncCmd(opts))
	return cmd
}

func newBugShowCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a bug and its tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := frontend.NewBugClientWorkflow(opts.Client).Show(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return renderBugDetail(opts.Out, opts.Output, result)
		},
	}

	return cmd
}

func newBugListCmd(opts *Options) *cobra.Command {
	var (
		projects   []string
		status     []string
		importance []string
		assignee   string
		tags       []string
		since      string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bug tasks across bug trackers",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := frontend.NewBugClientWorkflow(opts.Client).List(cmd.Context(), frontend.BugListRequest{
				Projects:   projects,
				Status:     status,
				Importance: importance,
				Assignee:   assignee,
				Tags:       tags,
				Since:      since,
			})
			if err != nil {
				return err
			}
			for _, w := range result.Warnings {
				fmt.Fprintf(opts.ErrOut, "warning: %s\n", w)
			}
			return renderBugTasks(opts.Out, opts.Output, result.Tasks)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&status, "status", nil, "filter by status: New, Confirmed, Triaged, In Progress, etc. (repeatable)")
	cmd.Flags().StringSliceVar(&importance, "importance", nil, "filter by importance: Critical, High, Medium, Low, etc. (repeatable)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "filter by assignee username")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "filter by tag (repeatable)")
	cmd.Flags().StringVar(&since, "since", "", "show only bugs created/modified since (e.g. 2d, 1w, 30m, 2025-01-01)")

	return cmd
}

func newBugSyncCmd(opts *Options) *cobra.Command {
	var (
		projects []string
		dryRun   bool
		since    string
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Update LP bug statuses from cached commits",
		Long:  "Scans cached commits for LP bug references and updates bug task statuses to Fix Committed. Also assigns bugs to the appropriate LP series based on which branches contain the fix.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := frontend.NewBugClientWorkflow(opts.Client).Sync(cmd.Context(), frontend.BugSyncRequest{
				Projects: projects,
				DryRun:   dryRun,
				Since:    since,
			})
			if err != nil {
				return err
			}
			for _, e := range result.Warnings {
				fmt.Fprintf(opts.ErrOut, "warning: %s\n", e)
			}
			return renderBugSyncResult(opts.Out, opts.Output, result.Result, dryRun)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without updating")
	cmd.Flags().StringVar(&since, "since", "", "only consider bugs created/modified since (e.g. 2d, 1w, 30m, 2025-01-01)")

	return cmd
}
