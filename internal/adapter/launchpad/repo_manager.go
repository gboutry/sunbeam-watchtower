// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package launchpad

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"time"

	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
)

// RepoManager implements port.RepoManager using the Launchpad API.
// It manages temporary projects and git repositories for local builds.
type RepoManager struct {
	client *lp.Client
	logger *slog.Logger
}

var _ port.RepoManager = (*RepoManager)(nil)

func NewRepoManager(client *lp.Client, logger *slog.Logger) *RepoManager {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &RepoManager{client: client, logger: logger}
}

func (m *RepoManager) GetOrCreateProject(ctx context.Context, owner string) (string, error) {
	projectName := owner + "-sunbeam-remote-build"

	_, err := m.client.GetProject(ctx, projectName)
	if err == nil {
		m.logger.Debug("using existing LP project", "project", projectName)
		return projectName, nil
	}

	m.logger.Info("creating LP project", "project", projectName)
	_, err = m.client.CreateProject(ctx, projectName,
		projectName,
		"Temporary project for remote builds",
		"Auto-created by sunbeam-watchtower for remote build recipes",
	)
	if err != nil {
		return "", fmt.Errorf("creating LP project %q: %w", projectName, err)
	}
	return projectName, nil
}

func (m *RepoManager) GetOrCreateRepo(ctx context.Context, owner, project, repoName string) (string, string, error) {
	repo, err := m.client.GetGitRepository(ctx, owner, project, repoName)
	if err == nil {
		m.logger.Debug("using existing LP git repo", "repo", repoName)
		return repo.SelfLink, injectSSHUser(repo.GitSSHURL, owner), nil
	}

	m.logger.Info("creating LP git repo", "owner", owner, "project", project, "name", repoName)
	repo, err = m.client.CreateGitRepository(ctx, owner, project, repoName)
	if err != nil {
		return "", "", fmt.Errorf("creating git repo ~%s/%s/+git/%s: %w", owner, project, repoName, err)
	}
	return repo.SelfLink, injectSSHUser(repo.GitSSHURL, owner), nil
}

// injectSSHUser ensures the SSH URL has the LP username set.
// LP's git_ssh_url omits the user, but LP requires <lp_username>@ for push auth.
func injectSSHUser(sshURL, lpUser string) string {
	// Normalise git+ssh:// → ssh:// (go-git doesn't support git+ssh).
	sshURL = strings.Replace(sshURL, "git+ssh://", "ssh://", 1)

	u, err := url.Parse(sshURL)
	if err != nil {
		return sshURL
	}
	if u.User == nil || u.User.Username() == "" {
		u.User = url.User(lpUser)
	}
	return u.String()
}

func (m *RepoManager) GetGitRef(ctx context.Context, repoSelfLink, refPath string) (string, error) {
	ref, err := m.client.GetGitRef(ctx, repoSelfLink, refPath)
	if err != nil {
		return "", fmt.Errorf("getting git ref %q: %w", refPath, err)
	}
	return ref.SelfLink, nil
}

func (m *RepoManager) WaitForGitRef(ctx context.Context, repoSelfLink, refPath string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	wait := 1 * time.Second
	maxWait := 30 * time.Second

	for {
		refLink, err := m.GetGitRef(ctx, repoSelfLink, refPath)
		if err == nil {
			return refLink, nil
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for git ref %q after %v", refPath, timeout)
		}

		m.logger.Debug("waiting for git ref to appear", "ref", refPath, "retry_in", wait)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(wait):
		}

		wait *= 2
		if wait > maxWait {
			wait = maxWait
		}
	}
}
