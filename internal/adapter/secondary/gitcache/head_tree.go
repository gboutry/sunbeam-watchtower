// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package gitcache

import (
	"fmt"
	"path"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// HeadFile stores one file found in the HEAD tree of a cached repository.
type HeadFile struct {
	Path    string
	Content []byte
}

// ReadHEADFile reads one file from the HEAD commit tree of a cached repository.
func ReadHEADFile(repoPath, filePath string) ([]byte, error) {
	tree, err := headTree(repoPath)
	if err != nil {
		return nil, err
	}
	file, err := tree.File(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s from HEAD tree: %w", filePath, err)
	}
	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("reading %s contents: %w", filePath, err)
	}
	return []byte(content), nil
}

// FindHEADFilesByBaseName lists files in the HEAD commit tree matching one basename.
func FindHEADFilesByBaseName(repoPath, baseName string) ([]HeadFile, error) {
	tree, err := headTree(repoPath)
	if err != nil {
		return nil, err
	}
	iter := tree.Files()
	defer iter.Close()

	var matches []HeadFile
	err = iter.ForEach(func(file *object.File) error {
		if path.Base(file.Name) != baseName {
			return nil
		}
		content, err := file.Contents()
		if err != nil {
			return fmt.Errorf("reading %s contents: %w", file.Name, err)
		}
		matches = append(matches, HeadFile{Path: file.Name, Content: []byte(content)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Path < matches[j].Path
	})
	return matches, nil
}

func headTree(repoPath string) (*object.Tree, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening repo at %s: %w", repoPath, err)
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD for %s: %w", repoPath, err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("loading HEAD commit for %s: %w", repoPath, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("loading HEAD tree for %s: %w", repoPath, err)
	}
	return tree, nil
}
