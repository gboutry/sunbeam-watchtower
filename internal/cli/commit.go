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
		projects []string
		forges   []string
		branch   string
		author   string
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "List commits across forges",
		RunE: func(cmd *cobra.Command, args []string) error {
			clients, err := buildCommitClients(opts)
			if err != nil {
				return err
			}

			svc := commit.NewService(clients)

			listOpts := commit.ListOptions{
				Projects: projects,
				Branch:   branch,
				Author:   author,
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

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&forges, "forge", nil, "filter by forge type: github, launchpad, gerrit (repeatable)")
	cmd.Flags().StringVar(&branch, "branch", "", "branch to list commits from")
	cmd.Flags().StringVar(&author, "author", "", "filter by author")

	return cmd
}

func newCommitTrackCmd(opts *Options) *cobra.Command {
	var (
		bugID    string
		projects []string
		forges   []string
		branch   string
	)

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Find commits referencing a bug ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if bugID == "" {
				return fmt.Errorf("--bug-id is required")
			}

			clients, err := buildCommitClients(opts)
			if err != nil {
				return err
			}

			svc := commit.NewService(clients)

			listOpts := commit.ListOptions{
				Projects: projects,
				Branch:   branch,
				BugID:    bugID,
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

	return cmd
}

// buildCommitClients converts forge clients to commit.ProjectForge map.
func buildCommitClients(opts *Options) (map[string]commit.ProjectForge, error) {
	reviewClients, err := buildForgeClients(opts)
	if err != nil {
		return nil, err
	}

	result := make(map[string]commit.ProjectForge, len(reviewClients))
	for name, pf := range reviewClients {
		result[name] = commit.ProjectForge{
			Forge:     pf.Forge,
			ProjectID: pf.ProjectID,
		}
	}
	return result, nil
}
