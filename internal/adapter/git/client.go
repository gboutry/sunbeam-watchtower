// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"fmt"

	"github.com/gboutry/sunbeam-watchtower/internal/port"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

// Client implements port.GitClient using go-git.
type Client struct{}

var _ port.GitClient = (*Client)(nil)

func NewClient() *Client { return &Client{} }

func (c *Client) IsRepo(path string) bool {
	_, err := gogit.PlainOpen(path)
	return err == nil
}

func (c *Client) HeadSHA(path string) (string, error) {
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("open repo %s: %w", path, err)
	}
	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("get HEAD for %s: %w", path, err)
	}
	return ref.Hash().String(), nil
}

func (c *Client) HasUncommittedChanges(path string) (bool, error) {
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
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", path, err)
	}
	r, err := repo.Remote(remote)
	if err != nil {
		return fmt.Errorf("get remote %s for %s: %w", remote, path, err)
	}
	refspec := config.RefSpec(fmt.Sprintf("%s:%s", localRef, remoteRef))
	opts := &gogit.PushOptions{
		RemoteName: r.Config().Name,
		RefSpecs:   []config.RefSpec{refspec},
		Force:      force,
	}
	if err := r.Push(opts); err != nil {
		return fmt.Errorf("push %s to %s for %s: %w", localRef, remoteRef, path, err)
	}
	return nil
}

func (c *Client) AddRemote(path, name, url string) error {
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
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo %s: %w", path, err)
	}
	if err := repo.DeleteRemote(name); err != nil {
		return fmt.Errorf("remove remote %s from %s: %w", name, path, err)
	}
	return nil
}
