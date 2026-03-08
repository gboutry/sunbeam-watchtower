// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	if _, err := client.Do(req); err != nil {
		t.Fatalf("Do() error = %v", err)
	}

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
