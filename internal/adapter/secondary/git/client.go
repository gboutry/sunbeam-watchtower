// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os/user"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// Client implements port.GitClient using go-git.
type Client struct {
	logger *slog.Logger
}

var _ port.GitClient = (*Client)(nil)

// NewClient creates a new git Client. If logger is nil, a no-op logger is used.
func NewClient(logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Client{logger: logger}
}

func (c *Client) IsRepo(path string) bool {
	c.logger.Debug("checking if path is repo", "path", path)
	_, err := gogit.PlainOpen(path)
	return err == nil
}

func (c *Client) HeadSHA(path string) (string, error) {
	c.logger.Debug("reading HEAD SHA", "path", path)
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("open repo %s: %w", path, err)
	}
	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("get HEAD for %s: %w", path, err)
	}
	sha := ref.Hash().String()
	c.logger.Debug("HEAD SHA resolved", "path", path, "sha", sha)
	return sha, nil
}

func (c *Client) HasUncommittedChanges(path string) (bool, error) {
	c.logger.Debug("checking for uncommitted changes", "path", path)
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return false, fmt.Errorf("open repo %s: %w", path, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("get worktree for %s: %w", path, err)
	}
	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("get status for %s: %w", path, err)
	}
	return !status.IsClean(), nil
}

func (c *Client) Push(path, remote, localRef, remoteRef string, force bool) error {
	c.logger.Debug("pushing", "path", path, "remote", remote, "localRef", localRef, "remoteRef", remoteRef, "force", force)
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", path, err)
	}
	r, err := repo.Remote(remote)
	if err != nil {
		return fmt.Errorf("get remote %s for %s: %w", remote, path, err)
	}

	remoteURL := r.Config().URLs[0]
	sshUser, err := sshUserFromURL(remoteURL)
	if err != nil {
		return fmt.Errorf("determine SSH user for remote URL %s: %w", remoteURL, err)
	}

	auth, err := gitssh.NewSSHAgentAuth(sshUser)
	if err != nil {
		return fmt.Errorf("creating SSH agent auth: %w", err)
	}
	refspec := config.RefSpec(fmt.Sprintf("%s:%s", localRef, remoteRef))
	opts := &gogit.PushOptions{
		RemoteName: r.Config().Name,
		RefSpecs:   []config.RefSpec{refspec},
		Auth:       auth,
		Force:      force,
	}
	if err := r.Push(opts); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("push %s to %s for %s: %w", localRef, remoteRef, path, err)
	}
	return nil
}

// sshUserFromURL extracts the SSH user from a remote URL.
// Returns an error for HTTPS remotes (unsupported).
// Falls back to the effective unix user if no user is present.
func sshUserFromURL(remoteURL string) (string, error) {
	if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		return "", fmt.Errorf("HTTPS remotes are not supported for push; use an SSH remote")
	}

	// SCP-style: user@host:path
	if !strings.Contains(remoteURL, "://") {
		if at := strings.Index(remoteURL, "@"); at > 0 {
			return remoteURL[:at], nil
		}
		return effectiveUser()
	}

	// URL-style: ssh://user@host/path
	u, err := url.Parse(remoteURL)
	if err == nil && u.User != nil && u.User.Username() != "" {
		return u.User.Username(), nil
	}
	return effectiveUser()
}

func effectiveUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("determining current user: %w", err)
	}
	return u.Username, nil
}

func (c *Client) AddRemote(path, name, url string) error {
	c.logger.Debug("adding remote", "path", path, "name", name, "url", url)
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", path, err)
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
		Fetch: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", name)),
		},
	})
	if err != nil {
		return fmt.Errorf("add remote %s to %s: %w", name, path, err)
	}
	return nil
}

func (c *Client) RemoveRemote(path, name string) error {
	c.logger.Debug("removing remote", "path", path, "name", name)
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", path, err)
	}
	if err := repo.DeleteRemote(name); err != nil {
		return fmt.Errorf("remove remote %s from %s: %w", name, path, err)
	}
	return nil
}
