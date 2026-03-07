package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
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
	var source, owner, prefix, localPath, artifactsDir string
	var wait, download, async bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "trigger <project> [artifacts...]",
		Short: "Trigger builds for a project",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			artifactNames := args[1:]

			// --download implies --wait.
			if download {
				wait = true
			}
			if async && wait {
				return fmt.Errorf("--async cannot be combined with --wait or --download")
			}

			triggerOpts := client.BuildsTriggerOptions{
				Project:   projectName,
				Artifacts: artifactNames,
				Wait:      wait,
				Timeout:   timeout.String(),
				Owner:     owner,
				Prefix:    prefix,
			}

			if source == "local" {
				preparer, err := newLocalBuildPreparer(opts)
				if err != nil {
					return err
				}
				triggerOpts, err = preparer.PrepareTrigger(cmd.Context(), triggerOpts, localPath)
				if err != nil {
					return err
				}
			}

			if async {
				job, err := opts.Client.BuildsTriggerAsync(cmd.Context(), triggerOpts)
				if err != nil {
					return err
				}
				return renderOperationJob(opts.Out, opts.Output, job)
			}

			result, err := opts.Client.BuildsTrigger(cmd.Context(), triggerOpts)
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

			// Download succeeded build artifacts when requested.
			if download && len(builds) > 0 {
				dlArtifacts := triggerOpts.Artifacts
				if len(dlArtifacts) == 0 {
					dlArtifacts = artifactNames
				}
				if err := opts.Client.BuildsDownload(cmd.Context(), client.BuildsDownloadOptions{
					Project:      projectName,
					Artifacts:    dlArtifacts,
					ArtifactsDir: artifactsDir,
				}); err != nil {
					errs = append(errs, fmt.Errorf("download: %w", err))
				}
			}

			return errors.Join(errs...)
		},
	}

	cmd.Flags().StringVar(&source, "source", "remote", "build source (remote|local)")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for builds to complete")
	cmd.Flags().BoolVar(&download, "download", false, "download artifacts after builds succeed (implies --wait)")
	cmd.Flags().BoolVar(&async, "async", false, "queue the build trigger as a long-running operation")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Hour, "max wait time")
	cmd.Flags().StringVar(&owner, "owner", "", "override LP owner")
	cmd.Flags().StringVar(&prefix, "prefix", "tmp-build", "temp recipe name prefix (local mode)")
	cmd.Flags().StringVar(&localPath, "local-path", ".", "path to local git repo (local mode)")
	cmd.Flags().StringVar(&artifactsDir, "artifacts-dir", "", "output directory for downloaded artifacts (default from config)")

	return cmd
}

func newBuildListCmd(opts *Options) *cobra.Command {
	var projects []string
	var all bool
	var state, source, sha, prefix, owner string

	cmd := &cobra.Command{
		Use:   "list [projects...]",
		Short: "List builds across projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Positional args and --project flag are merged.
			allProjects := append(projects, args...)

			listOpts := client.BuildsListOptions{
				Projects: allProjects,
				All:      all,
				State:    state,
			}

			if source == "local" {
				preparer, err := newLocalBuildPreparer(opts)
				if err != nil {
					return err
				}
				// Default to showing all builds for local source (user
				// typically wants to see completed results).
				if !cmd.Flags().Changed("all") {
					listOpts.All = true
				}
				// When a SHA is given, narrow the prefix to match only
				// that commit (e.g. "tmp-build-9e1ed720-").
				listPrefix := prefix
				if sha != "" {
					listPrefix = prefix + sha + "-"
				}
				listOpts, err = preparer.PrepareListByPrefix(cmd.Context(), listOpts, listPrefix)
				if err != nil {
					return err
				}
			}
			if owner != "" {
				listOpts.Owner = owner
			}

			builds, err := opts.Client.BuildsList(cmd.Context(), listOpts)
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
	cmd.Flags().StringVar(&sha, "sha", "", "git commit SHA for local build lookup")
	cmd.Flags().StringVar(&prefix, "prefix", "tmp-build", "temp recipe name prefix (local mode)")
	cmd.Flags().StringVar(&owner, "owner", "", "override LP owner")

	return cmd
}

func newBuildDownloadCmd(opts *Options) *cobra.Command {
	var artifactsDir, source, sha, prefix, owner string

	cmd := &cobra.Command{
		Use:   "download <project> [artifacts...]",
		Short: "Download build artifacts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			artifactNames := args[1:]

			dlOpts := client.BuildsDownloadOptions{
				Project:      projectName,
				Artifacts:    artifactNames,
				ArtifactsDir: artifactsDir,
			}

			if source == "local" {
				preparer, err := newLocalBuildPreparer(opts)
				if err != nil {
					return err
				}
				listPrefix := prefix
				if sha != "" {
					listPrefix = prefix + sha + "-"
				}
				dlOpts, err = preparer.PrepareDownloadByPrefix(cmd.Context(), dlOpts, listPrefix)
				if err != nil {
					return err
				}
			}
			if owner != "" {
				dlOpts.Owner = owner
			}

			return opts.Client.BuildsDownload(cmd.Context(), dlOpts)
		},
	}

	cmd.Flags().StringVar(&artifactsDir, "artifacts-dir", "", "output directory (default from config)")
	cmd.Flags().StringVar(&source, "source", "remote", "build source (remote|local)")
	cmd.Flags().StringVar(&sha, "sha", "", "git commit SHA (narrows prefix in local mode)")
	cmd.Flags().StringVar(&prefix, "prefix", "tmp-build-", "temp recipe name prefix (local mode)")
	cmd.Flags().StringVar(&owner, "owner", "", "override LP owner")

	return cmd
}

func newLocalBuildPreparer(opts *Options) (*frontend.LocalBuildPreparer, error) {
	repoMgr, err := opts.App.BuildRepoManager()
	if err != nil {
		return nil, fmt.Errorf("init repo manager: %w", err)
	}
	if repoMgr == nil {
		return nil, app.ErrLaunchpadAuthRequired
	}

	builders, err := opts.App.BuildRecipeBuilders()
	if err != nil {
		return nil, fmt.Errorf("init recipe builders: %w", err)
	}

	return frontend.NewLocalBuildPreparer(opts.App.GitClient(), repoMgr, builders), nil
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
