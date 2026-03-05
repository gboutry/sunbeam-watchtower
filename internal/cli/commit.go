package cli

import (
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/service/commit"
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
			sources, err := buildCommitSources(opts)
			if err != nil {
				return err
			}

			svc := commit.NewService(sources, opts.Logger)

			listOpts := commit.ListOptions{
				Projects:   projects,
				Branch:     branch,
				Author:     author,
				IncludeMRs: includeMRs,
			}

			for _, f := range forges {
				ft, err := parseForgeType(f)
				if err != nil {
					return err
				}
				listOpts.Forges = append(listOpts.Forges, ft)
			}

			commits, results, err := svc.List(cmd.Context(), listOpts)
			if err != nil {
				return err
			}

			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: %v\n", r.Err)
				}
			}

			opts.Logger.Debug("commit log complete", "total_commits", len(commits))
			return renderCommits(opts.Out, opts.Output, commits)
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

			sources, err := buildCommitSources(opts)
			if err != nil {
				return err
			}

			svc := commit.NewService(sources, opts.Logger)

			listOpts := commit.ListOptions{
				Projects:   projects,
				Branch:     branch,
				BugID:      bugID,
				IncludeMRs: includeMRs,
			}

			for _, f := range forges {
				ft, err := parseForgeType(f)
				if err != nil {
					return err
				}
				listOpts.Forges = append(listOpts.Forges, ft)
			}

			commits, results, err := svc.List(cmd.Context(), listOpts)
			if err != nil {
				return err
			}

			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: %v\n", r.Err)
				}
			}

			return renderCommits(opts.Out, opts.Output, commits)
		},
	}

	cmd.Flags().StringVar(&bugID, "bug-id", "", "LP bug ID to track (required)")
	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&forges, "forge", nil, "filter by forge type: github, launchpad, gerrit (repeatable)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch to list commits from")
	cmd.Flags().BoolVar(&includeMRs, "include-mrs", false, "include commits from merge request refs")

	return cmd
}
