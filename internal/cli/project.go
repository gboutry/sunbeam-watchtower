// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	lpadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/launchpad"
	projectsvc "github.com/gboutry/sunbeam-watchtower/internal/service/project"
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

			cfg := opts.Config
			if cfg == nil {
				return fmt.Errorf("no configuration loaded")
			}

			// Collect unique LP project names from bug tracker entries.
			seen := make(map[string]bool)
			var lpProjects []string
			for _, proj := range cfg.Projects {
				for _, b := range proj.Bugs {
					if b.Forge == "launchpad" && !seen[b.Project] {
						seen[b.Project] = true
						lpProjects = append(lpProjects, b.Project)
					}
				}
			}

			if len(lpProjects) == 0 {
				fmt.Fprintln(opts.Out, "no Launchpad projects found in configuration")
				return nil
			}

			lpClient := newLaunchpadClient(cfg.Launchpad, opts)
			if lpClient == nil {
				return fmt.Errorf("Launchpad authentication required (run: watchtower auth login)")
			}

			manager := lpadapter.NewProjectManager(lpClient)
			svc := projectsvc.NewService(
				manager,
				lpProjects,
				cfg.Launchpad.Series,
				cfg.Launchpad.DevelopmentFocus,
				opts.Logger,
			)

			result, err := svc.Sync(cmd.Context(), projectsvc.SyncOptions{
				Projects: projects,
				DryRun:   dryRun,
			})
			if err != nil {
				return err
			}

			for _, a := range result.Actions {
				switch a.ActionType {
				case projectsvc.ActionCreateSeries:
					if dryRun {
						fmt.Fprintf(opts.Out, "would create: series %q on project %q\n", a.Series, a.Project)
					} else {
						fmt.Fprintf(opts.Out, "created: series %q on project %q\n", a.Series, a.Project)
					}
				case projectsvc.ActionSetDevFocus:
					if dryRun {
						fmt.Fprintf(opts.Out, "would set: development focus to %q on project %q\n", a.Series, a.Project)
					} else {
						fmt.Fprintf(opts.Out, "set: development focus to %q on project %q\n", a.Series, a.Project)
					}
				case projectsvc.ActionDevFocusUnchanged:
					fmt.Fprintf(opts.Out, "unchanged: development focus already %q on project %q\n", a.Series, a.Project)
				}
			}

			for _, e := range result.Errors {
				fmt.Fprintf(opts.ErrOut, "error: %v\n", e)
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter to specific LP project names")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")

	return cmd
}
