// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/core/port"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

const (
	// DefaultLaunchpadFlowTTL is the default lifetime of a pending auth flow.
	DefaultLaunchpadFlowTTL = 10 * time.Minute
)

var (
	// ErrLaunchpadAuthFlowNotFound indicates the requested pending auth flow was not found.
	ErrLaunchpadAuthFlowNotFound = errors.New("launchpad auth flow not found")
	// ErrLaunchpadAuthFlowExpired indicates the requested pending auth flow has expired.
	ErrLaunchpadAuthFlowExpired = errors.New("launchpad auth flow expired")
	// ErrLaunchpadEnvironmentCredentials indicates logout cannot clear env-provided credentials.
	ErrLaunchpadEnvironmentCredentials = errors.New(
		"launchpad credentials are provided by environment variables and cannot be removed",
	)
)

// Service exposes application-surface authentication workflows.
type Service struct {
	store     port.LaunchpadCredentialStore
	flows     port.LaunchpadPendingAuthFlowStore
	launchpad port.LaunchpadAuthenticator
	logger    *slog.Logger
	now       func() time.Time
	newFlowID func() (string, error)
	flowTTL   time.Duration
}

// NewService creates a new auth service.
func NewService(
	store port.LaunchpadCredentialStore,
	flows port.LaunchpadPendingAuthFlowStore,
	launchpad port.LaunchpadAuthenticator,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		store:     store,
		flows:     flows,
		launchpad: launchpad,
		logger:    logger,
		now:       time.Now,
		newFlowID: randomFlowID,
		flowTTL:   DefaultLaunchpadFlowTTL,
	}
}

// Status returns the current authentication status.
func (s *Service) Status(ctx context.Context) (*dto.AuthStatus, error) {
	record, err := s.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading launchpad credentials: %w", err)
	}

	launchpadStatus, err := s.launchpadStatus(ctx, record)
	if err != nil {
		return nil, err
	}

	return &dto.AuthStatus{Launchpad: *launchpadStatus}, nil
}

// BeginLaunchpad starts a new Launchpad auth flow and stores its server-side secret state.
func (s *Service) BeginLaunchpad(ctx context.Context) (*dto.LaunchpadAuthBeginResult, error) {
	token, err := s.launchpad.ObtainRequestToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("obtaining launchpad request token: %w", err)
	}

	flowID, err := s.newFlowID()
	if err != nil {
		return nil, fmt.Errorf("generating auth flow ID: %w", err)
	}

	now := s.now().UTC()
	flow := &lp.PendingAuthFlow{
		ID:                 flowID,
		RequestToken:       token.Token,
		RequestTokenSecret: token.TokenSecret,
		CreatedAt:          now,
		ExpiresAt:          now.Add(s.flowTTL),
	}
	if err := s.flows.Put(ctx, flow); err != nil {
		return nil, fmt.Errorf("storing launchpad auth flow: %w", err)
	}

	return &dto.LaunchpadAuthBeginResult{
		FlowID:       flowID,
		AuthorizeURL: token.AuthorizeURL(),
		ExpiresAt:    flow.ExpiresAt,
	}, nil
}

// FinalizeLaunchpad completes a pending Launchpad auth flow and persists the resulting credentials.
func (s *Service) FinalizeLaunchpad(ctx context.Context, flowID string) (*dto.LaunchpadAuthFinalizeResult, error) {
	flow, err := s.flows.Get(ctx, flowID)
	if err != nil {
		switch {
		case errors.Is(err, lp.ErrPendingAuthFlowNotFound):
			return nil, ErrLaunchpadAuthFlowNotFound
		case errors.Is(err, lp.ErrPendingAuthFlowExpired):
			return nil, ErrLaunchpadAuthFlowExpired
		default:
			return nil, fmt.Errorf("loading launchpad auth flow: %w", err)
		}
	}

	creds, err := s.launchpad.ExchangeAccessToken(ctx, &lp.RequestToken{
		Token:       flow.RequestToken,
		TokenSecret: flow.RequestTokenSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("exchanging launchpad access token: %w", err)
	}

	if err := s.flows.Delete(ctx, flowID); err != nil {
		return nil, fmt.Errorf("deleting completed launchpad auth flow: %w", err)
	}

	record, err := s.store.Save(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("saving launchpad credentials: %w", err)
	}

	launchpadStatus, err := s.launchpadStatus(ctx, record)
	if err != nil {
		return nil, err
	}

	return &dto.LaunchpadAuthFinalizeResult{Launchpad: *launchpadStatus}, nil
}

// LogoutLaunchpad clears persisted Launchpad credentials when they are file-backed.
func (s *Service) LogoutLaunchpad(ctx context.Context) (*dto.LaunchpadAuthLogoutResult, error) {
	record, err := s.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading launchpad credentials: %w", err)
	}
	if record == nil {
		return &dto.LaunchpadAuthLogoutResult{}, nil
	}
	if record.Source == lp.CredentialSourceEnvironment {
		return nil, ErrLaunchpadEnvironmentCredentials
	}
	if err := s.store.Clear(ctx); err != nil {
		return nil, fmt.Errorf("clearing launchpad credentials: %w", err)
	}

	return &dto.LaunchpadAuthLogoutResult{
		Cleared:         true,
		CredentialsPath: record.Path,
	}, nil
}

func (s *Service) launchpadStatus(
	ctx context.Context,
	record *lp.CredentialRecord,
) (*dto.LaunchpadAuthStatus, error) {
	status := &dto.LaunchpadAuthStatus{}
	if record == nil || record.Credentials == nil {
		return status, nil
	}

	status.Source = record.Source
	if record.Path != "" {
		status.CredentialsPath = record.Path
	}

	identity, statusError := s.lookupLaunchpadIdentity(ctx, record)
	if statusError != "" {
		status.Error = statusError
		return status, nil
	}

	status.Authenticated = true
	status.Username = identity.Name
	status.DisplayName = identity.DisplayName
	return status, nil
}

func randomFlowID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func (s *Service) lookupLaunchpadIdentity(
	ctx context.Context,
	record *lp.CredentialRecord,
) (lp.Person, string) {
	identity, err := s.launchpad.CurrentUser(ctx, record.Credentials)
	if err != nil {
		return lp.Person{}, err.Error()
	}
	return identity, ""
}
