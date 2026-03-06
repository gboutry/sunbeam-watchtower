// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/andygrunwald/go-gerrit"
	"github.com/google/go-github/v68/github"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/bugcache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/distrocache"
	adaptergit "github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/git"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/gitcache"
	lpadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/launchpad"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/openstack"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/bug"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/build"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/commit"
	projectsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/project"
	"github.com/gboutry/sunbeam-watchtower/internal/core/service/review"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

var ErrLaunchpadAuthRequired = errors.New("launchpad authentication required")

// App holds shared application state and provides lazy-initialized factories
// for services and adapters. Both the CLI and HTTP API use this layer.
type App struct {
	Config *config.Config
	Logger *slog.Logger

	distroOnce  sync.Once
	distroCache *distrocache.Cache
	distroErr   error

	gitOnce  sync.Once
	gitCache *gitcache.Cache
	gitErr   error

	bugCacheOnce sync.Once
	bugCache     *bugcache.Cache
	bugCacheErr  error
}

// NewApp creates a new App instance.
func NewApp(cfg *config.Config, logger *slog.Logger) *App {
	return &App{Config: cfg, Logger: logger}
}

// Close releases resources held by the App (e.g. distro cache).
func (a *App) Close() error {
	var errs []error
	if a.distroCache != nil {
		if err := a.distroCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if a.bugCache != nil {
		if err := a.bugCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ResolveCacheDir returns the cache directory for sunbeam-watchtower.
// It uses $XDG_CACHE_HOME/sunbeam-watchtower if set, otherwise ~/.cache/sunbeam-watchtower.
func ResolveCacheDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home directory: %w", err)
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "sunbeam-watchtower"), nil
}

// DistroCache returns a lazy-initialized distro cache singleton.
func (a *App) DistroCache() (*distrocache.Cache, error) {
	a.distroOnce.Do(func() {
		cacheDir, err := ResolveCacheDir()
		if err != nil {
			a.distroErr = err
			return
		}
		a.distroCache, a.distroErr = distrocache.NewCache(filepath.Join(cacheDir, "distro"), a.Logger)
	})
	return a.distroCache, a.distroErr
}

// GitCache returns a lazy-initialized git cache singleton.
func (a *App) GitCache() (*gitcache.Cache, error) {
	a.gitOnce.Do(func() {
		cacheDir, err := ResolveCacheDir()
		if err != nil {
			a.gitErr = err
			return
		}
		a.Logger.Debug("resolved cache directory", "path", cacheDir)
		reposDir := filepath.Join(cacheDir, "repos")
		a.Logger.Debug("initializing git cache", "cacheDir", reposDir)
		a.gitCache = gitcache.NewCache(reposDir, a.Logger)
	})
	return a.gitCache, a.gitErr
}

// BugCache returns a lazy-initialized bug cache singleton.
func (a *App) BugCache() (*bugcache.Cache, error) {
	a.bugCacheOnce.Do(func() {
		cacheDir, err := ResolveCacheDir()
		if err != nil {
			a.bugCacheErr = err
			return
		}
		a.bugCache, a.bugCacheErr = bugcache.NewCache(filepath.Join(cacheDir, "bugs"), a.Logger)
	})
	return a.bugCache, a.bugCacheErr
}

// NewLaunchpadClient creates an LP client with credentials from env/file cache.
// Returns nil if no credentials are available.
func NewLaunchpadClient(lpCfg config.LaunchpadConfig, logger *slog.Logger) *lp.Client {
	_ = lpCfg
	creds, err := lp.LoadCredentials()
	if err != nil {
		logger.Warn("failed to load LP credentials", "error", err)
		return nil
	}
	if creds == nil {
		return nil
	}
	return lp.NewClient(creds, logger)
}

// NewLaunchpadForge creates a LaunchpadForge client, or nil if no auth is available.
func NewLaunchpadForge(lpCfg config.LaunchpadConfig, logger *slog.Logger) *forge.LaunchpadForge {
	client := NewLaunchpadClient(lpCfg, logger)
	if client == nil {
		return nil
	}
	return forge.NewLaunchpadForge(client)
}

// ForgeTypeFromConfig maps a config forge name string to a forge.ForgeType.
func ForgeTypeFromConfig(forgeName string) forge.ForgeType {
	switch forgeName {
	case "github":
		return forge.ForgeGitHub
	case "launchpad":
		return forge.ForgeLaunchpad
	case "gerrit":
		return forge.ForgeGerrit
	default:
		return forge.ForgeGitHub
	}
}

// MRRefSpecs returns the additional git refspecs needed to fetch MR refs for a given forge.
func MRRefSpecs(forgeName string) []string {
	switch forgeName {
	case "github":
		return []string{"+refs/pull/*/head:refs/pull/*/head"}
	case "gerrit":
		return []string{"+refs/changes/*:refs/changes/*"}
	default:
		return nil
	}
}

// MRGitRef computes the git ref path for a merge request ID on the given forge.
func MRGitRef(forgeName string, mrID string) string {
	switch forgeName {
	case "github":
		return fmt.Sprintf("refs/pull/%s/head", strings.TrimPrefix(mrID, "#"))
	default:
		return ""
	}
}

// ConvertToMRMetadata converts forge MergeRequests to dto.MRMetadata entries.
func ConvertToMRMetadata(mrs []forge.MergeRequest, forgeName string) []dto.MRMetadata {
	result := make([]dto.MRMetadata, 0, len(mrs))
	for _, mr := range mrs {
		result = append(result, dto.MRMetadata{
			ID:     mr.ID,
			State:  mr.State,
			URL:    mr.URL,
			GitRef: MRGitRef(forgeName, mr.ID),
		})
	}
	return result
}

// UpstreamCacheDir returns the path to the upstream repos cache directory.
func UpstreamCacheDir() (string, error) {
	cacheDir, err := ResolveCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "upstream"), nil
}

// UpstreamRepoPath returns the local path for a given upstream repo URL.
func UpstreamRepoPath(cacheDir, repoURL string) string {
	name := filepath.Base(repoURL)
	name = strings.TrimSuffix(name, ".git")
	return filepath.Join(cacheDir, name)
}

// BuildForgeClients creates forge clients from config, caching one per forge type/host.
func (a *App) BuildForgeClients() (map[string]review.ProjectForge, error) {
	cfg := a.Config
	if cfg == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	a.Logger.Debug("building forge clients", "project_count", len(cfg.Projects))

	result := make(map[string]review.ProjectForge, len(cfg.Projects))

	var ghClient *forge.GitHubForge
	gerritClients := make(map[string]*forge.GerritForge)
	var lpClient *forge.LaunchpadForge

	for _, proj := range cfg.Projects {
		var pf review.ProjectForge
		code := proj.Code

		a.Logger.Debug("configuring forge client", "project", proj.Name, "forge", code.Forge)

		switch code.Forge {
		case "github":
			if ghClient == nil {
				ghClient = forge.NewGitHubForge(github.NewClient(nil))
			}
			pf = review.ProjectForge{
				Forge:     ghClient,
				ProjectID: code.Owner + "/" + code.Project,
			}

		case "gerrit":
			gc, ok := gerritClients[code.Host]
			if !ok {
				client, err := gerrit.NewClient(context.Background(), code.Host, nil)
				if err != nil {
					return nil, fmt.Errorf("creating Gerrit client for %s: %w", code.Host, err)
				}
				gc = forge.NewGerritForge(client, code.Host)
				gerritClients[code.Host] = gc
			}
			pf = review.ProjectForge{
				Forge:     gc,
				ProjectID: code.Project,
			}

		case "launchpad":
			if lpClient == nil {
				lpClient = NewLaunchpadForge(cfg.Launchpad, a.Logger)
			}
			if lpClient == nil {
				a.Logger.Warn("skipping Launchpad project (no auth configured)", "project", proj.Name)
				continue
			}
			pf = review.ProjectForge{
				Forge:     lpClient,
				ProjectID: code.Project,
			}

		default:
			return nil, fmt.Errorf("unknown forge type %q for project %s", code.Forge, proj.Name)
		}

		result[proj.Name] = pf
	}

	return result, nil
}

// BuildBugTrackers creates bug tracker clients from config, deduplicating by (forge, project).
func (a *App) BuildBugTrackers() (map[string]bug.ProjectBugTracker, map[string][]string, error) {
	cfg := a.Config
	if cfg == nil {
		return nil, nil, fmt.Errorf("no configuration loaded")
	}

	trackers := make(map[string]bug.ProjectBugTracker)
	projectMap := make(map[string][]string)

	var lpBugTracker *forge.LaunchpadBugTracker

	cache, cacheErr := a.BugCache()
	if cacheErr != nil {
		a.Logger.Warn("bug cache unavailable, using live trackers only", "error", cacheErr)
	}

	for _, proj := range cfg.Projects {
		for _, b := range proj.Bugs {
			switch b.Forge {
			case "launchpad":
				if lpBugTracker == nil {
					lpClient := NewLaunchpadClient(cfg.Launchpad, a.Logger)
					if lpClient == nil {
						a.Logger.Warn("skipping Launchpad bug tracker (no auth configured)", "project", proj.Name)
						continue
					}
					lpBugTracker = forge.NewLaunchpadBugTracker(lpClient)
				}

				key := "launchpad:" + b.Project
				if _, ok := trackers[key]; !ok {
					var tracker port.BugTracker = lpBugTracker
					if cache != nil {
						tracker = bugcache.NewCachedBugTracker(lpBugTracker, cache, b.Project, a.Logger)
					}
					trackers[key] = bug.ProjectBugTracker{
						Tracker:   tracker,
						ProjectID: b.Project,
					}
				}
				projectMap[key] = append(projectMap[key], proj.Name)

			default:
				return nil, nil, fmt.Errorf("unsupported bug tracker forge %q for project %s", b.Forge, proj.Name)
			}
		}
	}

	return trackers, projectMap, nil
}

// BuildRecipeBuilders creates per-project RecipeBuilder instances from config.
func (a *App) BuildRecipeBuilders() (map[string]build.ProjectBuilder, error) {
	cfg := a.Config
	if cfg == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	result := make(map[string]build.ProjectBuilder)
	var lpClient *lp.Client

	for _, proj := range cfg.Projects {
		if proj.Build == nil && proj.ArtifactType == "" {
			continue
		}

		if lpClient == nil {
			lpClient = NewLaunchpadClient(cfg.Launchpad, a.Logger)
			if lpClient == nil {
				a.Logger.Warn("skipping build projects (no LP auth configured)")
				return result, nil
			}
		}

		artifactType := proj.ArtifactType

		var builder port.RecipeBuilder
		var strategy build.ArtifactStrategy
		switch artifactType {
		case "rock":
			builder = lpadapter.NewRockBuilder(lpClient)
			strategy = &build.RockStrategy{}
		case "charm":
			builder = lpadapter.NewCharmBuilder(lpClient)
			strategy = &build.CharmStrategy{}
		case "snap":
			builder = lpadapter.NewSnapBuilder(lpClient, "", "")
			strategy = &build.SnapStrategy{}
		default:
			return nil, fmt.Errorf("unsupported artifact type %q for project %s", artifactType, proj.Name)
		}

		var owner string
		var recipes []string
		var lpProject string
		var officialCodehosting bool
		if proj.Build != nil {
			owner = proj.Build.Owner
			recipes = proj.Build.Recipes
			lpProject = proj.Build.LPProject
			officialCodehosting = proj.Build.OfficialCodehosting
		}

		series := proj.Series
		if len(series) == 0 {
			series = cfg.Launchpad.Series
		}
		devFocus := proj.DevelopmentFocus
		if devFocus == "" {
			devFocus = cfg.Launchpad.DevelopmentFocus
		}

		result[proj.Name] = build.ProjectBuilder{
			Builder:             builder,
			Owner:               owner,
			Project:             proj.Code.Project,
			LPProject:           lpProject,
			Recipes:             recipes,
			Series:              series,
			DevFocus:            devFocus,
			OfficialCodehosting: officialCodehosting,
			Strategy:            strategy,
		}
	}

	return result, nil
}

// BuildService creates the build service with all required dependencies wired.
func (a *App) BuildService() (*build.Service, error) {
	builders, err := a.BuildRecipeBuilders()
	if err != nil {
		return nil, err
	}
	repoMgr, err := a.BuildRepoManager()
	if err != nil {
		return nil, err
	}
	gitClient := adaptergit.NewClient(a.Logger)
	return build.NewService(builders, repoMgr, gitClient, a.Logger), nil
}

// BuildRepoManager creates a RepoManager backed by Launchpad.
func (a *App) BuildRepoManager() (port.RepoManager, error) {
	if a.Config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	lpClient := NewLaunchpadClient(a.Config.Launchpad, a.Logger)
	if lpClient == nil {
		return nil, nil
	}

	return lpadapter.NewRepoManager(lpClient, a.Logger), nil
}

// BuildProjectSyncConfigs resolves project sync configuration from the loaded config.
func (a *App) BuildProjectSyncConfigs() (map[string]projectsvc.ProjectSyncConfig, error) {
	if a.Config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	projectConfigs := make(map[string]projectsvc.ProjectSyncConfig)
	for _, proj := range a.Config.Projects {
		for _, b := range proj.Bugs {
			if b.Forge != "launchpad" {
				continue
			}
			if _, ok := projectConfigs[b.Project]; ok {
				continue
			}
			psc := projectsvc.ProjectSyncConfig{
				Series:           a.Config.Launchpad.Series,
				DevelopmentFocus: a.Config.Launchpad.DevelopmentFocus,
			}
			if len(proj.Series) > 0 {
				psc.Series = proj.Series
			}
			if proj.DevelopmentFocus != "" {
				psc.DevelopmentFocus = proj.DevelopmentFocus
			}
			projectConfigs[b.Project] = psc
		}
	}

	return projectConfigs, nil
}

// ProjectService creates the project sync service with config-derived project settings.
func (a *App) ProjectService() (*projectsvc.Service, error) {
	projectConfigs, err := a.BuildProjectSyncConfigs()
	if err != nil {
		return nil, err
	}
	if len(projectConfigs) == 0 {
		return projectsvc.NewService(nil, projectConfigs, a.Logger), nil
	}

	lpClient := NewLaunchpadClient(a.Config.Launchpad, a.Logger)
	if lpClient == nil {
		return nil, ErrLaunchpadAuthRequired
	}

	manager := lpadapter.NewProjectManager(lpClient)
	return projectsvc.NewService(manager, projectConfigs, a.Logger), nil
}

// BuildUpstreamProvider creates an UpstreamProvider from config, or returns nil
// if upstream is not configured.
func (a *App) BuildUpstreamProvider() (port.UpstreamProvider, error) {
	cfg := a.Config
	if cfg == nil || cfg.Packages.Upstream == nil {
		return nil, nil
	}

	up := cfg.Packages.Upstream
	if up.Provider != "openstack" {
		return nil, fmt.Errorf("unsupported upstream provider %q", up.Provider)
	}

	upDir, err := UpstreamCacheDir()
	if err != nil {
		return nil, err
	}

	releasesDir := UpstreamRepoPath(upDir, up.ReleasesRepo)
	requirementsDir := UpstreamRepoPath(upDir, up.RequirementsRepo)

	return openstack.NewProvider(releasesDir, requirementsDir), nil
}

// BuildCommitSources creates commit sources backed by the local git cache.
func (a *App) BuildCommitSources() (map[string]port.CommitSource, error) {
	cfg := a.Config
	if cfg == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	a.Logger.Debug("building commit sources", "project_count", len(cfg.Projects))

	cache, err := a.GitCache()
	if err != nil {
		return nil, err
	}

	result := make(map[string]port.CommitSource, len(cfg.Projects))
	for _, proj := range cfg.Projects {
		cloneURL, err := proj.Code.CloneURL()
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", proj.Name, err)
		}

		a.Logger.Debug("configured commit source", "project", proj.Name, "cloneURL", cloneURL)

		forgeType := ForgeTypeFromConfig(proj.Code.Forge)
		result[proj.Name] = &commit.CachedGitSource{
			Cache:     cache,
			CloneURL:  cloneURL,
			Type:      forgeType,
			CommitURL: proj.Code.CommitURL,
		}
	}

	return result, nil
}

// SyncBugCache syncs the bug cache for configured projects.
// If project is empty, all configured projects are synced.
func (a *App) SyncBugCache(ctx context.Context, project string) (int, error) {
	trackers, _, err := a.BuildBugTrackers()
	if err != nil {
		return 0, err
	}

	total := 0
	for _, pbt := range trackers {
		if project != "" && pbt.ProjectID != project {
			continue
		}
		ct, ok := pbt.Tracker.(*bugcache.CachedBugTracker)
		if !ok {
			continue
		}
		synced, sErr := ct.Sync(ctx)
		if sErr != nil {
			return total, fmt.Errorf("syncing %s: %w", pbt.ProjectID, sErr)
		}
		total += synced
	}

	return total, nil
}

// BuildPackageSources resolves distro, release, suite, and backport filters against config
// to produce source entries for the package cache.
//
// Suite types are expanded relative to each release: "release" → release name,
// "updates" → "<release>-updates", etc.
//
// Backport filter semantics:
//   - empty/nil or ["none"]: skip all backports (default)
//   - ["gazpacho", "flamingo"]: include only those backports
func (a *App) BuildPackageSources(distros, releases, suites, backports []string) []dto.PackageSource {
	cfg := a.Config.Packages
	var sources []dto.PackageSource

	// Build backport filter.
	// nil → include all backports (used by cache sync)
	// ["none"] → skip all backports (default for query commands)
	// ["gazpacho", ...] → include only named backports
	bpFilter := make(map[string]bool, len(backports))
	for _, bp := range backports {
		bpFilter[bp] = true
	}
	includeAllBackports := backports == nil
	skipAllBackports := !includeAllBackports && (len(bpFilter) == 0 || bpFilter["none"])
	filterBackports := !includeAllBackports && !skipAllBackports

	// Build release filter.
	relFilter := make(map[string]bool, len(releases))
	for _, r := range releases {
		relFilter[r] = true
	}
	filterReleases := len(relFilter) > 0

	// Build suite-type filter.
	stFilter := make(map[string]bool, len(suites))
	for _, s := range suites {
		stFilter[s] = true
	}
	filterSuiteTypes := len(stFilter) > 0

	// Resolve distros.
	distroNames := distros
	if len(distroNames) == 0 {
		for name := range cfg.Distros {
			distroNames = append(distroNames, name)
		}
	}

	for _, name := range distroNames {
		if name == "none" {
			continue
		}
		d, ok := cfg.Distros[name]
		if !ok {
			a.Logger.Warn("unknown distro in config, skipping", "distro", name)
			continue
		}

		// When backports are requested without an explicit --release filter,
		// infer releases from the config:
		//   - parent_release (where packages are uploaded natively) → full main suites
		//   - backport target (where the backport config lives) → backport pockets only
		// e.g. --backport gazpacho: resolute gets full suites, noble gets only gazpacho pockets.
		effectiveRelFilter := relFilter
		effectiveFilterReleases := filterReleases
		backportOnlyReleases := make(map[string]bool)
		if filterBackports && !filterReleases {
			effectiveRelFilter = make(map[string]bool)
			parentReleases := make(map[string]bool)
			for relName, rel := range d.Releases {
				for bpName, bp := range rel.Backports {
					if bpFilter[bpName] {
						effectiveRelFilter[relName] = true
						backportOnlyReleases[relName] = true
						if bp.ParentRelease != "" {
							effectiveRelFilter[bp.ParentRelease] = true
							parentReleases[bp.ParentRelease] = true
						}
					}
				}
			}
			// A release that is both a backport target and a parent release
			// for another requested backport gets full suites.
			for r := range parentReleases {
				delete(backportOnlyReleases, r)
			}
			effectiveFilterReleases = len(effectiveRelFilter) > 0
		}

		var entries []dto.SourceEntry
		for relName, rel := range d.Releases {
			if effectiveFilterReleases && !effectiveRelFilter[relName] {
				continue
			}
			// For backport-only releases, skip main suites — only include backport pockets.
			if !backportOnlyReleases[relName] {
				for _, suiteType := range rel.Suites {
					if filterSuiteTypes && !stFilter[suiteType] {
						continue
					}
					fullSuite := config.ExpandSuiteType(relName, suiteType)
					for _, comp := range d.Components {
						entries = append(entries, dto.SourceEntry{
							Mirror:    d.Mirror,
							Suite:     fullSuite,
							Component: comp,
						})
					}
				}
			}

			if skipAllBackports {
				continue
			}

			// Include backports belonging to this release.
			for bpName, bp := range rel.Backports {
				if filterBackports && !bpFilter[bpName] {
					continue
				}
				qualifiedName := name + "/" + bpName
				var bpEntries []dto.SourceEntry
				for _, src := range bp.Sources {
					for _, suite := range src.Suites {
						expandedSuite := config.ExpandBackportSuiteType(relName, bpName, suite)
						for _, comp := range src.Components {
							bpEntries = append(bpEntries, dto.SourceEntry{
								Mirror:    src.Mirror,
								Suite:     expandedSuite,
								Component: comp,
							})
						}
					}
				}
				sources = append(sources, dto.PackageSource{
					Name:    qualifiedName,
					Entries: bpEntries,
				})
			}
		}

		if len(entries) > 0 {
			sources = append(sources, dto.PackageSource{
				Name:    name,
				Entries: entries,
			})
		}
	}

	return sources
}
