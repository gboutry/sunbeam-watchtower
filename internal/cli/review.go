package cli

import (
	"fmt"
	"strings"

	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/service/review"
	"github.com/spf13/cobra"
)

func newReviewCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Manage merge requests and reviews",
	}

	cmd.AddCommand(newReviewListCmd(opts))
	return cmd
}

func newReviewListCmd(opts *Options) *cobra.Command {
	var (
		projects []string
		forges   []string
		state    string
		author   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List merge requests across forges",
		RunE: func(cmd *cobra.Command, args []string) error {
			clients, err := buildForgeClients(opts)
			if err != nil {
				return err
			}

			svc := review.NewService(clients)

			listOpts := review.ListOptions{
				Projects: projects,
				Author:   author,
			}

			if state != "" {
				s, err := parseMergeState(state)
				if err != nil {
					return err
				}
				listOpts.State = s
			}

			for _, f := range forges {
				ft, err := parseForgeType(f)
				if err != nil {
					return err
				}
				listOpts.Forges = append(listOpts.Forges, ft)
			}

			mrs, results, err := svc.List(cmd.Context(), listOpts)
			if err != nil {
				return err
			}

			// Report per-repo errors.
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: %v\n", r.Err)
				}
			}

			return renderMergeRequests(opts.Out, opts.Output, mrs)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().StringSliceVar(&forges, "forge", nil, "filter by forge type: github, launchpad, gerrit (repeatable)")
	cmd.Flags().StringVar(&state, "state", "", "filter by state: open, merged, closed, wip, abandoned")
	cmd.Flags().StringVar(&author, "author", "", "filter by author")

	return cmd
}

func parseMergeState(s string) (forge.MergeState, error) {
	switch strings.ToLower(s) {
	case "open":
		return forge.MergeStateOpen, nil
	case "merged":
		return forge.MergeStateMerged, nil
	case "closed":
		return forge.MergeStateClosed, nil
	case "wip":
		return forge.MergeStateWIP, nil
	case "abandoned":
		return forge.MergeStateAbandoned, nil
	default:
		return 0, fmt.Errorf("invalid state %q (valid: open, merged, closed, wip, abandoned)", s)
	}
}

func parseForgeType(s string) (forge.ForgeType, error) {
	switch strings.ToLower(s) {
	case "github":
		return forge.ForgeGitHub, nil
	case "launchpad":
		return forge.ForgeLaunchpad, nil
	case "gerrit":
		return forge.ForgeGerrit, nil
	default:
		return 0, fmt.Errorf("invalid forge %q (valid: github, launchpad, gerrit)", s)
	}
}
