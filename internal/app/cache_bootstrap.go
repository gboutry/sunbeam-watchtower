// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/bugcache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/distrocache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/excusescache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/gitcache"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

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

func cacheSubdir(name string) (string, error) {
	cacheDir, err := ResolveCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, name), nil
}

// DistroCache returns a lazy-initialized distro cache singleton.
func (a *App) DistroCache() (*distrocache.Cache, error) {
	a.distroOnce.Do(func() {
		path, err := cacheSubdir("distro")
		if err != nil {
			a.distroErr = err
			return
		}
		a.distroCache, a.distroErr = distrocache.NewCache(path, a.Logger)
	})
	return a.distroCache, a.distroErr
}

// GitCache returns a lazy-initialized git cache singleton.
func (a *App) GitCache() (*gitcache.Cache, error) {
	a.gitOnce.Do(func() {
		reposDir, err := cacheSubdir("repos")
		if err != nil {
			a.gitErr = err
			return
		}
		a.Logger.Debug("initializing git cache", "cacheDir", reposDir)
		a.gitCache = gitcache.NewCache(reposDir, a.Logger)
	})
	return a.gitCache, a.gitErr
}

// BugCache returns a lazy-initialized bug cache singleton.
func (a *App) BugCache() (*bugcache.Cache, error) {
	a.bugCacheOnce.Do(func() {
		path, err := cacheSubdir("bugs")
		if err != nil {
			a.bugCacheErr = err
			return
		}
		a.bugCache, a.bugCacheErr = bugcache.NewCache(path, a.Logger)
	})
	return a.bugCache, a.bugCacheErr
}

// ExcusesCache returns a lazy-initialized excuses cache singleton.
func (a *App) ExcusesCache() (*excusescache.Cache, error) {
	a.excusesOnce.Do(func() {
		path, err := cacheSubdir("excuses")
		if err != nil {
			a.excusesErr = err
			return
		}
		a.excusesCache, a.excusesErr = excusescache.NewCache(path, a.ExcusesSources(), a.Logger)
	})
	return a.excusesCache, a.excusesErr
}

// ExcusesSources returns the configured excuses trackers, with built-in Ubuntu/Debian
// defaults synthesized for legacy configs during migration.
func (a *App) ExcusesSources() []dto.ExcusesSource {
	if a == nil || a.Config == nil {
		return dto.KnownExcusesSources()
	}
	return configuredExcusesSources(a.Config)
}

// DefaultExcusesTracker returns the preferred default tracker for list/show commands.
func (a *App) DefaultExcusesTracker() string {
	sources := a.ExcusesSources()
	if _, ok := dto.ExcusesSourceByTracker(sources, dto.ExcusesTrackerUbuntu); ok {
		return dto.ExcusesTrackerUbuntu
	}
	if len(sources) == 0 {
		return dto.ExcusesTrackerUbuntu
	}
	return sources[0].Tracker
}

func configuredExcusesSources(cfg *config.Config) []dto.ExcusesSource {
	if cfg == nil || len(cfg.Packages.Distros) == 0 {
		return dto.KnownExcusesSources()
	}

	distroNames := make([]string, 0, len(cfg.Packages.Distros))
	for name := range cfg.Packages.Distros {
		distroNames = append(distroNames, name)
	}
	sort.Strings(distroNames)

	sources := make([]dto.ExcusesSource, 0, len(distroNames))
	seen := make(map[string]bool, len(distroNames))
	for _, distroName := range distroNames {
		distroCfg := cfg.Packages.Distros[distroName]
		if distroCfg.Excuses == nil {
			continue
		}
		provider := distroCfg.Excuses.Provider
		if provider == "" {
			provider = distroName
		}
		sources = append(sources, dto.ExcusesSource{
			Tracker:  distroName,
			Provider: provider,
			URL:      distroCfg.Excuses.URL,
			TeamURL:  distroCfg.Excuses.TeamURL,
		})
		seen[distroName] = true
	}

	for _, source := range dto.KnownExcusesSources() {
		if seen[source.Tracker] {
			continue
		}
		if _, ok := cfg.Packages.Distros[source.Tracker]; ok {
			sources = append(sources, source)
		}
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Tracker < sources[j].Tracker
	})
	return sources
}
