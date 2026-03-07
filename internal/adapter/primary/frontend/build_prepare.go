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
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
)

// LocalBuildPreparer handles frontend-side local preparation for split build workflows.
type LocalBuildPreparer struct {
	gitClient   port.GitClient
	repoManager port.RepoManager
	builders    map[string]build.ProjectBuilder
}

// NewLocalBuildPreparer creates a reusable local build preparer.
func NewLocalBuildPreparer(
	gitClient port.GitClient,
	repoManager port.RepoManager,
	builders map[string]build.ProjectBuilder,
) *LocalBuildPreparer {
	return &LocalBuildPreparer{
		gitClient:   gitClient,
		repoManager: repoManager,
		builders:    builders,
	}
}

// PrepareTrigger resolves local git and Launchpad state and returns a prepared build-trigger request.
func (p *LocalBuildPreparer) PrepareTrigger(
	ctx context.Context,
	opts client.BuildsTriggerOptions,
	localPath string,
) (client.BuildsTriggerOptions, error) {
	if p.repoManager == nil {
		return opts, app.ErrLaunchpadAuthRequired
	}
	pb, ok := p.builders[opts.Project]
	if !ok {
		return opts, fmt.Errorf("unknown project %q", opts.Project)
	}

	lpOwner := opts.Owner
	var err error
	if lpOwner == "" {
		lpOwner, err = p.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return opts, fmt.Errorf("get current LP user: %w", err)
		}
	}
	opts.Owner = lpOwner

	sha, err := p.gitClient.HeadSHA(localPath)
	if err != nil {
		return opts, fmt.Errorf("resolve HEAD SHA: %w", err)
	}
	shortSHA := sha[:8]

	artifactNames := opts.Artifacts
	if len(artifactNames) == 0 {
		artifactNames, err = pb.Strategy.DiscoverRecipes(localPath)
		if err != nil {
			return opts, fmt.Errorf("discover artifacts: %w", err)
		}
	}

	tempNames := make([]string, 0, len(artifactNames))
	buildPaths := make(map[string]string, len(artifactNames))
	for _, name := range artifactNames {
		tempName := pb.Strategy.TempRecipeName(name, sha, opts.Prefix)
		tempNames = append(tempNames, tempName)
		buildPaths[tempName] = pb.Strategy.BuildPath(name)
	}
	opts.Artifacts = tempNames
	opts.BuildPaths = buildPaths

	lpProject, err := p.repoManager.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return opts, fmt.Errorf("get/create LP project: %w", err)
	}
	opts.LPProject = lpProject

	repoSelfLink, gitSSHURL, err := p.repoManager.GetOrCreateRepo(ctx, lpOwner, lpProject, opts.Project)
	if err != nil {
		return opts, fmt.Errorf("get/create LP repo: %w", err)
	}
	opts.RepoSelfLink = repoSelfLink

	if err := pushToLaunchpad(p.gitClient, localPath, gitSSHURL, lpOwner, shortSHA); err != nil {
		return opts, fmt.Errorf("push to LP: %w", err)
	}

	tmpBranch := "refs/heads/tmp-" + shortSHA
	refLink, err := p.repoManager.WaitForGitRef(ctx, repoSelfLink, tmpBranch, 2*time.Minute)
	if err != nil {
		return opts, fmt.Errorf("wait for git ref: %w", err)
	}

	gitRefLinks := make(map[string]string, len(tempNames))
	for _, name := range tempNames {
		gitRefLinks[name] = refLink
	}
	opts.GitRefLinks = gitRefLinks

	return opts, nil
}

// PrepareListByPrefix resolves Launchpad owner/project state for local-build discovery by prefix.
func (p *LocalBuildPreparer) PrepareListByPrefix(
	ctx context.Context,
	opts client.BuildsListOptions,
	prefix string,
) (client.BuildsListOptions, error) {
	if p.repoManager == nil {
		return opts, app.ErrLaunchpadAuthRequired
	}

	lpOwner := opts.Owner
	var err error
	if lpOwner == "" {
		lpOwner, err = p.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return opts, fmt.Errorf("get current LP user: %w", err)
		}
		opts.Owner = lpOwner
	}

	lpProject, err := p.repoManager.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return opts, fmt.Errorf("get LP project: %w", err)
	}
	opts.LPProject = lpProject
	opts.RecipePrefix = prefix

	return opts, nil
}

// PrepareDownloadByPrefix resolves Launchpad owner/project state for local-build downloads.
func (p *LocalBuildPreparer) PrepareDownloadByPrefix(
	ctx context.Context,
	opts client.BuildsDownloadOptions,
	prefix string,
) (client.BuildsDownloadOptions, error) {
	if p.repoManager == nil {
		return opts, app.ErrLaunchpadAuthRequired
	}

	lpOwner := opts.Owner
	var err error
	if lpOwner == "" {
		lpOwner, err = p.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return opts, fmt.Errorf("get current LP user: %w", err)
		}
	}
	opts.Owner = lpOwner

	lpProject, err := p.repoManager.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return opts, fmt.Errorf("get LP project: %w", err)
	}
	opts.LPProject = lpProject
	opts.RecipePrefix = prefix

	return opts, nil
}

func pushToLaunchpad(gitClient port.GitClient, localPath, gitSSHURL, lpOwner, shortSHA string) error {
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

	tmpBranch := "refs/heads/tmp-" + shortSHA
	if err := gitClient.Push(localPath, remoteName, "HEAD", "refs/heads/main", true); err != nil {
		return fmt.Errorf("push main: %w", err)
	}
	if err := gitClient.Push(localPath, remoteName, "HEAD", tmpBranch, true); err != nil {
		return fmt.Errorf("push tmp branch: %w", err)
	}

	return nil
}
