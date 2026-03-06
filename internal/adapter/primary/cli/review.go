package cli

import (
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	"github.com/spf13/cobra"
)

func newReviewCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Manage merge requests and reviews",
	}

	cmd.AddCommand(newReviewListCmd(opts))
	cmd.AddCommand(newReviewShowCmd(opts))
	return cmd
}

func newReviewShowCmd(opts *Options) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a merge request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("review show command started", "project", project, "id", args[0])
			if project == "" {
				return fmt.Errorf("--project is required for review show")
			}

			mr, err := opts.Client.ReviewsGet(cmd.Context(), project, args[0])
			if err != nil {
				return err
			}

			return renderMergeRequestDetail(opts.Out, opts.Output, mr)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "project name (required)")

	return cmd
}

func newReviewListCmd(opts *Options) *cobra.Command {
	var (
		projects []string
		forges   []string
		state    string
		author   string
		since    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List merge requests across forges",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedSince, err := dto.ResolveSince(since)
			if err != nil {
				return err
			}

			opts.Logger.Debug("review list command started",
				"projects", projects,
				"forges", forges,
				"state", state,
				"author", author,
				"since", resolvedSince,
			)

			result, err := opts.Client.ReviewsList(cmd.Context(), client.ReviewsListOptions{
				Projects: projects,
				Forges:   forges,
				State:    state,
				Author:   author,
				Since:    resolvedSince,
			})
			if err != nil {
				return err
			}

			for _, w := range result.Warnings {
				fmt.Fprintf(opts.ErrOut, "warning: %s\n", w)
			}

			return renderMergeRequests(opts.Out, opts.Output, result.MergeRequests)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&forges, "forge", nil, "filter by forge type: github, launchpad, gerrit (repeatable)")
	cmd.Flags().StringVar(&state, "state", "", "filter by state: open, merged, closed, wip, abandoned")
	cmd.Flags().StringVar(&author, "author", "", "filter by author")
	cmd.Flags().StringVar(&since, "since", "", "show only MRs updated since (e.g. 2d, 1w, 30m, 2025-01-01)")

	return cmd
}
