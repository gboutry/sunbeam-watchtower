package cli

import (
	"fmt"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/git"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"github.com/gboutry/sunbeam-watchtower/internal/service/build"
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

			builders, err := buildRecipeBuilders(opts)
			if err != nil {
				return err
			}

			repoMgr, err := buildRepoManager(opts)
			if err != nil {
				return err
			}

			gitClient := git.NewClient(opts.Logger)

			svc := build.NewService(builders, repoMgr, gitClient, opts.Logger)

			triggerOpts := build.TriggerOpts{
				Source:    source,
				Wait:      wait,
				Timeout:   timeout,
				Owner:     owner,
				Prefix:    prefix,
				LocalPath: localPath,
			}

			result, err := svc.Trigger(cmd.Context(), projectName, recipeNames, triggerOpts)
			if err != nil {
				return err
			}

			var builds []port.Build
			var requests []port.BuildRequest
			for _, r := range result.RecipeResults {
				builds = append(builds, r.Builds...)
				if r.BuildRequest != nil {
					requests = append(requests, *r.BuildRequest)
				}
			}

			if len(requests) > 0 {
				renderBuildRequests(opts.Out, opts.Output, requests)
			}
			if len(builds) > 0 {
				renderBuilds(opts.Out, opts.Output, builds)
			}

			return nil
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
	var state string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List builds across projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			builders, err := buildRecipeBuilders(opts)
			if err != nil {
				return err
			}

			svc := build.NewService(builders, nil, nil, opts.Logger)

			listOpts := build.ListOpts{
				Projects: projects,
				All:      all,
				State:    state,
			}

			builds, _, err := svc.List(cmd.Context(), listOpts)
			if err != nil {
				return err
			}

			return renderBuilds(opts.Out, opts.Output, builds)
		},
	}

	cmd.Flags().StringSliceVar(&projects, "project", nil, "filter by project name")
	cmd.Flags().BoolVar(&all, "all", false, "show all builds (not just active)")
	cmd.Flags().StringVar(&state, "state", "", "filter by state")

	return cmd
}

func newBuildDownloadCmd(opts *Options) *cobra.Command {
	var artifactsDir string

	cmd := &cobra.Command{
		Use:   "download <project> [recipes...]",
		Short: "Download build artifacts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			builders, err := buildRecipeBuilders(opts)
			if err != nil {
				return err
			}

			svc := build.NewService(builders, nil, nil, opts.Logger)

			projectName := args[0]
			recipeNames := args[1:]

			if artifactsDir == "" {
				artifactsDir = opts.Config.Build.ArtifactsDir
			}

			return svc.Download(cmd.Context(), projectName, recipeNames, artifactsDir)
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
			builders, err := buildRecipeBuilders(opts)
			if err != nil {
				return err
			}

			svc := build.NewService(builders, nil, nil, opts.Logger)

			cleanupOpts := build.CleanupOpts{
				Owner:  owner,
				Prefix: prefix,
				DryRun: dryRun,
			}
			if project != "" {
				cleanupOpts.Projects = []string{project}
			}

			deleted, err := svc.Cleanup(cmd.Context(), cleanupOpts)
			if err != nil {
				return err
			}

			for _, name := range deleted {
				if dryRun {
					fmt.Fprintf(opts.Out, "would delete: %s\n", name)
				} else {
					fmt.Fprintf(opts.Out, "deleted: %s\n", name)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "filter to specific project")
	cmd.Flags().StringVar(&owner, "owner", "", "LP owner")
	cmd.Flags().StringVar(&prefix, "prefix", "tmp-build", "recipe name prefix to match")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted")

	return cmd
}
