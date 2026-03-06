// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func setRequestTokenURL(u string) { requestTokenURL = u }
func setAccessTokenURL(u string)  { accessTokenURL = u }

func TestObtainRequestToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if got := r.FormValue("oauth_consumer_key"); got != "test-app" {
			t.Errorf("consumer_key = %q, want %q", got, "test-app")
		}
		if got := r.FormValue("oauth_signature_method"); got != "PLAINTEXT" {
			t.Errorf("signature_method = %q, want %q", got, "PLAINTEXT")
		}
		if got := r.FormValue("oauth_signature"); got != "&" {
			t.Errorf("signature = %q, want %q", got, "&")
		}

		w.Write([]byte("oauth_token=req_token_123&oauth_token_secret=req_secret_456"))
	}))
	defer server.Close()

	orig := requestTokenURL
	defer setRequestTokenURL(orig)
	setRequestTokenURL(server.URL)

	rt, err := ObtainRequestToken("test-app")
	if err != nil {
		t.Fatalf("ObtainRequestToken() error: %v", err)
	}

	if rt.Token != "req_token_123" {
		t.Errorf("Token = %q, want %q", rt.Token, "req_token_123")
	}
	if rt.TokenSecret != "req_secret_456" {
		t.Errorf("TokenSecret = %q, want %q", rt.TokenSecret, "req_secret_456")
	}
}

func TestObtainRequestToken_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	orig := requestTokenURL
	defer setRequestTokenURL(orig)
	setRequestTokenURL(server.URL)

	_, err := ObtainRequestToken("test-app")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestObtainRequestToken_MissingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("oauth_token=&oauth_token_secret="))
	}))
	defer server.Close()

	orig := requestTokenURL
	defer setRequestTokenURL(orig)
	setRequestTokenURL(server.URL)

	_, err := ObtainRequestToken("test-app")
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestRequestToken_AuthorizeURL(t *testing.T) {
	rt := &RequestToken{Token: "my_token", TokenSecret: "my_secret"}
	got := rt.AuthorizeURL()
	want := authorizeURL + "?oauth_token=my_token"
	if got != want {
		t.Errorf("AuthorizeURL() = %q, want %q", got, want)
	}
}

func TestExchangeAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if got := r.FormValue("oauth_consumer_key"); got != "test-app" {
			t.Errorf("consumer_key = %q, want %q", got, "test-app")
		}
		if got := r.FormValue("oauth_token"); got != "req_token" {
			t.Errorf("token = %q, want %q", got, "req_token")
		}
		if got := r.FormValue("oauth_signature"); got != "&req_secret" {
			t.Errorf("signature = %q, want %q", got, "&req_secret")
		}

		w.Write([]byte("oauth_token=access_token_789&oauth_token_secret=access_secret_012"))
	}))
	defer server.Close()

	orig := accessTokenURL
	defer setAccessTokenURL(orig)
	setAccessTokenURL(server.URL)

	rt := &RequestToken{Token: "req_token", TokenSecret: "req_secret"}
	creds, err := ExchangeAccessToken("test-app", rt)
	if err != nil {
		t.Fatalf("ExchangeAccessToken() error: %v", err)
	}

	if creds.ConsumerKey != "test-app" {
		t.Errorf("ConsumerKey = %q, want %q", creds.ConsumerKey, "test-app")
	}
	if creds.AccessToken != "access_token_789" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "access_token_789")
	}
	if creds.AccessTokenSecret != "access_secret_012" {
		t.Errorf("AccessTokenSecret = %q, want %q", creds.AccessTokenSecret, "access_secret_012")
	}
}

func TestExchangeAccessToken_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	orig := accessTokenURL
	defer setAccessTokenURL(orig)
	setAccessTokenURL(server.URL)

	rt := &RequestToken{Token: "bad", TokenSecret: "bad"}
	_, err := ExchangeAccessToken("test-app", rt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSignRequest(t *testing.T) {
	creds := &Credentials{
		ConsumerKey:       "my-app",
		AccessToken:       "token123",
		AccessTokenSecret: "secret456",
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.launchpad.net/devel/~me", nil)
	signRequest(req, creds)

	auth := req.Header.Get("Authorization")
	if auth == "" {
		t.Fatal("Authorization header is empty")
	}

	required := []string{
		`OAuth realm=`,
		`oauth_consumer_key="my-app"`,
		`oauth_token="token123"`,
		`oauth_signature_method="PLAINTEXT"`,
		`oauth_signature="&secret456"`,
		`oauth_timestamp=`,
		`oauth_nonce=`,
		`oauth_version="1.0"`,
	}
	for _, param := range required {
		if !strings.Contains(auth, param) {
			t.Errorf("Authorization header missing %q\ngot: %s", param, auth)
		}
	}

	re := regexp.MustCompile(`oauth_nonce="([0-9a-f]+)"`)
	match := re.FindStringSubmatch(auth)
	if len(match) != 2 {
		t.Fatalf("Authorization header missing hex nonce: %s", auth)
	}
	if len(match[1]) != 32 {
		t.Fatalf("expected 32 hex chars in nonce, got %q", match[1])
	}
}
