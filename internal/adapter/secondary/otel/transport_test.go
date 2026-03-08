// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWrapHTTPClientRecordsOutboundSpanAndPropagatesContext(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	requestSeen := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen <- r.Clone(r.Context())
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := WrapHTTPClient(&http.Client{}, "snapstore")
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/v2/charms/info/keystone-k8s?token=secret", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	seen := <-requestSeen
	if seen.Header.Get("traceparent") == "" {
		t.Fatal("traceparent header not propagated")
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if got := spans[0].Name(); got != "snapstore.http" {
		t.Fatalf("span name = %q, want %q", got, "snapstore.http")
	}
	attrs := spans[0].Attributes()
	assertSpanAttribute(t, attrs, "watchtower.upstream", "snapstore")
	assertSpanAttribute(t, attrs, "url.path", "/v2/charms/info/:id")
	assertSpanAttribute(t, attrs, "http.response.status_code", int64(http.StatusAccepted))
}

func TestSanitizePathNormalizesVariableSegments(t *testing.T) {
	if got := sanitizePath("/v2/snaps/info/consul-client-2024.1"); got != "/v2/snaps/info/:id" {
		t.Fatalf("sanitizePath() = %q", got)
	}
	if got := sanitizePath(""); got != "/" {
		t.Fatalf("sanitizePath(\"\") = %q", got)
	}
}

func TestWrapHTTPClientClonesBaseClient(t *testing.T) {
	base := &http.Client{}
	wrapped := WrapHTTPClient(base, "snapstore")
	if wrapped == base {
		t.Fatal("WrapHTTPClient() should clone the base client")
	}
	if wrapped.Transport == nil {
		t.Fatal("WrapHTTPClient() should install a transport")
	}
	if base.Transport != nil {
		t.Fatal("WrapHTTPClient() should not mutate the base client")
	}
}

func TestWrapHTTPTransportRecordsErrorsAndFailureStatuses(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	transport := WrapHTTPTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	}), "launchpad")
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.invalid/v1/builds/abc123", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatal("RoundTrip() error = nil, want boom")
	}

	errorSpans := recorder.Ended()
	if len(errorSpans) != 1 {
		t.Fatalf("error span count = %d, want 1", len(errorSpans))
	}
	if errorSpans[0].Status().Code != codes.Error {
		t.Fatalf("error span status = %v, want error", errorSpans[0].Status())
	}

	recorder = tracetest.NewSpanRecorder()
	provider = sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	transport = WrapHTTPTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	}), "launchpad")
	resp, err = transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	resp.Body.Close()

	failureSpans := recorder.Ended()
	if len(failureSpans) != 1 {
		t.Fatalf("failure span count = %d, want 1", len(failureSpans))
	}
	if failureSpans[0].Status().Code != codes.Error {
		t.Fatalf("failure span status = %v, want error", failureSpans[0].Status())
	}
}

func TestPathSegmentVariableAndHostname(t *testing.T) {
	cases := []struct {
		part string
		want bool
	}{
		{part: "v2", want: false},
		{part: "keystone", want: false},
		{part: "keystone-2024.1", want: true},
		{part: "abcdef0123456789abcdef0123456789", want: true},
	}
	for _, tc := range cases {
		if got := pathSegmentVariable(tc.part); got != tc.want {
			t.Fatalf("pathSegmentVariable(%q) = %t, want %t", tc.part, got, tc.want)
		}
	}

	if got := hostname("127.0.0.1:8080"); got != "127.0.0.1" {
		t.Fatalf("hostname() = %q, want 127.0.0.1", got)
	}
	if got := hostname("example.invalid"); got != "example.invalid" {
		t.Fatalf("hostname() = %q, want example.invalid", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func assertSpanAttribute(t *testing.T, attrs []attribute.KeyValue, key string, want any) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) != key {
			continue
		}
		switch value := want.(type) {
		case string:
			if got := attr.Value.AsString(); got != value {
				t.Fatalf("%s = %q, want %q", key, got, value)
			}
		case int64:
			if got := attr.Value.AsInt64(); got != value {
				t.Fatalf("%s = %d, want %d", key, got, value)
			}
		default:
			t.Fatalf("unsupported expected type %T", want)
		}
		return
	}
	t.Fatalf("attribute %s not found", key)
}
