// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/appclient"
	dto "github.com/gboutry/sunbeam-watchtower/internal/dto/v1"
	"github.com/spf13/cobra"
)

func newProjectCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage LP project metadata",
	}
	cmd.AddCommand(newProjectSyncCmd(opts))
	return cmd
}

func newProjectSyncCmd(opts *Options) *cobra.Command {
	var (
		projects []string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Ensure LP projects have declared series and development focus",
		Long:  "Iterates over all unique LP projects from bug tracker config entries, ensures each declared series exists (creating if missing), and sets the development focus to the configured series.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("project sync command started", "dry_run", dryRun)

			result, err := opts.Client.ProjectsSync(cmd.Context(), appclient.ProjectsSyncOptions{
				Projects: projects,
				DryRun:   dryRun,
			})
			if err != nil {
				return err
			}

			for _, e := range result.Errors {
				fmt.Fprintf(opts.ErrOut, "error: %s\n", e)
			}

			syncResult := &dto.ProjectSyncResult{
				Actions: result.Actions,
			}
			return renderProjectSyncResult(opts.Out, opts.Output, syncResult, dryRun)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter to specific LP project names")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")

	return cmd
}
