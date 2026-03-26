// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// PreparedBuildTriggerRequest holds build trigger fields after frontend-side preparation.
type PreparedBuildTriggerRequest struct {
	Project   string
	Artifacts []string
	Wait      bool
	Timeout   time.Duration
	Owner     string
	Prefix    string
	Prepared  *dto.PreparedBuildSource
}

// PreparedBuildListRequest holds build list fields after frontend-side preparation.
type PreparedBuildListRequest struct {
	Projects     []string
	All          bool
	State        string
	Owner        string
	TargetRef    string
	RecipePrefix string
}

// PreparedBuildDownloadRequest holds build download fields after frontend-side preparation.
type PreparedBuildDownloadRequest struct {
	Project      string
	Artifacts    []string
	ArtifactsDir string
	Owner        string
	TargetRef    string
	RecipePrefix string
}

// LocalBuildPreparer handles frontend-side local preparation for split build workflows.
type LocalBuildPreparer struct {
	gitClient   port.GitClient
	repoManager port.RepoManager
	builders    map[string]build.ProjectBuilder
	cmdRunner   port.CommandRunner
}

// NewLocalBuildPreparer creates a reusable local build preparer.
func NewLocalBuildPreparer(
	gitClient port.GitClient,
	repoManager port.RepoManager,
	builders map[string]build.ProjectBuilder,
	cmdRunner port.CommandRunner,
) *LocalBuildPreparer {
	return &LocalBuildPreparer{
		gitClient:   gitClient,
		repoManager: repoManager,
		builders:    builders,
		cmdRunner:   cmdRunner,
	}
}

// PrepareTrigger resolves local git and Launchpad state and returns a prepared build-trigger request.
func (p *LocalBuildPreparer) PrepareTrigger(
	ctx context.Context,
	req PreparedBuildTriggerRequest,
	localPath string,
) (PreparedBuildTriggerRequest, error) {
	if p.repoManager == nil {
		return req, app.ErrLaunchpadAuthRequired
	}
	pb, ok := p.builders[req.Project]
	if !ok {
		return req, fmt.Errorf("unknown project %q", req.Project)
	}

	// 1. Resolve HEAD SHA from local clone.
	sha, err := p.gitClient.HeadSHA(localPath)
	if err != nil {
		return req, fmt.Errorf("resolve HEAD SHA: %w", err)
	}
	shortSHA := sha
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}

	// 2. Resolve LP owner.
	lpOwner := req.Owner
	if lpOwner == "" {
		lpOwner, err = p.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return req, fmt.Errorf("get current LP user: %w", err)
		}
	}
	req.Owner = lpOwner

	// 3. Get or create personal LP project and repo.
	lpProject, err := p.repoManager.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return req, fmt.Errorf("get/create LP project: %w", err)
	}

	repoSelfLink, gitSSHURL, err := p.repoManager.GetOrCreateRepo(ctx, lpOwner, lpProject, req.Project)
	if err != nil {
		return req, fmt.Errorf("get/create LP repo: %w", err)
	}

	// 4. Build branch name.
	branchName := "tmp-" + req.Prefix + "-" + shortSHA
	refPath := "refs/heads/" + branchName

	// 5. Check if branch already exists on LP.
	refLink, err := p.repoManager.GetGitRef(ctx, repoSelfLink, refPath)
	if err != nil {
		// 6. Branch doesn't exist — prepare and push.
		if err := p.prepareAndPush(ctx, localPath, gitSSHURL, lpOwner, branchName, sha, pb.PrepareCommand); err != nil {
			return req, fmt.Errorf("prepare and push: %w", err)
		}

		// 7. Wait for git ref on LP.
		refLink, err = p.repoManager.WaitForGitRef(ctx, repoSelfLink, refPath, 2*time.Minute)
		if err != nil {
			return req, fmt.Errorf("wait for git ref: %w", err)
		}
	}

	// 8. Discover artifacts from local clone.
	artifactNames := req.Artifacts
	if len(artifactNames) == 0 {
		artifactNames, err = pb.Strategy.DiscoverRecipes(localPath)
		if err != nil {
			return req, fmt.Errorf("discover artifacts: %w", err)
		}
	}

	tempNames := make([]string, 0, len(artifactNames))
	buildPaths := make(map[string]string, len(artifactNames))
	for _, name := range artifactNames {
		tempName := pb.Strategy.TempRecipeName(name, sha, req.Prefix)
		tempNames = append(tempNames, tempName)
		buildPaths[tempName] = pb.Strategy.BuildPath(name)
	}
	req.Artifacts = tempNames

	// 9. Build PreparedBuildSource with one entry per artifact.
	req.Prepared = &dto.PreparedBuildSource{
		Backend:       dto.PreparedBuildBackendLaunchpad,
		TargetRef:     lpProject,
		RepositoryRef: repoSelfLink,
		Recipes:       make(map[string]dto.PreparedBuildRecipe, len(tempNames)),
	}
	for _, name := range tempNames {
		req.Prepared.Recipes[name] = dto.PreparedBuildRecipe{
			SourceRef: refLink,
			BuildPath: buildPaths[name],
		}
	}

	return req, nil
}

// prepareAndPush creates a temp branch, optionally runs a prepare command, and pushes to LP.
func (p *LocalBuildPreparer) prepareAndPush(
	ctx context.Context,
	localPath, gitSSHURL, lpOwner, branchName, sha, prepareCommand string,
) error {
	// Save current branch.
	origBranch, err := p.gitClient.CurrentBranch(localPath)
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}

	// Create and checkout temp branch.
	if err := p.gitClient.CreateBranch(localPath, branchName, sha); err != nil {
		return fmt.Errorf("create branch %s: %w", branchName, err)
	}
	if err := p.gitClient.CheckoutBranch(localPath, branchName); err != nil {
		return fmt.Errorf("checkout branch %s: %w", branchName, err)
	}

	// Restore original branch and delete temp branch on exit.
	defer func() {
		_ = p.gitClient.CheckoutBranch(localPath, origBranch)
		_ = p.gitClient.DeleteLocalBranch(localPath, branchName)
	}()

	// Optionally run prepare command.
	if prepareCommand != "" {
		if p.cmdRunner == nil {
			return fmt.Errorf("prepare command configured but no command runner available")
		}
		if err := p.cmdRunner.Run(ctx, localPath, prepareCommand); err != nil {
			return fmt.Errorf("run prepare command: %w", err)
		}
		if err := p.gitClient.AddAll(localPath); err != nil {
			return fmt.Errorf("stage prepared changes: %w", err)
		}
		if err := p.gitClient.Commit(localPath, "watchtower: prepare build"); err != nil {
			return fmt.Errorf("commit prepared changes: %w", err)
		}
	}

	// Push to Launchpad.
	if err := pushToLaunchpad(p.gitClient, localPath, gitSSHURL, lpOwner, branchName); err != nil {
		return fmt.Errorf("push to LP: %w", err)
	}

	return nil
}

// PrepareListByPrefix resolves Launchpad owner/project state for local-build discovery by prefix.
func (p *LocalBuildPreparer) PrepareListByPrefix(
	ctx context.Context,
	req PreparedBuildListRequest,
	prefix string,
) (PreparedBuildListRequest, error) {
	if p.repoManager == nil {
		return req, app.ErrLaunchpadAuthRequired
	}

	lpOwner := req.Owner
	var err error
	if lpOwner == "" {
		lpOwner, err = p.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return req, fmt.Errorf("get current LP user: %w", err)
		}
		req.Owner = lpOwner
	}

	lpProject, err := p.repoManager.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return req, fmt.Errorf("get LP project: %w", err)
	}
	req.TargetRef = lpProject
	req.RecipePrefix = prefix

	return req, nil
}

// PrepareDownloadByPrefix resolves Launchpad owner/project state for local-build downloads.
func (p *LocalBuildPreparer) PrepareDownloadByPrefix(
	ctx context.Context,
	req PreparedBuildDownloadRequest,
	prefix string,
) (PreparedBuildDownloadRequest, error) {
	if p.repoManager == nil {
		return req, app.ErrLaunchpadAuthRequired
	}

	lpOwner := req.Owner
	var err error
	if lpOwner == "" {
		lpOwner, err = p.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return req, fmt.Errorf("get current LP user: %w", err)
		}
	}
	req.Owner = lpOwner

	lpProject, err := p.repoManager.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return req, fmt.Errorf("get LP project: %w", err)
	}
	req.TargetRef = lpProject
	req.RecipePrefix = prefix

	return req, nil
}

func pushToLaunchpad(gitClient port.GitClient, localPath, gitSSHURL, lpOwner, branchName string) error {
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

	if err := gitClient.Push(localPath, remoteName, "refs/heads/"+branchName, "refs/heads/"+branchName, true); err != nil {
		return fmt.Errorf("push branch %s: %w", branchName, err)
	}

	return nil
}
