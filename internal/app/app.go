// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/authflowstore"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/bugcache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/credentials"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/distrocache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/excusescache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/gitcache"
	lpadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/launchpad"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/operationstore"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	authsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/auth"
	opsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/operation"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

var ErrLaunchpadAuthRequired = errors.New("launchpad authentication required")

// RuntimeMode selects how frontend/server state should be managed.
type RuntimeMode string

const (
	// RuntimeModePersistent is intended for long-lived server processes.
	RuntimeModePersistent RuntimeMode = "persistent"
	// RuntimeModeEphemeral is intended for short-lived embedded frontends.
	RuntimeModeEphemeral RuntimeMode = "ephemeral"
)

// Options configures runtime-specific application behavior.
type Options struct {
	RuntimeMode RuntimeMode
}

// App holds shared application state and provides lazy-initialized factories
// for services and adapters. Both the CLI and HTTP API use this layer.
type App struct {
	Config      *config.Config
	Logger      *slog.Logger
	runtimeMode RuntimeMode

	distroOnce  sync.Once
	distroCache *distrocache.Cache
	distroErr   error

	gitOnce  sync.Once
	gitCache *gitcache.Cache
	gitErr   error

	bugCacheOnce sync.Once
	bugCache     *bugcache.Cache
	bugCacheErr  error

	lpCredsOnce  sync.Once
	lpCredsStore port.LaunchpadCredentialStore

	lpFlowOnce  sync.Once
	lpFlowStore port.LaunchpadPendingAuthFlowStore

	excusesOnce  sync.Once
	excusesCache *excusescache.Cache
	excusesErr   error

	operationStoreOnce sync.Once
	operationStore     port.OperationStore

	operationServiceOnce sync.Once
	operationService     *opsvc.Service
}

// NewApp creates a new App instance.
func NewApp(cfg *config.Config, logger *slog.Logger) *App {
	return NewAppWithOptions(cfg, logger, Options{RuntimeMode: RuntimeModePersistent})
}

// NewAppWithOptions creates a new App instance with explicit runtime behavior.
func NewAppWithOptions(cfg *config.Config, logger *slog.Logger, opts Options) *App {
	mode := opts.RuntimeMode
	if mode == "" {
		mode = RuntimeModePersistent
	}
	return &App{Config: cfg, Logger: logger, runtimeMode: mode}
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
	if a.excusesCache != nil {
		if err := a.excusesCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if closer, ok := a.lpFlowStore.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if closer, ok := a.operationStore.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
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

// ExcusesCache returns a lazy-initialized excuses cache singleton.
func (a *App) ExcusesCache() (*excusescache.Cache, error) {
	a.excusesOnce.Do(func() {
		cacheDir, err := ResolveCacheDir()
		if err != nil {
			a.excusesErr = err
			return
		}
		a.excusesCache, a.excusesErr = excusescache.NewCache(filepath.Join(cacheDir, "excuses"), a.ExcusesSources(), a.Logger)
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

// LaunchpadCredentialStore returns the shared Launchpad credential store.
func (a *App) LaunchpadCredentialStore() port.LaunchpadCredentialStore {
	a.lpCredsOnce.Do(func() {
		a.lpCredsStore = credentials.NewLaunchpadStore("")
	})
	return a.lpCredsStore
}

// LaunchpadPendingAuthFlowStore returns the shared pending Launchpad auth flow store.
func (a *App) LaunchpadPendingAuthFlowStore() port.LaunchpadPendingAuthFlowStore {
	a.lpFlowOnce.Do(func() {
		if a.runtimeMode == RuntimeModeEphemeral {
			a.lpFlowStore = authflowstore.NewMemoryLaunchpadFlowStore()
			return
		}

		cacheDir, err := ResolveCacheDir()
		if err != nil {
			a.Logger.Warn("failed to resolve cache dir for auth flow store, falling back to memory", "error", err)
			a.lpFlowStore = authflowstore.NewMemoryLaunchpadFlowStore()
			return
		}

		store, err := authflowstore.NewBoltLaunchpadFlowStore(filepath.Join(cacheDir, "state"))
		if err != nil {
			a.Logger.Warn("failed to initialize auth flow store, falling back to memory", "error", err)
			a.lpFlowStore = authflowstore.NewMemoryLaunchpadFlowStore()
			return
		}
		a.lpFlowStore = store
	})
	return a.lpFlowStore
}

// OperationStore returns the shared long-running operation store.
func (a *App) OperationStore() port.OperationStore {
	a.operationStoreOnce.Do(func() {
		if a.runtimeMode == RuntimeModeEphemeral {
			a.operationStore = operationstore.NewMemoryStore()
			return
		}

		cacheDir, err := ResolveCacheDir()
		if err != nil {
			a.Logger.Warn("failed to resolve cache dir for operation store, falling back to memory", "error", err)
			a.operationStore = operationstore.NewMemoryStore()
			return
		}

		store, err := operationstore.NewBoltStore(filepath.Join(cacheDir, "state"))
		if err != nil {
			a.Logger.Warn("failed to initialize operation store, falling back to memory", "error", err)
			a.operationStore = operationstore.NewMemoryStore()
			return
		}
		a.operationStore = store
	})
	return a.operationStore
}

// OperationService returns the shared async operation service.
func (a *App) OperationService() (*opsvc.Service, error) {
	a.operationServiceOnce.Do(func() {
		a.operationService = opsvc.NewService(a.OperationStore(), a.Logger)
	})

	return a.operationService, nil
}

// AuthService creates the shared auth service.
func (a *App) AuthService() (*authsvc.Service, error) {
	return authsvc.NewService(
		a.LaunchpadCredentialStore(),
		a.LaunchpadPendingAuthFlowStore(),
		lpadapter.NewAuthenticator(lp.ConsumerKey(), a.Logger),
		a.Logger,
	), nil
}
