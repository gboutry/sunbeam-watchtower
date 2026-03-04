// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	requestTokenURL = "https://launchpad.net/+request-token"
	authorizeURL    = "https://launchpad.net/+authorize-token"
	accessTokenURL  = "https://launchpad.net/+access-token"
)

// Credentials holds the OAuth 1.0 tokens needed to authenticate with Launchpad.
type Credentials struct {
	ConsumerKey       string
	AccessToken       string
	AccessTokenSecret string
}

// RequestToken holds the temporary tokens from the first step of the OAuth flow.
type RequestToken struct {
	Token       string
	TokenSecret string
}

// AuthorizeURL returns the URL the user must visit to authorize the request token.
func (rt *RequestToken) AuthorizeURL() string {
	return authorizeURL + "?oauth_token=" + url.QueryEscape(rt.Token)
}

// ObtainRequestToken starts the OAuth 1.0 flow by requesting a temporary token
// from Launchpad.
//
// The consumerKey identifies your application (e.g. "sunbeam-watchtower").
func ObtainRequestToken(consumerKey string) (*RequestToken, error) {
	form := url.Values{
		"oauth_consumer_key":     {consumerKey},
		"oauth_signature_method": {"PLAINTEXT"},
		"oauth_signature":        {"&"},
	}

	resp, err := http.PostForm(requestTokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request token failed (HTTP %d): %s", resp.StatusCode, body)
	}

	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	token := vals.Get("oauth_token")
	secret := vals.Get("oauth_token_secret")
	if token == "" || secret == "" {
		return nil, fmt.Errorf("missing token or secret in response: %s", body)
	}

	return &RequestToken{Token: token, TokenSecret: secret}, nil
}

// ExchangeAccessToken completes the OAuth flow by exchanging an authorized
// request token for permanent access credentials.
//
// The user must have visited rt.AuthorizeURL() and granted access before
// calling this function.
func ExchangeAccessToken(consumerKey string, rt *RequestToken) (*Credentials, error) {
	// Launchpad PLAINTEXT signature: "&" + request_token_secret
	signature := "&" + rt.TokenSecret

	form := url.Values{
		"oauth_token":            {rt.Token},
		"oauth_consumer_key":     {consumerKey},
		"oauth_signature_method": {"PLAINTEXT"},
		"oauth_signature":        {signature},
	}

	resp, err := http.PostForm(accessTokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("exchanging token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("access token failed (HTTP %d): %s", resp.StatusCode, body)
	}

	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	accessToken := vals.Get("oauth_token")
	accessSecret := vals.Get("oauth_token_secret")
	if accessToken == "" || accessSecret == "" {
		return nil, fmt.Errorf("missing token or secret in response: %s", body)
	}

	return &Credentials{
		ConsumerKey:       consumerKey,
		AccessToken:       accessToken,
		AccessTokenSecret: accessSecret,
	}, nil
}

// signRequest adds the OAuth Authorization header to an HTTP request.
//
// Launchpad requires PLAINTEXT signing with all OAuth params in the
// Authorization header only — never in query strings or request bodies.
func signRequest(req *http.Request, creds *Credentials) {
	nonce := strconv.FormatInt(rand.Int64(), 36)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// PLAINTEXT signature: "&" + access_token_secret
	signature := "&" + creds.AccessTokenSecret

	params := []string{
		fmt.Sprintf("OAuth realm=%q", "https://api.launchpad.net/"),
		fmt.Sprintf("oauth_consumer_key=%q", creds.ConsumerKey),
		fmt.Sprintf("oauth_token=%q", creds.AccessToken),
		fmt.Sprintf("oauth_signature_method=%q", "PLAINTEXT"),
		fmt.Sprintf("oauth_signature=%q", signature),
		fmt.Sprintf("oauth_timestamp=%q", timestamp),
		fmt.Sprintf("oauth_nonce=%q", nonce),
		fmt.Sprintf("oauth_version=%q", "1.0"),
	}

	req.Header.Set("Authorization", strings.Join(params, ", "))
}
