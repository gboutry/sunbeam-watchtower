package cli

import (
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/appclient"
	"github.com/spf13/cobra"
)

func newCommitCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "View commits across forges",
	}

	cmd.AddCommand(newCommitLogCmd(opts))
	cmd.AddCommand(newCommitTrackCmd(opts))
	return cmd
}

func newCommitLogCmd(opts *Options) *cobra.Command {
	var (
		projects   []string
		forges     []string
		branch     string
		author     string
		includeMRs bool
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "List commits across forges",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("commit log command started",
				"projects", projects,
				"forges", forges,
				"branch", branch,
				"author", author,
				"include_mrs", includeMRs,
			)

			result, err := opts.Client.CommitsList(cmd.Context(), appclient.CommitsListOptions{
				Projects:   projects,
				Forges:     forges,
				Branch:     branch,
				Author:     author,
				IncludeMRs: includeMRs,
			})
			if err != nil {
				return err
			}

			for _, w := range result.Warnings {
				fmt.Fprintf(opts.ErrOut, "warning: %s\n", w)
			}

			opts.Logger.Debug("commit log complete", "total_commits", len(result.Commits))
			return renderCommits(opts.Out, opts.Output, result.Commits)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&forges, "forge", nil, "filter by forge type: github, launchpad, gerrit (repeatable)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch to list commits from")
	cmd.Flags().StringVar(&author, "author", "", "filter by author")
	cmd.Flags().BoolVar(&includeMRs, "include-mrs", false, "include commits from merge request refs")

	return cmd
}

func newCommitTrackCmd(opts *Options) *cobra.Command {
	var (
		bugID      string
		projects   []string
		forges     []string
		branch     string
		includeMRs bool
	)

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Find commits referencing a bug ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("commit track command started", "bugID", bugID)
			if bugID == "" {
				return fmt.Errorf("--bug-id is required")
			}

			result, err := opts.Client.CommitsTrack(cmd.Context(), appclient.CommitsTrackOptions{
				BugID:      bugID,
				Projects:   projects,
				Forges:     forges,
				Branch:     branch,
				IncludeMRs: includeMRs,
			})
			if err != nil {
				return err
			}

			for _, w := range result.Warnings {
				fmt.Fprintf(opts.ErrOut, "warning: %s\n", w)
			}

			return renderCommits(opts.Out, opts.Output, result.Commits)
		},
	}

	cmd.Flags().StringVar(&bugID, "bug-id", "", "LP bug ID to track (required)")
	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&forges, "forge", nil, "filter by forge type: github, launchpad, gerrit (repeatable)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch to list commits from")
	cmd.Flags().BoolVar(&includeMRs, "include-mrs", false, "include commits from merge request refs")

	return cmd
}
