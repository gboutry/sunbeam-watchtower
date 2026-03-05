package cli

import (
	"fmt"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"github.com/gboutry/sunbeam-watchtower/internal/service/bug"
	"github.com/gboutry/sunbeam-watchtower/internal/service/bugsync"
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
			opts.Logger.Debug("bug show command started", "id", args[0])
			trackers, projectMap, err := buildBugTrackers(opts)
			if err != nil {
				return err
			}

			svc := bug.NewService(trackers, projectMap, opts.Logger)

			b, err := svc.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			return renderBugDetail(opts.Out, opts.Output, b)
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
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bug tasks across bug trackers",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("bug list command started")
			trackers, projectMap, err := buildBugTrackers(opts)
			if err != nil {
				return err
			}

			svc := bug.NewService(trackers, projectMap, opts.Logger)

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

func newBugSyncCmd(opts *Options) *cobra.Command {
	var (
		projects []string
		dryRun   bool
		days     int
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Update LP bug statuses from cached commits",
		Long:  "Scans cached commits for LP bug references and updates bug task statuses to Fix Committed. Also assigns bugs to the appropriate LP series based on which branches contain the fix.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("bug sync command started", "dry_run", dryRun)

			sources, err := buildCommitSources(opts)
			if err != nil {
				return err
			}

			trackers, _, err := buildBugTrackers(opts)
			if err != nil {
				return err
			}

			// Use the first available bug tracker and collect LP project names.
			var tracker port.BugTracker
			var lpProjects []string
			for _, pt := range trackers {
				if tracker == nil {
					tracker = pt.Tracker
				}
				lpProjects = append(lpProjects, pt.ProjectID)
			}
			if tracker == nil {
				return fmt.Errorf("no bug tracker configured")
			}

			// Build watchtower project → LP bug project mapping.
			lpProjectMap := make(map[string][]string)
			for _, proj := range opts.Config.Projects {
				for _, b := range proj.Bugs {
					if b.Forge == "launchpad" {
						lpProjectMap[proj.Name] = append(lpProjectMap[proj.Name], b.Project)
					}
				}
			}

			svc := bugsync.NewService(sources, tracker, lpProjects, lpProjectMap, opts.Logger)
			syncOpts := bugsync.SyncOptions{
				Projects: projects,
				DryRun:   dryRun,
			}
			if days > 0 {
				since := time.Now().AddDate(0, 0, -days)
				syncOpts.Since = &since
			}
			result, err := svc.Sync(cmd.Context(), syncOpts)
			if err != nil {
				return err
			}

			for _, a := range result.Actions {
				switch a.ActionType {
				case bugsync.ActionStatusUpdate:
					if dryRun {
						fmt.Fprintf(opts.Out, "would update: Bug #%s task %q %s → %s\n", a.BugID, a.TaskTitle, a.OldStatus, a.NewStatus)
					} else {
						fmt.Fprintf(opts.Out, "updated: Bug #%s task %q %s → %s\n", a.BugID, a.TaskTitle, a.OldStatus, a.NewStatus)
					}
				case bugsync.ActionSeriesAssignment:
					if dryRun {
						fmt.Fprintf(opts.Out, "would assign: Bug #%s to series %q on project %q\n", a.BugID, a.Series, a.Project)
					} else {
						fmt.Fprintf(opts.Out, "assigned: Bug #%s to series %q on project %q\n", a.BugID, a.Series, a.Project)
					}
				case bugsync.ActionAddProjectTask:
					if dryRun {
						fmt.Fprintf(opts.Out, "would add: Bug #%s task on project %q\n", a.BugID, a.Project)
					} else {
						fmt.Fprintf(opts.Out, "added: Bug #%s task on project %q\n", a.BugID, a.Project)
					}
				}
			}

			for _, e := range result.Errors {
				fmt.Fprintf(opts.ErrOut, "warning: %v\n", e)
			}

			if len(result.Actions) == 0 && len(result.Errors) == 0 {
				fmt.Fprintln(opts.Out, "No bugs to sync.")
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without updating")
	cmd.Flags().IntVar(&days, "days", 0, "only consider bugs created in the last N days")

	return cmd
}
