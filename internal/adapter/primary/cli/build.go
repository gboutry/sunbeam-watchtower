package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
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
				if err := prepareLocalTrigger(cmd, opts, projectName, artifactNames, localPath, prefix, &triggerOpts); err != nil {
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

// prepareLocalTrigger resolves local git + LP resources and populates triggerOpts.
func prepareLocalTrigger(cmd *cobra.Command, opts *Options, projectName string, artifactNames []string, localPath, prefix string, triggerOpts *client.BuildsTriggerOptions) error {
	ctx := cmd.Context()
	app := opts.App

	gitClient := app.GitClient()
	repoMgr, err := app.BuildRepoManager()
	if err != nil {
		return fmt.Errorf("init repo manager: %w", err)
	}
	builders, err := app.BuildRecipeBuilders()
	if err != nil {
		return fmt.Errorf("init recipe builders: %w", err)
	}
	pb, ok := builders[projectName]
	if !ok {
		return fmt.Errorf("unknown project %q", projectName)
	}

	// Resolve owner.
	lpOwner := triggerOpts.Owner
	if lpOwner == "" {
		lpOwner, err = repoMgr.GetCurrentUser(ctx)
		if err != nil {
			return fmt.Errorf("get current LP user: %w", err)
		}
	}
	triggerOpts.Owner = lpOwner

	// Resolve HEAD SHA.
	sha, err := gitClient.HeadSHA(localPath)
	if err != nil {
		return fmt.Errorf("resolve HEAD SHA: %w", err)
	}
	shortSHA := sha[:8]

	// Discover artifacts if not specified.
	if len(artifactNames) == 0 {
		artifactNames, err = pb.Strategy.DiscoverRecipes(localPath)
		if err != nil {
			return fmt.Errorf("discover artifacts: %w", err)
		}
	}

	// Compute temp recipe names, build paths.
	tempNames := make([]string, 0, len(artifactNames))
	buildPaths := make(map[string]string, len(artifactNames))
	for _, name := range artifactNames {
		tempName := pb.Strategy.TempRecipeName(name, sha, prefix)
		tempNames = append(tempNames, tempName)
		buildPaths[tempName] = pb.Strategy.BuildPath(name)
	}
	triggerOpts.Artifacts = tempNames
	triggerOpts.BuildPaths = buildPaths

	// Ensure LP project + repo exist.
	lpProject, err := repoMgr.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return fmt.Errorf("get/create LP project: %w", err)
	}
	triggerOpts.LPProject = lpProject

	repoSelfLink, gitSSHURL, err := repoMgr.GetOrCreateRepo(ctx, lpOwner, lpProject, projectName)
	if err != nil {
		return fmt.Errorf("get/create LP repo: %w", err)
	}
	triggerOpts.RepoSelfLink = repoSelfLink

	// Push code to LP repo.
	if err := pushToLP(gitClient, localPath, gitSSHURL, lpOwner, shortSHA); err != nil {
		return fmt.Errorf("push to LP: %w", err)
	}

	// Wait for the git ref to appear on LP.
	tmpBranch := "refs/heads/tmp-" + shortSHA
	refLink, err := repoMgr.WaitForGitRef(ctx, repoSelfLink, tmpBranch, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("wait for git ref: %w", err)
	}

	// All temp recipes point to the same ref.
	gitRefLinks := make(map[string]string, len(tempNames))
	for _, name := range tempNames {
		gitRefLinks[name] = refLink
	}
	triggerOpts.GitRefLinks = gitRefLinks

	return nil
}

// pushToLP pushes the current HEAD to a Launchpad git repo as both the main
// branch and a tmp-<sha> branch. The remote is added/removed automatically.
func pushToLP(gitClient port.GitClient, localPath, gitSSHURL, lpOwner, shortSHA string) error {
	// Fixup git+ssh:// → ssh:// and inject username.
	sshURL := strings.Replace(gitSSHURL, "git+ssh://", "ssh://", 1)
	if !strings.Contains(sshURL, "@") {
		sshURL = strings.Replace(sshURL, "ssh://", "ssh://"+lpOwner+"@", 1)
	}

	const remoteName = "watchtower-tmp"
	_ = gitClient.RemoveRemote(localPath, remoteName)
	if err := gitClient.AddRemote(localPath, remoteName, sshURL); err != nil {
		return fmt.Errorf("add remote: %w", err)
	}
	defer func() { _ = gitClient.RemoveRemote(localPath, remoteName) }()

	// Push HEAD to both the main branch and a tmp branch.
	tmpBranch := "refs/heads/tmp-" + shortSHA
	if err := gitClient.Push(localPath, remoteName, "HEAD", "refs/heads/main", true); err != nil {
		return fmt.Errorf("push main: %w", err)
	}
	if err := gitClient.Push(localPath, remoteName, "HEAD", tmpBranch, true); err != nil {
		return fmt.Errorf("push tmp branch: %w", err)
	}

	return nil
}

// prepareLocalListByPrefix resolves owner and LP project, then sets
// RecipePrefix so the service discovers recipes via ListRecipesByOwner.
func prepareLocalListByPrefix(cmd *cobra.Command, opts *Options, prefix string, listOpts *client.BuildsListOptions) error {
	ctx := cmd.Context()
	app := opts.App

	repoMgr, err := app.BuildRepoManager()
	if err != nil {
		return fmt.Errorf("init repo manager: %w", err)
	}

	// Resolve owner.
	lpOwner := listOpts.Owner
	if lpOwner == "" {
		lpOwner, err = repoMgr.GetCurrentUser(ctx)
		if err != nil {
			return fmt.Errorf("get current LP user: %w", err)
		}
		listOpts.Owner = lpOwner
	}

	// Resolve LP project for local builds.
	lpProject, err := repoMgr.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return fmt.Errorf("get LP project: %w", err)
	}
	listOpts.LPProject = lpProject
	listOpts.RecipePrefix = prefix

	return nil
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
				if err := prepareLocalListByPrefix(cmd, opts, listPrefix, &listOpts); err != nil {
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
				listPrefix := prefix
				if sha != "" {
					listPrefix = prefix + sha + "-"
				}

				repoMgr, err := opts.App.BuildRepoManager()
				if err != nil {
					return fmt.Errorf("init repo manager: %w", err)
				}
				ctx := cmd.Context()

				lpOwner := owner
				if lpOwner == "" {
					lpOwner, err = repoMgr.GetCurrentUser(ctx)
					if err != nil {
						return fmt.Errorf("get current LP user: %w", err)
					}
				}
				dlOpts.Owner = lpOwner

				lpProject, err := repoMgr.GetOrCreateProject(ctx, lpOwner)
				if err != nil {
					return fmt.Errorf("get LP project: %w", err)
				}
				dlOpts.LPProject = lpProject
				dlOpts.RecipePrefix = listPrefix
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
