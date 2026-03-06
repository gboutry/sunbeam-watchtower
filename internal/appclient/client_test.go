// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package appclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewClient_TCP(t *testing.T) {
	c := NewClient("http://localhost:8080/")
	if c.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, want trailing slash stripped", c.baseURL)
	}
	if c.http == nil {
		t.Fatal("http client is nil")
	}
}

func TestNewClient_Unix(t *testing.T) {
	c := NewClient("unix:///tmp/test.sock")
	if c.baseURL != "http://localhost" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://localhost")
	}
	if c.http == nil {
		t.Fatal("http client is nil")
	}
}

func TestApiError_Error(t *testing.T) {
	t.Run("with detail", func(t *testing.T) {
		e := &apiError{Title: "Not Found", Status: 404, Detail: "resource missing"}
		got := e.Error()
		want := "Not Found (HTTP 404): resource missing"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("without detail", func(t *testing.T) {
		e := &apiError{Title: "Not Found", Status: 404}
		got := e.Error()
		want := "Not Found (HTTP 404)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestGet_Success(t *testing.T) {
	type resp struct {
		Name string `json:"name"`
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp{Name: "test"})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	var got resp
	err := c.get(context.Background(), "/foo", nil, &got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test" {
		t.Errorf("got %q, want %q", got.Name, "test")
	}
}

func TestGet_WithQuery(t *testing.T) {
	var receivedQuery url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	q := url.Values{"page": {"2"}, "size": {"10"}}
	var got map[string]interface{}
	err := c.get(context.Background(), "/items", q, &got)
	if err != nil {
		t.Fatal(err)
	}
	if receivedQuery.Get("page") != "2" {
		t.Errorf("page = %q, want %q", receivedQuery.Get("page"), "2")
	}
	if receivedQuery.Get("size") != "10" {
		t.Errorf("size = %q, want %q", receivedQuery.Get("size"), "10")
	}
}

func TestPost_Success(t *testing.T) {
	type reqBody struct {
		Value int `json:"value"`
	}
	type resp struct {
		ID string `json:"id"`
	}

	var gotCT string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp{ID: "abc"})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	var got resp
	err := c.post(context.Background(), "/create", reqBody{Value: 42}, &got)
	if err != nil {
		t.Fatal(err)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	var sent reqBody
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if sent.Value != 42 {
		t.Errorf("sent value = %d, want 42", sent.Value)
	}
	if got.ID != "abc" {
		t.Errorf("got ID %q, want %q", got.ID, "abc")
	}
}

func TestPost_NilBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Errorf("expected empty body, got %q", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	var got map[string]interface{}
	err := c.post(context.Background(), "/action", nil, &got)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDelete_Success(t *testing.T) {
	var gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	var got map[string]interface{}
	err := c.delete(context.Background(), "/resource/1", nil, &got)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
}

func TestDo_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(apiError{
			Title:  "Not Found",
			Status: 404,
			Detail: "resource missing",
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	var got map[string]interface{}
	err := c.get(context.Background(), "/missing", nil, &got)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ae, ok := err.(*apiError)
	if !ok {
		t.Fatalf("expected *apiError, got %T: %v", err, err)
	}
	if ae.Status != 404 {
		t.Errorf("status = %d, want 404", ae.Status)
	}
	if ae.Title != "Not Found" {
		t.Errorf("title = %q, want %q", ae.Title, "Not Found")
	}
	if ae.Detail != "resource missing" {
		t.Errorf("detail = %q, want %q", ae.Detail, "resource missing")
	}
}

func TestDo_HTTPError_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	var got map[string]interface{}
	err := c.get(context.Background(), "/broken", nil, &got)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := err.(*apiError); ok {
		t.Fatal("expected non-apiError when JSON is invalid")
	}
	want := "HTTP 500 (could not parse error body)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestDo_NilResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":"ignored"}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL)
	err := c.get(context.Background(), "/ok", nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
