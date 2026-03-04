package cli

import (
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/service/bug"
	"github.com/spf13/cobra"
)

func newBugCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bug",
		Short: "Manage bugs across trackers",
	}

	cmd.AddCommand(newBugListCmd(opts))
	return cmd
}

func newBugListCmd(opts *Options) *cobra.Command {
	var (
		projects   []string
		status     []string
		importance []string
		assignee   string
		tags       []string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bug tasks across bug trackers",
		RunE: func(cmd *cobra.Command, args []string) error {
			trackers, projectMap, err := buildBugTrackers(opts)
			if err != nil {
				return err
			}

			svc := bug.NewService(trackers, projectMap)

			listOpts := bug.ListOptions{
				Projects:   projects,
				Status:     status,
				Importance: importance,
				Assignee:   assignee,
				Tags:       tags,
			}

			tasks, results, err := svc.List(cmd.Context(), listOpts)
			if err != nil {
				return err
			}

			// Report per-tracker errors.
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: %v\n", r.Err)
				}
			}

			return renderBugTasks(opts.Out, opts.Output, tasks)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&status, "status", nil, "filter by status: New, Confirmed, Triaged, In Progress, etc. (repeatable)")
	cmd.Flags().StringSliceVar(&importance, "importance", nil, "filter by importance: Critical, High, Medium, Low, etc. (repeatable)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "filter by assignee username")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "filter by tag (repeatable)")

	return cmd
}
