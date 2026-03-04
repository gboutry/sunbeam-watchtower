// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	consumerKey    = "sunbeam-watchtower"
	credentialFile = "credentials.json"
)

// ConsumerKey returns the application consumer key.
func ConsumerKey() string { return consumerKey }

// CredentialsPath returns the default path for cached credentials.
func CredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "sunbeam-watchtower", credentialFile)
}

// LoadCredentials attempts to load LP credentials in priority order:
//  1. Environment variables LP_ACCESS_TOKEN + LP_ACCESS_TOKEN_SECRET
//  2. Cached credentials file (~/.config/sunbeam-watchtower/credentials.json)
//
// Returns nil (no error) if no credentials are found.
func LoadCredentials() (*Credentials, error) {
	// 1. Environment variables (CI use case)
	token := os.Getenv("LP_ACCESS_TOKEN")
	secret := os.Getenv("LP_ACCESS_TOKEN_SECRET")
	if token != "" && secret != "" {
		return &Credentials{
			ConsumerKey:       consumerKey,
			AccessToken:       token,
			AccessTokenSecret: secret,
		}, nil
	}

	// 2. File cache
	path := CredentialsPath()
	if path == "" {
		return nil, nil
	}
	return loadCredentialsFile(path)
}

// SaveCredentials persists credentials to the cache file.
func SaveCredentials(creds *Credentials) error {
	path := CredentialsPath()
	if path == "" {
		return fmt.Errorf("cannot determine credentials path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return nil
}

func loadCredentialsFile(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}
	if creds.AccessToken == "" {
		return nil, nil
	}
	creds.ConsumerKey = consumerKey
	return &creds, nil
}
