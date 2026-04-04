// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package appclient provides a typed HTTP client for the Sunbeam Watchtower API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Client is a typed HTTP client for the Sunbeam Watchtower API server.
type Client struct {
	baseURL string
	http    *http.Client
	token   string
}

type healthResponse struct {
	Status string `json:"status"`
}

// NewClient creates a new Client for the given address.
// The addr may be a unix socket path prefixed with "unix://" (e.g.
// "unix:///run/watchtower.sock") or a standard HTTP URL (e.g.
// "http://localhost:8472").
func NewClient(addr string) *Client {
	if strings.HasPrefix(addr, "unix://") {
		socketPath := strings.TrimPrefix(addr, "unix://")
		return &Client{
			baseURL: "http://localhost",
			http: &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
						return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
					},
				},
			},
		}
	}
	return &Client{
		baseURL: strings.TrimRight(addr, "/"),
		http:    &http.Client{},
	}
}

// NewClientWithToken creates a new Client for the given address with a bearer token
// that will be injected into every request as an Authorization header.
func NewClientWithToken(addr, token string) *Client {
	c := NewClient(addr)
	c.token = token
	return c
}

// apiError is the Huma error format returned by the server.
type apiError struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func (e *apiError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s (HTTP %d): %s", e.Title, e.Status, e.Detail)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Title, e.Status)
}

// get performs a GET request and JSON-decodes the response into result.
func (c *Client) get(ctx context.Context, path string, query url.Values, result interface{}) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	return c.do(req, result)
}

// post performs a POST request with a JSON body and decodes the response into result.
// If result is nil the response body is discarded.
func (c *Client) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.do(req, result)
}

// delete performs a DELETE request and JSON-decodes the response into result.
func (c *Client) delete(ctx context.Context, path string, query url.Values, result interface{}) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	return c.do(req, result)
}

// do executes the request, checks for errors, and optionally decodes the response.
func (c *Client) do(req *http.Request, result interface{}) error {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var ae apiError
		if err := json.NewDecoder(resp.Body).Decode(&ae); err != nil {
			return fmt.Errorf("HTTP %d (could not parse error body)", resp.StatusCode)
		}
		if ae.Status == 0 {
			ae.Status = resp.StatusCode
		}
		return &ae
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// Health checks that the API server is reachable and healthy.
func (c *Client) Health(ctx context.Context) error {
	var result healthResponse
	if err := c.get(ctx, "/api/v1/health", nil, &result); err != nil {
		return err
	}
	if result.Status != "ok" {
		return fmt.Errorf("unexpected health status %q", result.Status)
	}
	return nil
}
