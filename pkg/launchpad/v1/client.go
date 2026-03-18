// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package v1 provides an HTTP client for the Launchpad REST API (devel version).
//
// Authentication uses OAuth 1.0 with PLAINTEXT signing as required by Launchpad.
// Use [Login] for an interactive browser-based flow, or [NewClient] with
// pre-existing credentials.
package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// APIBaseURL is the Launchpad API service root.
	APIBaseURL = "https://api.launchpad.net/devel"
)

// Client is an authenticated Launchpad API client.
type Client struct {
	creds  *Credentials
	http   *http.Client
	logger *slog.Logger
}

// NewClient creates a Client from existing credentials.
func NewClient(creds *Credentials, logger *slog.Logger, httpClients ...*http.Client) *Client {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	httpClient := &http.Client{}
	if len(httpClients) > 0 && httpClients[0] != nil {
		httpClient = httpClients[0]
	}
	return &Client{
		creds:  creds,
		http:   httpClient,
		logger: logger,
	}
}

// Login performs the full interactive OAuth 1.0 flow:
//
//  1. Obtain a request token from Launchpad.
//  2. Print an authorization URL for the user to visit in a browser.
//  3. Wait for the user to confirm they've authorized the application.
//  4. Exchange the authorized request token for permanent access credentials.
//
// The promptFn callback is called with the authorization URL. It must present
// the URL to the user and block until they have authorized the application
// (e.g. prompt "press Enter"). If promptFn is nil, a default console prompt
// is used.
func Login(consumerKey string, promptFn func(authorizeURL string) error, logger *slog.Logger) (*Client, *Credentials, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if promptFn == nil {
		return nil, nil, fmt.Errorf("promptFn is required")
	}

	logger.Info("starting Launchpad OAuth flow")

	rt, err := ObtainRequestToken(consumerKey)
	if err != nil {
		return nil, nil, fmt.Errorf("obtain request token: %w", err)
	}

	authURL := rt.AuthorizeURL()

	if err := promptFn(authURL); err != nil {
		return nil, nil, fmt.Errorf("user prompt: %w", err)
	}

	creds, err := ExchangeAccessToken(consumerKey, rt)
	if err != nil {
		return nil, nil, fmt.Errorf("exchange access token: %w", err)
	}

	logger.Info("authenticated with Launchpad")

	return NewClient(creds, logger), creds, nil
}

// do executes a signed HTTP request and returns the response body.
func (c *Client) do(ctx context.Context, method, rawURL string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	if c.creds != nil {
		signRequest(req, c.creds)
	}

	c.logger.Debug("launchpad request", "method", method, "url", rawURL)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, rawURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, rawURL, resp.StatusCode, respBody)
	}

	return respBody, nil
}

// Get performs a signed GET request to the Launchpad API.
// The path is relative to [APIBaseURL] (e.g. "/~username" or "/devel/bugs/12345").
// If path starts with "https://", it is used as-is (for following self_link fields).
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	u := c.resolveURL(path)
	return c.do(ctx, http.MethodGet, u, nil, "")
}

// GetJSON performs a signed GET and unmarshals the JSON response into dest.
func (c *Client) GetJSON(ctx context.Context, path string, dest any) error {
	data, err := c.Get(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("decoding JSON from %s: %w", path, err)
	}
	return nil
}

// Post performs a signed POST request with form-encoded body.
func (c *Client) Post(ctx context.Context, path string, form url.Values) ([]byte, error) {
	u := c.resolveURL(path)
	return c.do(ctx, http.MethodPost, u, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

// PostJSON performs a signed POST and unmarshals the JSON response into dest.
func (c *Client) PostJSON(ctx context.Context, path string, form url.Values, dest any) error {
	data, err := c.Post(ctx, path, form)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("decoding JSON from %s: %w", path, err)
	}
	return nil
}

// Delete performs a signed DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	u := c.resolveURL(path)
	_, err := c.do(ctx, http.MethodDelete, u, nil, "")
	return err
}

// Patch performs a signed PATCH request with JSON body.
func (c *Client) Patch(ctx context.Context, path string, jsonBody []byte) ([]byte, error) {
	u := c.resolveURL(path)
	return c.do(ctx, http.MethodPatch, u, strings.NewReader(string(jsonBody)), "application/json")
}

// Me returns the authenticated user's information.
func (c *Client) Me(ctx context.Context) (Person, error) {
	var p Person
	if err := c.GetJSON(ctx, "/people/+me", &p); err != nil {
		return Person{}, fmt.Errorf("fetching current user: %w", err)
	}
	return p, nil
}

// GetCollection performs a signed GET and unmarshals a paginated collection response.
func GetCollection[T any](ctx context.Context, c *Client, path string) (*Collection[T], error) {
	var col Collection[T]
	if err := c.GetJSON(ctx, path, &col); err != nil {
		return nil, err
	}
	return &col, nil
}

// GetAllPages fetches all pages of a paginated collection.
func GetAllPages[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	currentPath := path
	for {
		col, err := GetCollection[T](ctx, c, currentPath)
		if err != nil {
			return nil, err
		}
		all = append(all, col.Entries...)
		if col.NextCollectionLink == "" {
			break
		}
		currentPath = col.NextCollectionLink
	}
	return all, nil
}

// wsOpURL appends ?ws.op=<op> (and optional extra params) to a base URL.
func wsOpURL(base, op string, params url.Values) string {
	u, err := url.Parse(base)
	if err != nil {
		return base + "?ws.op=" + op
	}
	q := u.Query()
	q.Set("ws.op", op)
	for k, vs := range params {
		q.Del(k)
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// resolveURL turns a relative path into a full API URL,
// or returns absolute URLs unchanged.
func (c *Client) resolveURL(path string) string {
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		return path
	}
	return APIBaseURL + path
}

// mustBeUTC parses an RFC 3339 date string, converts it to UTC, and
// re-formats it. Launchpad rejects date parameters with non-UTC offsets.
func mustBeUTC(s string) (string, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: %w", s, err)
	}
	return t.UTC().Format(time.RFC3339), nil
}
