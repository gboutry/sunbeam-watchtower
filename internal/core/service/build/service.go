// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// RecipeAction is the action determined for a recipe after assessment.
type RecipeAction = dto.BuildRecipeAction

const (
	ActionCreateRecipe  RecipeAction = dto.BuildActionCreateRecipe
	ActionRequestBuilds RecipeAction = dto.BuildActionRequestBuilds
	ActionRetryFailed   RecipeAction = dto.BuildActionRetryFailed
	ActionMonitor       RecipeAction = dto.BuildActionMonitor
	ActionDownload      RecipeAction = dto.BuildActionDownload
	ActionNoop          RecipeAction = dto.BuildActionNoop
)

// RecipeStatus holds the assessed state of a single recipe.
type RecipeStatus struct {
	Name   string
	Action RecipeAction
	Recipe *dto.Recipe
	Builds []dto.Build
	Error  error
}

// TriggerOpts holds options for triggering builds.
type TriggerOpts struct {
	Source        string // "remote" or "local"
	Wait          bool
	Timeout       time.Duration
	Owner         string // override project owner
	Prefix        string // temp recipe name prefix (local mode)
	LocalPath     string // path to local git repo (local mode)
	Channels      map[string]string
	Architectures []string
	// Snap-specific
	ArchiveLink string
	Pocket      string
}

// TriggerResult holds the result of a trigger operation.
type TriggerResult = dto.BuildTriggerResult

// RecipeResult holds the result of a single recipe action.
type RecipeResult = dto.BuildRecipeResult

// ListOpts holds options for listing builds.
type ListOpts struct {
	Projects  []string
	All       bool   // show all builds, not just active
	State     string // filter by state
	Source    string // "local" or "remote"
	LocalPath string // path to local git repo (local mode)
	Prefix    string // temp recipe name prefix (local mode)
}

// ProjectResult holds builds from one project, or an error.
type ProjectResult struct {
	ProjectName string
	Builds      []dto.Build
	Err         error
}

// CleanupOpts holds options for cleaning up temporary recipes.
type CleanupOpts struct {
	Projects []string
	Owner    string
	Prefix   string
	DryRun   bool
}

// Service orchestrates builds across projects.
type Service struct {
	projects    map[string]ProjectBuilder // keyed by watchtower project name
	repoManager port.RepoManager
	gitClient   port.GitClient
	logger      *slog.Logger
}

// NewService creates a build service with the given project-to-builder mappings.
func NewService(projects map[string]ProjectBuilder, repoManager port.RepoManager, gitClient port.GitClient, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		projects:    projects,
		repoManager: repoManager,
		gitClient:   gitClient,
		logger:      logger,
	}
}

// Trigger orchestrates the build pipeline for a project. It is re-entrant:
// calling it multiple times for the same recipes picks up where it left off.
func (s *Service) Trigger(ctx context.Context, projectName string, recipeNames []string, opts TriggerOpts) (*TriggerResult, error) {
	pb, ok := s.projects[projectName]
	if !ok {
		return nil, fmt.Errorf("unknown project %q", projectName)
	}

	owner := opts.Owner
	if owner == "" && opts.Source == "local" {
		// Local mode: resolve from LP Me() — config owner is the remote/official owner.
		if s.repoManager == nil {
			return nil, fmt.Errorf("local mode requires a RepoManager")
		}
		me, err := s.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolving current LP user: %w", err)
		}
		owner = me
	}
	if owner == "" {
		owner = pb.Owner
	}
	if owner == "" {
		return nil, fmt.Errorf("no owner configured for project %q (set build.owner in config or use --owner)", projectName)
	}
	pb.Owner = owner

	recipes := recipeNames
	if len(recipes) == 0 {
		recipes = pb.Recipes
	}

	// Local mode: auto-discover recipes from the local repo if none specified.
	if len(recipes) == 0 && opts.Source == "local" && opts.LocalPath != "" {
		discovered, err := pb.Strategy.DiscoverRecipes(opts.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("discovering recipes in %s: %w", opts.LocalPath, err)
		}
		recipes = discovered
	}

	if len(recipes) == 0 {
		return nil, fmt.Errorf("no recipes specified for project %q", projectName)
	}

	// Local mode: push local git to temp LP repo before recipe operations.
	var repoSelfLink string
	// Per-recipe git ref links: key is recipe name, value is git ref self_link
	gitRefLinks := make(map[string]string)
	// Per-recipe build paths: key is recipe name, value is build path
	buildPaths := make(map[string]string)

	if opts.Source == "local" {
		if opts.LocalPath == "" {
			return nil, fmt.Errorf("local mode requires LocalPath")
		}
		if s.repoManager == nil {
			return nil, fmt.Errorf("local mode requires a RepoManager")
		}
		if s.gitClient == nil {
			return nil, fmt.Errorf("local mode requires a GitClient")
		}

		projName, err := s.repoManager.GetOrCreateProject(ctx, owner)
		if err != nil {
			return nil, fmt.Errorf("get/create project: %w", err)
		}

		repoLink, sshURL, err := s.repoManager.GetOrCreateRepo(ctx, owner, projName, projectName)
		if err != nil {
			return nil, fmt.Errorf("get/create repo: %w", err)
		}
		repoSelfLink = repoLink

		sha, err := s.gitClient.HeadSHA(opts.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("get HEAD SHA: %w", err)
		}

		remoteName := "watchtower-tmp"
		_ = s.gitClient.RemoveRemote(opts.LocalPath, remoteName)
		if err := s.gitClient.AddRemote(opts.LocalPath, remoteName, sshURL); err != nil {
			return nil, fmt.Errorf("add remote: %w", err)
		}
		defer func() {
			_ = s.gitClient.RemoveRemote(opts.LocalPath, remoteName)
		}()

		refBranch := "refs/heads/" + sha[:8]
		if err := s.gitClient.Push(opts.LocalPath, remoteName, "HEAD", refBranch, true); err != nil {
			return nil, fmt.Errorf("push to LP: %w", err)
		}

		timeout := 2 * time.Minute
		refLink, err := s.repoManager.WaitForGitRef(ctx, repoSelfLink, refBranch, timeout)
		if err != nil {
			return nil, fmt.Errorf("wait for git ref: %w", err)
		}

		// All local recipes use the same ref
		for _, name := range recipes {
			tempName := pb.Strategy.TempRecipeName(name, sha, opts.Prefix)
			gitRefLinks[tempName] = refLink
			buildPaths[tempName] = pb.Strategy.BuildPath(name)
		}

		// Rewrite recipe names to temp names for local mode.
		tempRecipes := make([]string, len(recipes))
		for i, name := range recipes {
			tempRecipes[i] = pb.Strategy.TempRecipeName(name, sha, opts.Prefix)
		}
		recipes = tempRecipes

	} else if opts.Source == "remote" && pb.OfficialCodehosting {
		// Remote mode with official LP git repo: resolve repo and branches.
		if s.repoManager == nil {
			return nil, fmt.Errorf("remote mode requires a RepoManager")
		}

		lpProject := pb.RecipeProject()
		repoLink, defaultBranch, err := s.repoManager.GetDefaultRepo(ctx, lpProject)
		if err != nil {
			return nil, fmt.Errorf("get default repo for %q: %w", lpProject, err)
		}
		repoSelfLink = repoLink

		// If series are configured, expand artifacts into series-based recipes
		if len(pb.Series) > 0 && len(pb.DevFocus) > 0 {
			var expandedRecipes []string
			for _, artifactName := range recipes {
				for _, series := range pb.Series {
					recipeName := pb.Strategy.OfficialRecipeName(artifactName, series, pb.DevFocus)
					branch := pb.Strategy.BranchForSeries(series, pb.DevFocus, defaultBranch)
					refPath := "refs/heads/" + branch

					refLink, err := s.repoManager.GetGitRef(ctx, repoSelfLink, refPath)
					if err != nil {
						s.logger.Warn("branch not found, skipping", "branch", branch, "series", series, "error", err)
						continue
					}
					gitRefLinks[recipeName] = refLink
					buildPaths[recipeName] = pb.Strategy.BuildPath(artifactName)
					expandedRecipes = append(expandedRecipes, recipeName)
				}
			}
			recipes = expandedRecipes
		} else {
			// No series: use default branch for all recipes
			refPath := "refs/heads/" + defaultBranch
			refLink, err := s.repoManager.GetGitRef(ctx, repoSelfLink, refPath)
			if err != nil {
				return nil, fmt.Errorf("get git ref %q: %w", refPath, err)
			}
			for _, name := range recipes {
				gitRefLinks[name] = refLink
			}
		}
	}

	result := &TriggerResult{Project: projectName}
	var recipePtrs []*dto.Recipe

	for _, name := range recipes {
		status := s.assessRecipe(ctx, pb, name)
		refLink := gitRefLinks[name]
		bp := buildPaths[name]
		rr := s.executeAction(ctx, pb, status, opts, repoSelfLink, refLink, bp)
		result.RecipeResults = append(result.RecipeResults, rr)

		// Collect recipe pointers for wait loop.
		if rr.Error == nil && rr.Recipe != nil {
			recipePtrs = append(recipePtrs, rr.Recipe)
		}
	}

	if opts.Wait && len(recipePtrs) > 0 {
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = 30 * time.Minute
		}
		builds, err := s.waitForBuilds(ctx, pb, recipePtrs, timeout)
		if err != nil {
			s.logger.Warn("wait for builds completed with error", "error", err)
		}
		// Update results with final build states.
		for i := range result.RecipeResults {
			rr := &result.RecipeResults[i]
			for _, b := range builds {
				if b.Recipe == rr.Name {
					rr.Builds = append(rr.Builds, b)
				}
			}
		}
	}

	return result, nil
}

func (s *Service) assessRecipe(ctx context.Context, pb ProjectBuilder, recipeName string) RecipeStatus {
	recipe, err := pb.Builder.GetRecipe(ctx, pb.Owner, pb.RecipeProject(), recipeName)
	if err != nil {
		return RecipeStatus{Name: recipeName, Action: ActionCreateRecipe}
	}

	builds, err := pb.Builder.ListBuilds(ctx, recipe)
	if err != nil {
		return RecipeStatus{Name: recipeName, Action: ActionRequestBuilds, Recipe: recipe}
	}

	if len(builds) == 0 {
		return RecipeStatus{Name: recipeName, Action: ActionRequestBuilds, Recipe: recipe}
	}

	allSucceeded := true
	hasActive := false
	hasFailed := false
	for _, b := range builds {
		if !b.State.IsTerminal() {
			hasActive = true
			allSucceeded = false
		} else if b.State.IsFailure() {
			hasFailed = true
			allSucceeded = false
		} else if b.State != dto.BuildSucceeded {
			allSucceeded = false
		}
	}

	if allSucceeded {
		return RecipeStatus{Name: recipeName, Action: ActionDownload, Recipe: recipe, Builds: builds}
	}
	if hasActive {
		return RecipeStatus{Name: recipeName, Action: ActionMonitor, Recipe: recipe, Builds: builds}
	}
	if hasFailed {
		return RecipeStatus{Name: recipeName, Action: ActionRetryFailed, Recipe: recipe, Builds: builds}
	}
	return RecipeStatus{Name: recipeName, Action: ActionRequestBuilds, Recipe: recipe, Builds: builds}
}

func (s *Service) executeAction(ctx context.Context, pb ProjectBuilder, status RecipeStatus, opts TriggerOpts, repoSelfLink, gitRefLink, buildPath string) RecipeResult {
	result := RecipeResult{Name: status.Name, Action: status.Action, Recipe: status.Recipe}

	switch status.Action {
	case ActionCreateRecipe:
		if repoSelfLink == "" || gitRefLink == "" {
			result.Error = fmt.Errorf("recipe %q not found (create requires git repo info; use local mode or enable official_codehosting)", status.Name)
			return result
		}
		bp := buildPath
		if bp == "" {
			bp = pb.Strategy.BuildPath(status.Name)
		}
		recipe, err := pb.Builder.CreateRecipe(ctx, dto.CreateRecipeOpts{
			Name:        status.Name,
			Owner:       pb.Owner,
			Project:     pb.RecipeProject(),
			GitRepoLink: repoSelfLink,
			GitRefLink:  gitRefLink,
			BuildPath:   bp,
		})
		if err != nil {
			result.Error = fmt.Errorf("create recipe %q: %w", status.Name, err)
			return result
		}
		result.Recipe = recipe
		br, err := pb.Builder.RequestBuilds(ctx, recipe, buildOpts(opts))
		result.BuildRequest = br
		if err != nil {
			result.Error = fmt.Errorf("request builds for %q: %w", status.Name, err)
		}

	case ActionRequestBuilds:
		br, err := pb.Builder.RequestBuilds(ctx, status.Recipe, buildOpts(opts))
		result.BuildRequest = br
		if err != nil {
			result.Error = fmt.Errorf("request builds for %q: %w", status.Name, err)
		}

	case ActionRetryFailed:
		for _, b := range status.Builds {
			if b.State.IsFailure() && b.CanRetry {
				if err := pb.Builder.RetryBuild(ctx, b.SelfLink); err != nil {
					s.logger.Warn("failed to retry build", "build", b.SelfLink, "error", err)
				}
			}
		}
		result.Action = ActionMonitor
		result.Builds = status.Builds

	case ActionMonitor:
		result.Builds = status.Builds

	case ActionDownload:
		result.Builds = status.Builds

	case ActionNoop:
		// nothing to do
	}

	return result
}

func buildOpts(opts TriggerOpts) dto.RequestBuildsOpts {
	return dto.RequestBuildsOpts{
		Channels:      opts.Channels,
		Architectures: opts.Architectures,
		ArchiveLink:   opts.ArchiveLink,
		Pocket:        opts.Pocket,
	}
}

func (s *Service) waitForBuilds(ctx context.Context, pb ProjectBuilder, recipes []*dto.Recipe, timeout time.Duration) ([]dto.Build, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 60 * time.Second

	for {
		var allBuilds []dto.Build
		allTerminal := true

		for _, recipe := range recipes {
			builds, err := pb.Builder.ListBuilds(ctx, recipe)
			if err != nil {
				s.logger.Warn("error listing builds", "recipe", recipe.Name, "error", err)
				continue
			}
			allBuilds = append(allBuilds, builds...)
			for _, b := range builds {
				if !b.State.IsTerminal() {
					allTerminal = false
				}
			}
		}

		if allTerminal {
			return allBuilds, nil
		}

		if time.Now().After(deadline) {
			return allBuilds, fmt.Errorf("timeout waiting for builds after %v", timeout)
		}

		select {
		case <-ctx.Done():
			return allBuilds, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// List returns builds across configured projects, applying filters.
// Per-project errors are collected but do not stop aggregation (graceful degradation).
func (s *Service) List(ctx context.Context, opts ListOpts) ([]dto.Build, []ProjectResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

	// For local mode, resolve owner and SHA once.
	var localOwner, localSHA string
	if opts.Source == "local" {
		if s.repoManager == nil {
			return nil, nil, fmt.Errorf("local mode requires a RepoManager")
		}
		if s.gitClient == nil {
			return nil, nil, fmt.Errorf("local mode requires a GitClient")
		}
		if opts.LocalPath == "" {
			return nil, nil, fmt.Errorf("local mode requires LocalPath")
		}
		me, err := s.repoManager.GetCurrentUser(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving current LP user: %w", err)
		}
		localOwner = me

		sha, err := s.gitClient.HeadSHA(opts.LocalPath)
		if err != nil {
			return nil, nil, fmt.Errorf("get HEAD SHA: %w", err)
		}
		localSHA = sha
	}

	var results []ProjectResult
	var all []dto.Build

	for name, pb := range s.projects {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}

		result := ProjectResult{ProjectName: name}
		var projBuilds []dto.Build

		owner := pb.Owner
		recipeNames := pb.Recipes

		if opts.Source == "local" {
			owner = localOwner
			prefix := opts.Prefix
			if prefix == "" {
				prefix = "tmp-build"
			}
			tempNames := make([]string, len(recipeNames))
			for i, rn := range recipeNames {
				tempNames[i] = pb.Strategy.TempRecipeName(rn, localSHA, prefix)
			}
			recipeNames = tempNames
		}

		for _, recipeName := range recipeNames {
			recipe, err := pb.Builder.GetRecipe(ctx, owner, pb.RecipeProject(), recipeName)
			if err != nil {
				s.logger.Warn("error getting recipe", "project", name, "recipe", recipeName, "error", err)
				continue
			}

			builds, err := pb.Builder.ListBuilds(ctx, recipe)
			if err != nil {
				s.logger.Warn("error listing builds", "project", name, "recipe", recipeName, "error", err)
				continue
			}

			for i := range builds {
				builds[i].Project = name
			}
			projBuilds = append(projBuilds, builds...)
		}

		// Apply state filter.
		if opts.State != "" {
			filtered := projBuilds[:0]
			for _, b := range projBuilds {
				if strings.EqualFold(b.State.String(), opts.State) {
					filtered = append(filtered, b)
				}
			}
			projBuilds = filtered
		}

		// If not showing all, only return active builds.
		if !opts.All {
			filtered := projBuilds[:0]
			for _, b := range projBuilds {
				if b.State.IsActive() {
					filtered = append(filtered, b)
				}
			}
			projBuilds = filtered
		}

		result.Builds = projBuilds
		results = append(results, result)
		all = append(all, projBuilds...)
	}

	// Sort by CreatedAt descending.
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	return all, results, nil
}

// Download retrieves build artifacts for succeeded builds of the given recipes.
func (s *Service) Download(ctx context.Context, projectName string, recipeNames []string, outputDir string) error {
	pb, ok := s.projects[projectName]
	if !ok {
		return fmt.Errorf("unknown project %q", projectName)
	}

	recipes := recipeNames
	if len(recipes) == 0 {
		recipes = pb.Recipes
	}

	for _, name := range recipes {
		recipe, err := pb.Builder.GetRecipe(ctx, pb.Owner, pb.RecipeProject(), name)
		if err != nil {
			return fmt.Errorf("recipe %q: %w", name, err)
		}
		builds, err := pb.Builder.ListBuilds(ctx, recipe)
		if err != nil {
			return fmt.Errorf("listing builds for %q: %w", name, err)
		}

		for _, b := range builds {
			if b.State != dto.BuildSucceeded {
				continue
			}
			urls, err := pb.Builder.GetBuildFileURLs(ctx, b.SelfLink)
			if err != nil {
				s.logger.Warn("failed to get file URLs", "build", b.SelfLink, "error", err)
				continue
			}
			for _, u := range urls {
				if err := downloadFile(u, outputDir, name); err != nil {
					return fmt.Errorf("downloading %s: %w", u, err)
				}
			}
		}
	}
	return nil
}

// downloadFile downloads a file from fileURL into outputDir/artifactName/,
// with path traversal protection.
func downloadFile(fileURL, outputDir, artifactName string) error {
	// Extract filename from URL (last path segment).
	filename := path.Base(fileURL)
	if filename == "" || filename == "." || filename == "/" {
		return fmt.Errorf("cannot determine filename from URL %q", fileURL)
	}
	if strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename %q: contains path traversal", filename)
	}

	destDir := filepath.Join(outputDir, artifactName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	destPath := filepath.Join(destDir, filename)
	// Verify resolved path is within outputDir.
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output dir: %w", err)
	}
	absDest, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("resolving dest path: %w", err)
	}
	if !strings.HasPrefix(absDest, absOutput+string(filepath.Separator)) {
		return fmt.Errorf("path traversal detected: %q is outside %q", absDest, absOutput)
	}

	resp, err := http.Get(fileURL) //nolint:gosec // URL comes from LP API
	if err != nil {
		return fmt.Errorf("HTTP GET %q: %w", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP GET %q: status %d", fileURL, resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file %q: %w", destPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing file %q: %w", destPath, err)
	}

	return nil
}

// Cleanup removes temporary recipes matching the given prefix.
func (s *Service) Cleanup(ctx context.Context, opts CleanupOpts) ([]string, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

	owner := opts.Owner

	var deleted []string
	for name, pb := range s.projects {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}

		projOwner := pb.Owner
		if owner != "" {
			projOwner = owner
		}

		for _, recipeName := range pb.Recipes {
			if opts.Prefix != "" && !strings.HasPrefix(recipeName, opts.Prefix) {
				continue
			}

			recipe, err := pb.Builder.GetRecipe(ctx, projOwner, pb.RecipeProject(), recipeName)
			if err != nil {
				s.logger.Warn("recipe not found for cleanup", "recipe", recipeName, "error", err)
				continue
			}

			if opts.DryRun {
				s.logger.Info("would delete recipe", "recipe", recipeName)
				deleted = append(deleted, recipeName)
				continue
			}

			if err := pb.Builder.DeleteRecipe(ctx, recipe.SelfLink); err != nil {
				s.logger.Warn("failed to delete recipe", "recipe", recipeName, "error", err)
				continue
			}
			deleted = append(deleted, recipeName)
		}
	}

	return deleted, nil
}
