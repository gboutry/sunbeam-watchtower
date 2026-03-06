package cli

import (
	"errors"
	"fmt"
	"time"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	"github.com/spf13/cobra"
)

func newBuildCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Manage Launchpad builds",
	}
	cmd.AddCommand(
		newBuildTriggerCmd(opts),
		newBuildListCmd(opts),
		newBuildDownloadCmd(opts),
		newBuildCleanupCmd(opts),
	)
	return cmd
}

func newBuildTriggerCmd(opts *Options) *cobra.Command {
	var source, owner, prefix, localPath string
	var wait bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "trigger <project> [recipes...]",
		Short: "Trigger builds for a project",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			recipeNames := args[1:]

			result, err := opts.Client.BuildsTrigger(cmd.Context(), client.BuildsTriggerOptions{
				Project:   projectName,
				Recipes:   recipeNames,
				Source:    source,
				Wait:      wait,
				Timeout:   timeout.String(),
				Owner:     owner,
				Prefix:    prefix,
				LocalPath: localPath,
			})
			if err != nil {
				return err
			}

			var builds []dto.Build
			var requests []dto.BuildRequest
			var errs []error
			for _, r := range result.RecipeResults {
				builds = append(builds, r.Builds...)
				if r.BuildRequest != nil {
					requests = append(requests, *r.BuildRequest)
				}
				if r.ErrorMessage != "" {
					errs = append(errs, fmt.Errorf("recipe %s: %s", r.Name, r.ErrorMessage))
				}
			}

			if len(requests) > 0 {
				if err := renderBuildRequests(opts.Out, opts.Output, requests); err != nil {
					return err
				}
			}
			if len(builds) > 0 {
				if err := renderBuilds(opts.Out, opts.Output, builds); err != nil {
					return err
				}
			}

			return errors.Join(errs...)
		},
	}

	cmd.Flags().StringVar(&source, "source", "remote", "build source (remote|local)")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for builds to complete")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Hour, "max wait time")
	cmd.Flags().StringVar(&owner, "owner", "", "override LP owner")
	cmd.Flags().StringVar(&prefix, "prefix", "tmp-build", "temp recipe name prefix (local mode)")
	cmd.Flags().StringVar(&localPath, "local-path", ".", "path to local git repo (local mode)")

	return cmd
}

func newBuildListCmd(opts *Options) *cobra.Command {
	var projects []string
	var all bool
	var state, source, prefix, localPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List builds across projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			builds, err := opts.Client.BuildsList(cmd.Context(), client.BuildsListOptions{
				Projects:  projects,
				All:       all,
				State:     state,
				Source:    source,
				LocalPath: localPath,
				Prefix:    prefix,
			})
			if err != nil {
				return err
			}

			return renderBuilds(opts.Out, opts.Output, builds)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name")
	cmd.Flags().BoolVar(&all, "all", false, "show all builds (not just active)")
	cmd.Flags().StringVar(&state, "state", "", "filter by state")
	cmd.Flags().StringVar(&source, "source", "remote", "build source (remote|local)")
	cmd.Flags().StringVar(&localPath, "local-path", ".", "path to local git repo (local mode)")
	cmd.Flags().StringVar(&prefix, "prefix", "tmp-build", "temp recipe name prefix (local mode)")

	return cmd
}

func newBuildDownloadCmd(opts *Options) *cobra.Command {
	var artifactsDir string

	cmd := &cobra.Command{
		Use:   "download <project> [recipes...]",
		Short: "Download build artifacts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			recipeNames := args[1:]

			return opts.Client.BuildsDownload(cmd.Context(), client.BuildsDownloadOptions{
				Project:      projectName,
				Recipes:      recipeNames,
				ArtifactsDir: artifactsDir,
			})
		},
	}

	cmd.Flags().StringVar(&artifactsDir, "artifacts-dir", "", "output directory (default from config)")

	return cmd
}

func newBuildCleanupCmd(opts *Options) *cobra.Command {
	var project, owner, prefix string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete temporary build recipes",
		RunE: func(cmd *cobra.Command, args []string) error {
			deleted, err := opts.Client.BuildsCleanup(cmd.Context(), client.BuildsCleanupOptions{
				Project: project,
				Owner:   owner,
				Prefix:  prefix,
				DryRun:  dryRun,
			})
			if err != nil {
				return err
			}

			return renderStringList(opts.Out, opts.Output, deleted)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "filter to specific project")
	cmd.Flags().StringVar(&owner, "owner", "", "LP owner")
	cmd.Flags().StringVar(&prefix, "prefix", "tmp-build", "recipe name prefix to match")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted")

	return cmd
}
