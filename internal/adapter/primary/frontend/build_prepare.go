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
	LPProject    string
	RecipePrefix string
}

// PreparedBuildDownloadRequest holds build download fields after frontend-side preparation.
type PreparedBuildDownloadRequest struct {
	Project      string
	Artifacts    []string
	ArtifactsDir string
	Owner        string
	LPProject    string
	RecipePrefix string
}

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

	lpOwner := req.Owner
	var err error
	if lpOwner == "" {
		lpOwner, err = p.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return req, fmt.Errorf("get current LP user: %w", err)
		}
	}
	req.Owner = lpOwner

	sha, err := p.gitClient.HeadSHA(localPath)
	if err != nil {
		return req, fmt.Errorf("resolve HEAD SHA: %w", err)
	}
	shortSHA := sha[:8]

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

	lpProject, err := p.repoManager.GetOrCreateProject(ctx, lpOwner)
	if err != nil {
		return req, fmt.Errorf("get/create LP project: %w", err)
	}

	repoSelfLink, gitSSHURL, err := p.repoManager.GetOrCreateRepo(ctx, lpOwner, lpProject, req.Project)
	if err != nil {
		return req, fmt.Errorf("get/create LP repo: %w", err)
	}

	if err := pushToLaunchpad(p.gitClient, localPath, gitSSHURL, lpOwner, shortSHA); err != nil {
		return req, fmt.Errorf("push to LP: %w", err)
	}

	tmpBranch := "refs/heads/tmp-" + shortSHA
	refLink, err := p.repoManager.WaitForGitRef(ctx, repoSelfLink, tmpBranch, 2*time.Minute)
	if err != nil {
		return req, fmt.Errorf("wait for git ref: %w", err)
	}

	gitRefLinks := make(map[string]string, len(tempNames))
	for _, name := range tempNames {
		gitRefLinks[name] = refLink
	}
	req.Prepared = &dto.PreparedBuildSource{
		LPProject:    lpProject,
		RepoSelfLink: repoSelfLink,
		GitRefLinks:  gitRefLinks,
		BuildPaths:   buildPaths,
	}

	return req, nil
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
	req.LPProject = lpProject
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
	req.LPProject = lpProject
	req.RecipePrefix = prefix

	return req, nil
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
