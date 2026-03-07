// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/bugcache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/distrocache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/excusescache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/gitcache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/releasecache"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	opsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/operation"
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

	releaseCacheOnce sync.Once
	releaseCache     *releasecache.Cache
	releaseCacheErr  error

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
	if a.releaseCache != nil {
		if err := a.releaseCache.Close(); err != nil {
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
