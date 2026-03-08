// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/credentials"
	lpadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/secondary/launchpad"
	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	authsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/auth"
	opsvc "github.com/gboutry/sunbeam-watchtower/internal/core/service/operation"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

func (a *App) stateDir() (string, error) {
	return cacheSubdir("state")
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
		a.lpFlowStore = newLaunchpadPendingAuthFlowStore(a.Logger, a.runtimeMode, a.stateDir)
	})
	return a.lpFlowStore
}

// OperationStore returns the shared long-running operation store.
func (a *App) OperationStore() port.OperationStore {
	a.operationStoreOnce.Do(func() {
		a.operationStore = newOperationStore(a.Logger, a.runtimeMode, a.stateDir)
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
