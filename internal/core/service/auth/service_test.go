// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
)

type fakeCredentialStore struct {
	record   *lp.CredentialRecord
	savePath string
}

func (s *fakeCredentialStore) Load(context.Context) (*lp.CredentialRecord, error) {
	if s.record == nil {
		return nil, nil
	}
	recordCopy := *s.record
	return &recordCopy, nil
}

func (s *fakeCredentialStore) Save(_ context.Context, creds *lp.Credentials) (*lp.CredentialRecord, error) {
	s.record = &lp.CredentialRecord{
		Credentials: creds,
		Source:      lp.CredentialSourceFile,
		Path:        s.savePath,
	}
	return s.Load(context.Background())
}

func (s *fakeCredentialStore) Clear(context.Context) error {
	s.record = nil
	return nil
}

type fakeFlowStore struct {
	flows map[string]lp.PendingAuthFlow
	err   error
}

func (s *fakeFlowStore) Put(_ context.Context, flow *lp.PendingAuthFlow) error {
	if s.flows == nil {
		s.flows = make(map[string]lp.PendingAuthFlow)
	}
	s.flows[flow.ID] = *flow
	return s.err
}

func (s *fakeFlowStore) Get(_ context.Context, id string) (*lp.PendingAuthFlow, error) {
	if s.err != nil {
		return nil, s.err
	}
	flow, ok := s.flows[id]
	if !ok {
		return nil, lp.ErrPendingAuthFlowNotFound
	}
	flowCopy := flow
	return &flowCopy, nil
}

func (s *fakeFlowStore) Delete(_ context.Context, id string) error {
	delete(s.flows, id)
	return s.err
}

type fakeLaunchpadAuthenticator struct {
	requestToken *lp.RequestToken
	creds        *lp.Credentials
	identity     lp.Person
	userErr      error
	lastToken    *lp.RequestToken
}

func (a *fakeLaunchpadAuthenticator) ConsumerKey() string { return "sunbeam-watchtower" }

func (a *fakeLaunchpadAuthenticator) ObtainRequestToken(context.Context) (*lp.RequestToken, error) {
	return a.requestToken, nil
}

func (a *fakeLaunchpadAuthenticator) ExchangeAccessToken(_ context.Context, token *lp.RequestToken) (*lp.Credentials, error) {
	a.lastToken = token
	return a.creds, nil
}

func (a *fakeLaunchpadAuthenticator) CurrentUser(context.Context, *lp.Credentials) (lp.Person, error) {
	if a.userErr != nil {
		return lp.Person{}, a.userErr
	}
	return a.identity, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBeginLaunchpadStoresServerSideSecretState(t *testing.T) {
	flows := &fakeFlowStore{}
	svc := NewService(
		&fakeCredentialStore{},
		flows,
		&fakeLaunchpadAuthenticator{
			requestToken: &lp.RequestToken{
				Token:       "request-token",
				TokenSecret: "request-secret",
			},
		},
		testLogger(),
	)
	svc.now = func() time.Time { return time.Date(2026, 3, 6, 20, 0, 0, 0, time.UTC) }
	svc.newFlowID = func() (string, error) { return "flow-123", nil }

	result, err := svc.BeginLaunchpad(context.Background())
	if err != nil {
		t.Fatalf("BeginLaunchpad() error = %v", err)
	}

	if result.FlowID != "flow-123" {
		t.Fatalf("FlowID = %q, want flow-123", result.FlowID)
	}
	if result.AuthorizeURL != "https://launchpad.net/+authorize-token?oauth_token=request-token" {
		t.Fatalf("AuthorizeURL = %q", result.AuthorizeURL)
	}
	if _, ok := flows.flows["flow-123"]; !ok {
		t.Fatal("pending flow not stored")
	}
	if got := flows.flows["flow-123"].RequestTokenSecret; got != "request-secret" {
		t.Fatalf("stored request token secret = %q, want request-secret", got)
	}
}

func TestFinalizeLaunchpadSavesCredentialsAndDeletesFlow(t *testing.T) {
	store := &fakeCredentialStore{savePath: "/tmp/credentials.json"}
	flows := &fakeFlowStore{
		flows: map[string]lp.PendingAuthFlow{
			"flow-123": {
				ID:                 "flow-123",
				RequestToken:       "request-token",
				RequestTokenSecret: "request-secret",
			},
		},
	}
	authenticator := &fakeLaunchpadAuthenticator{
		creds: &lp.Credentials{
			ConsumerKey:       "sunbeam-watchtower",
			AccessToken:       "access-token",
			AccessTokenSecret: "access-secret",
		},
		identity: lp.Person{
			Name:        "gboutry",
			DisplayName: "Guillaume Boutry",
		},
	}
	svc := NewService(store, flows, authenticator, testLogger())

	result, err := svc.FinalizeLaunchpad(context.Background(), "flow-123")
	if err != nil {
		t.Fatalf("FinalizeLaunchpad() error = %v", err)
	}

	if authenticator.lastToken.TokenSecret != "request-secret" {
		t.Fatalf("exchange used secret %q, want request-secret", authenticator.lastToken.TokenSecret)
	}
	if _, ok := flows.flows["flow-123"]; ok {
		t.Fatal("flow was not deleted after finalize")
	}
	if !result.Launchpad.Authenticated {
		t.Fatal("expected authenticated status after finalize")
	}
	if result.Launchpad.CredentialsPath != "/tmp/credentials.json" {
		t.Fatalf("credentials path = %q", result.Launchpad.CredentialsPath)
	}
}

func TestStatusReportsInvalidStoredCredentialsWithoutFailing(t *testing.T) {
	svc := NewService(
		&fakeCredentialStore{
			record: &lp.CredentialRecord{
				Credentials: &lp.Credentials{AccessToken: "token", AccessTokenSecret: "secret"},
				Source:      lp.CredentialSourceFile,
				Path:        "/tmp/credentials.json",
			},
		},
		&fakeFlowStore{},
		&fakeLaunchpadAuthenticator{userErr: errors.New("invalid credentials")},
		testLogger(),
	)

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Launchpad.Authenticated {
		t.Fatal("expected unauthenticated status when verification fails")
	}
	if status.Launchpad.Error == "" {
		t.Fatal("expected verification error in auth status")
	}
}

func TestLogoutLaunchpadRejectsEnvironmentCredentials(t *testing.T) {
	svc := NewService(
		&fakeCredentialStore{
			record: &lp.CredentialRecord{
				Credentials: &lp.Credentials{AccessToken: "token", AccessTokenSecret: "secret"},
				Source:      lp.CredentialSourceEnvironment,
			},
		},
		&fakeFlowStore{},
		&fakeLaunchpadAuthenticator{},
		testLogger(),
	)

	_, err := svc.LogoutLaunchpad(context.Background())
	if !errors.Is(err, ErrLaunchpadEnvironmentCredentials) {
		t.Fatalf("LogoutLaunchpad() error = %v, want %v", err, ErrLaunchpadEnvironmentCredentials)
	}
}
