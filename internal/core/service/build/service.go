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
	"sync"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// defaultConcurrency is the max number of parallel LP API operations.
const defaultConcurrency = 4

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
	Wait    bool
	Timeout time.Duration
	Owner   string // override project owner
	Prefix  string // temp recipe name prefix

	TargetRef string // override backend target reference for recipe operations
	Prepared  *dto.PreparedBuildSource

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
	Projects     []string
	All          bool     // show all builds, not just active
	State        string   // filter by state
	Owner        string   // override project owner
	TargetRef    string   // override backend target reference for recipe lookup
	RecipeNames  []string // explicit recipe names (overrides project config)
	RecipePrefix string   // filter recipes by name prefix (used with ListRecipesByOwner)
}

// ProjectResult holds builds from one project, or an error.
type ProjectResult struct {
	ProjectName string
	Builds      []dto.Build
	Err         error
}

// CleanupOpts holds options for cleaning up temporary recipes.
type CleanupOpts struct {
	Projects  []string
	Owner     string
	Prefix    string
	DryRun    bool
	TargetRef string // LP project for branch cleanup resolution
}

// CleanupResult holds the result of a cleanup operation.
type CleanupResult struct {
	DeletedRecipes  []string
	DeletedBranches []string
}

// Service orchestrates builds across projects.
type Service struct {
	projects    map[string]ProjectBuilder // keyed by watchtower project name
	repoManager port.RepoManager
	logger      *slog.Logger
}

// NewService creates a build service with the given project-to-builder mappings.
func NewService(projects map[string]ProjectBuilder, repoManager port.RepoManager, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		projects:    projects,
		repoManager: repoManager,
		logger:      logger,
	}
}

// Trigger orchestrates the build pipeline for a project. It is re-entrant:
// calling it multiple times for the same recipes picks up where it left off.
//
// When opts.Prepared is provided (e.g. by CLI/TUI local preparation), the
// service uses those pre-resolved Launchpad references directly. Otherwise, it
// resolves repo and ref information from Launchpad (remote/official mode).
func (s *Service) Trigger(ctx context.Context, projectName string, artifactNames []string, opts TriggerOpts) (*TriggerResult, error) {
	pb, ok := s.projects[projectName]
	if !ok {
		return nil, fmt.Errorf("unknown project %q", projectName)
	}

	owner := opts.Owner
	if owner == "" {
		owner = pb.Owner
	}
	if owner == "" {
		return nil, fmt.Errorf("no owner configured for project %q (set build.owner in config or use --owner)", projectName)
	}
	pb.Owner = owner

	prepared := opts.Prepared.Normalize()

	targetRef := opts.TargetRef
	if prepared != nil && prepared.TargetRef != "" {
		targetRef = prepared.TargetRef
	}
	if targetRef != "" {
		pb.LPProject = targetRef
	}

	recipes := artifactNames
	if len(recipes) == 0 {
		recipes = pb.Artifacts
	}
	if len(recipes) == 0 {
		return nil, fmt.Errorf("no artifacts specified for project %q", projectName)
	}

	// Resolve LP repo and ref information.
	repoSelfLink := ""
	var gitRefLinks map[string]string
	var buildPaths map[string]string
	if prepared != nil {
		repoSelfLink = prepared.RepositoryRef
		if len(prepared.Recipes) > 0 {
			gitRefLinks = make(map[string]string, len(prepared.Recipes))
			buildPaths = make(map[string]string, len(prepared.Recipes))
			for recipeName, recipe := range prepared.Recipes {
				gitRefLinks[recipeName] = recipe.SourceRef
				buildPaths[recipeName] = recipe.BuildPath
			}
		}
	}
	if gitRefLinks == nil {
		gitRefLinks = make(map[string]string)
	}
	if buildPaths == nil {
		buildPaths = make(map[string]string)
	}

	// If caller didn't provide pre-resolved values and project uses official
	// codehosting, resolve repo and refs from Launchpad.
	if repoSelfLink == "" && pb.OfficialCodehosting {
		if s.repoManager == nil {
			return nil, fmt.Errorf("official codehosting requires a RepoManager")
		}

		lpProject := pb.RecipeProject()
		repoLink, defaultBranch, err := s.repoManager.GetDefaultRepo(ctx, lpProject)
		if err != nil {
			return nil, fmt.Errorf("get default repo for %q: %w", lpProject, err)
		}
		repoSelfLink = repoLink

		// If series are configured, expand artifacts into series-based recipes.
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
			// No series: use default branch for all recipes.
			refPath := "refs/heads/" + defaultBranch
			refLink, err := s.repoManager.GetGitRef(ctx, repoSelfLink, refPath)
			if err != nil {
				return nil, fmt.Errorf("get git ref %q: %w", refPath, err)
			}
			for _, name := range recipes {
				gitRefLinks[name] = refLink
				buildPaths[name] = pb.Strategy.BuildPath(name)
			}
		}
	}

	result := &TriggerResult{Project: projectName}

	// Assess and execute recipe actions concurrently.
	type triggerJob struct {
		index int
		name  string
	}
	type triggerResult struct {
		index  int
		result RecipeResult
	}

	workerCount := min(defaultConcurrency, len(recipes))
	jobs := make(chan triggerJob)
	results := make(chan triggerResult, len(recipes))

	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				status := s.assessRecipe(ctx, pb, job.name)
				refLink := gitRefLinks[job.name]
				bp := buildPaths[job.name]
				s.logger.Debug("dispatching recipe action",
					"recipe", job.name, "action", status.Action,
					"repoSelfLink", repoSelfLink, "gitRefLink", refLink, "buildPath", bp)
				rr := s.executeAction(ctx, pb, status, opts, repoSelfLink, refLink, bp)
				results <- triggerResult{index: job.index, result: rr}
			}
		}()
	}

	for i, name := range recipes {
		if ctx.Err() != nil {
			break
		}
		jobs <- triggerJob{index: i, name: name}
	}
	close(jobs)
	wg.Wait()
	close(results)

	// Reassemble results in original order.
	ordered := make([]RecipeResult, len(recipes))
	for tr := range results {
		ordered[tr.index] = tr.result
	}

	var recipePtrs []*dto.Recipe
	for _, rr := range ordered {
		if rr.Name == "" {
			continue // skipped due to context cancellation
		}
		result.RecipeResults = append(result.RecipeResults, rr)
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
		// Replace results with final build states from the wait loop.
		for i := range result.RecipeResults {
			rr := &result.RecipeResults[i]
			var recipeBuilds []dto.Build
			for _, b := range builds {
				if b.Recipe == rr.Name {
					recipeBuilds = append(recipeBuilds, b)
				}
			}
			if len(recipeBuilds) > 0 {
				rr.Builds = recipeBuilds
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

	setErr := func(err error) {
		result.Error = err
		result.ErrorMessage = err.Error()
		s.logger.Error("recipe action failed", "recipe", status.Name, "action", status.Action, "error", err)
	}

	switch status.Action {
	case ActionCreateRecipe:
		if repoSelfLink == "" || gitRefLink == "" {
			setErr(fmt.Errorf("recipe %q not found (create requires git repo info; use local mode or enable official_codehosting)", status.Name))
			return result
		}
		bp := buildPath
		s.logger.Info("creating recipe", "recipe", status.Name, "owner", pb.Owner, "project", pb.RecipeProject(), "buildPath", bp)
		createOpts := dto.CreateRecipeOpts{
			Name:        status.Name,
			Owner:       pb.Owner,
			Project:     pb.RecipeProject(),
			GitRepoLink: repoSelfLink,
			GitRefLink:  gitRefLink,
			BuildPath:   bp,
		}
		// LP has a propagation delay between its git indexer and recipe
		// service. Retry on "No such object" errors for the git_ref.
		var recipe *dto.Recipe
		var err error
		for attempt := range 5 {
			recipe, err = pb.Builder.CreateRecipe(ctx, createOpts)
			if err == nil || !strings.Contains(err.Error(), "No such object") {
				break
			}
			s.logger.Warn("LP ref not yet propagated, retrying", "recipe", status.Name, "attempt", attempt+1, "error", err)
			select {
			case <-ctx.Done():
				setErr(ctx.Err())
				return result
			case <-time.After(30 * time.Second):
			}
		}
		if err != nil {
			setErr(fmt.Errorf("create recipe %q: %w", status.Name, err))
			return result
		}
		result.Recipe = recipe
		br, err := pb.Builder.RequestBuilds(ctx, recipe, buildOpts(opts))
		result.BuildRequest = br
		if err != nil {
			setErr(fmt.Errorf("request builds for %q: %w", status.Name, err))
		}

	case ActionRequestBuilds:
		br, err := pb.Builder.RequestBuilds(ctx, status.Recipe, buildOpts(opts))
		result.BuildRequest = br
		if err != nil {
			setErr(fmt.Errorf("request builds for %q: %w", status.Name, err))
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
//
// When opts.Owner is set, it overrides the project's configured owner.
// When opts.RecipeNames is set, it overrides the project's configured recipe list.
func (s *Service) List(ctx context.Context, opts ListOpts) ([]dto.Build, []ProjectResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
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
		if opts.Owner != "" {
			owner = opts.Owner
		}

		targetRef := pb.RecipeProject()
		if opts.TargetRef != "" {
			targetRef = opts.TargetRef
		}

		recipeNames := pb.Artifacts
		if len(opts.RecipeNames) > 0 {
			recipeNames = opts.RecipeNames
		}

		// When a prefix is given without explicit recipe names, discover
		// recipes from LP and filter by prefix + LP project.
		if opts.RecipePrefix != "" && len(opts.RecipeNames) == 0 {
			if owner == "" {
				s.logger.Warn("skipping prefix discovery: owner required", "project", name)
				continue
			}
			allRecipes, err := pb.Builder.ListRecipesByOwner(ctx, owner)
			if err != nil {
				s.logger.Warn("error listing recipes by owner", "project", name, "error", err)
				result.Err = err
				results = append(results, result)
				continue
			}
			recipeNames = nil
			for _, r := range allRecipes {
				if !strings.HasPrefix(r.Name, opts.RecipePrefix) {
					continue
				}
				if targetRef != "" && r.Project != targetRef {
					continue
				}
				recipeNames = append(recipeNames, r.Name)
			}
		}

		for _, recipeName := range recipeNames {
			recipe, err := pb.Builder.GetRecipe(ctx, owner, targetRef, recipeName)
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

// DownloadOpts holds options for downloading build artifacts.
type DownloadOpts struct {
	Projects      []string // project name filter
	ArtifactNames []string // explicit artifact names (maps to recipe names)
	RecipePrefix  string   // discover recipes by prefix
	Owner         string   // override LP owner
	TargetRef     string   // override backend target reference
	OutputDir     string   // output directory
}

// Download retrieves build artifacts for succeeded builds of the given recipes.
func (s *Service) Download(ctx context.Context, opts DownloadOpts) error {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

	for name, pb := range s.projects {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}

		owner := pb.Owner
		if opts.Owner != "" {
			owner = opts.Owner
		}

		targetRef := pb.RecipeProject()
		if opts.TargetRef != "" {
			targetRef = opts.TargetRef
		}

		recipeNames := opts.ArtifactNames
		if len(recipeNames) == 0 && opts.RecipePrefix == "" {
			recipeNames = pb.Artifacts
		}

		// Prefix-based discovery.
		if opts.RecipePrefix != "" && len(opts.ArtifactNames) == 0 {
			if owner == "" {
				s.logger.Warn("skipping prefix discovery: owner required", "project", name)
				continue
			}
			allRecipes, err := pb.Builder.ListRecipesByOwner(ctx, owner)
			if err != nil {
				return fmt.Errorf("listing recipes by owner for %q: %w", name, err)
			}
			recipeNames = nil
			for _, r := range allRecipes {
				if !strings.HasPrefix(r.Name, opts.RecipePrefix) {
					continue
				}
				if targetRef != "" && r.Project != targetRef {
					continue
				}
				recipeNames = append(recipeNames, r.Name)
			}
		}

		for _, recipeName := range recipeNames {
			recipe, err := pb.Builder.GetRecipe(ctx, owner, targetRef, recipeName)
			if err != nil {
				return fmt.Errorf("recipe %q: %w", recipeName, err)
			}
			builds, err := pb.Builder.ListBuilds(ctx, recipe)
			if err != nil {
				return fmt.Errorf("listing builds for %q: %w", recipeName, err)
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
					if err := downloadFile(u, opts.OutputDir, recipeName); err != nil {
						return fmt.Errorf("downloading %s: %w", u, err)
					}
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

// Cleanup removes temporary recipes matching the given prefix and cleans up
// temporary branches. It uses ListRecipesByOwner to discover recipes by prefix
// rather than iterating the configured artifact list.
func (s *Service) Cleanup(ctx context.Context, opts CleanupOpts) (*CleanupResult, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

	owner := opts.Owner
	result := &CleanupResult{}

	for name, pb := range s.projects {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}

		projOwner := pb.Owner
		if owner != "" {
			projOwner = owner
		}

		if projOwner == "" {
			s.logger.Warn("skipping cleanup: owner required", "project", name)
			continue
		}

		targetRef := pb.RecipeProject()
		if opts.TargetRef != "" {
			targetRef = opts.TargetRef
		}

		// Discover recipes by owner and filter by prefix.
		allRecipes, err := pb.Builder.ListRecipesByOwner(ctx, projOwner)
		if err != nil {
			s.logger.Warn("error listing recipes by owner", "project", name, "error", err)
			continue
		}

		// Filter matching recipes.
		var toDelete []*dto.Recipe
		for _, recipe := range allRecipes {
			if opts.Prefix != "" && !strings.HasPrefix(recipe.Name, opts.Prefix) {
				continue
			}
			// When cleaning up by prefix, skip the project filter —
			// temp recipes live under the user's personal LP project,
			// not the configured recipe project.
			if opts.Prefix == "" && targetRef != "" && recipe.Project != "" && recipe.Project != targetRef {
				continue
			}
			toDelete = append(toDelete, recipe)
		}

		if opts.DryRun {
			for _, recipe := range toDelete {
				s.logger.Info("would delete recipe", "recipe", recipe.Name)
				result.DeletedRecipes = append(result.DeletedRecipes, recipe.Name)
			}
		} else {
			deleted := s.deleteRecipesConcurrent(ctx, pb.Builder, toDelete)
			result.DeletedRecipes = append(result.DeletedRecipes, deleted...)
		}
	}

	// Clean up temporary branches if repoManager is available and prefix is set.
	if s.repoManager != nil && opts.Prefix != "" {
		branchResult, err := s.cleanupBranches(ctx, opts)
		if err != nil {
			s.logger.Warn("branch cleanup failed", "error", err)
		} else {
			result.DeletedBranches = append(result.DeletedBranches, branchResult...)
		}
	}

	return result, nil
}

// cleanupBranches removes temporary branches matching the prefix from LP repos.
// Branches live in the user's personal LP project (same as the trigger flow),
// keyed by the watchtower project name — not the code forge project path.
func (s *Service) cleanupBranches(ctx context.Context, opts CleanupOpts) ([]string, error) {
	projFilter := make(map[string]bool, len(opts.Projects))
	for _, p := range opts.Projects {
		projFilter[p] = true
	}

	owner := opts.Owner
	if owner == "" {
		return nil, nil
	}

	// Resolve the user's personal LP build project (same as trigger uses).
	lpProject, err := s.repoManager.GetOrCreateProject(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("resolve LP project for branch cleanup: %w", err)
	}

	branchPrefix := "refs/heads/" + opts.Prefix

	// Check each watchtower project's repo for matching branches.
	var deleted []string
	seen := make(map[string]bool) // track repos we've already cleaned
	for name := range s.projects {
		if len(projFilter) > 0 && !projFilter[name] {
			continue
		}

		// The repo name is the watchtower project name (e.g. "sunbeam-charms"),
		// not the code project path (e.g. "openstack/sunbeam-charms").
		repoSelfLink, _, err := s.repoManager.GetOrCreateRepo(ctx, owner, lpProject, name)
		if err != nil {
			s.logger.Warn("could not resolve repo for branch cleanup", "project", name, "error", err)
			continue
		}
		if seen[repoSelfLink] {
			continue
		}
		seen[repoSelfLink] = true

		branches, err := s.repoManager.ListBranches(ctx, repoSelfLink)
		if err != nil {
			s.logger.Warn("could not list branches for cleanup", "project", name, "error", err)
			continue
		}

		// Filter matching branches.
		var toDelete []dto.BranchRef
		for _, branch := range branches {
			if !strings.HasPrefix(branch.Path, branchPrefix) {
				continue
			}
			toDelete = append(toDelete, branch)
		}

		if opts.DryRun {
			for _, branch := range toDelete {
				s.logger.Info("would delete branch", "branch", branch.Path)
				deleted = append(deleted, branch.Path)
			}
		} else {
			deleted = append(deleted, s.deleteBranchesConcurrent(ctx, toDelete)...)
		}
	}

	return deleted, nil
}

// deleteRecipesConcurrent deletes recipes using a bounded worker pool.
func (s *Service) deleteRecipesConcurrent(ctx context.Context, builder port.RecipeBuilder, recipes []*dto.Recipe) []string {
	if len(recipes) == 0 {
		return nil
	}

	type deleteResult struct {
		name string
		ok   bool
	}

	workerCount := min(defaultConcurrency, len(recipes))
	jobs := make(chan *dto.Recipe)
	results := make(chan deleteResult, len(recipes))

	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for recipe := range jobs {
				if ctx.Err() != nil {
					return
				}
				if err := builder.DeleteRecipe(ctx, recipe.SelfLink); err != nil {
					s.logger.Warn("failed to delete recipe", "recipe", recipe.Name, "error", err)
					continue
				}
				results <- deleteResult{name: recipe.Name, ok: true}
			}
		}()
	}

	for _, recipe := range recipes {
		if ctx.Err() != nil {
			break
		}
		jobs <- recipe
	}
	close(jobs)
	wg.Wait()
	close(results)

	var deleted []string
	for r := range results {
		if r.ok {
			deleted = append(deleted, r.name)
		}
	}
	return deleted
}

// deleteBranchesConcurrent deletes git ref branches using a bounded worker pool.
func (s *Service) deleteBranchesConcurrent(ctx context.Context, branches []dto.BranchRef) []string {
	if len(branches) == 0 {
		return nil
	}

	type deleteResult struct {
		path string
		ok   bool
	}

	workerCount := min(defaultConcurrency, len(branches))
	jobs := make(chan dto.BranchRef)
	results := make(chan deleteResult, len(branches))

	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for branch := range jobs {
				if ctx.Err() != nil {
					return
				}
				if err := s.repoManager.DeleteGitRef(ctx, branch.SelfLink); err != nil {
					s.logger.Warn("failed to delete branch", "branch", branch.Path, "error", err)
					continue
				}
				results <- deleteResult{path: branch.Path, ok: true}
			}
		}()
	}

	for _, branch := range branches {
		if ctx.Err() != nil {
			break
		}
		jobs <- branch
	}
	close(jobs)
	wg.Wait()
	close(results)

	var deleted []string
	for r := range results {
		if r.ok {
			deleted = append(deleted, r.path)
		}
	}
	return deleted
}
