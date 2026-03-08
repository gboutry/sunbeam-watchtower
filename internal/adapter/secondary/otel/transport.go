// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"net"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// WrapHTTPClient returns a copy of base whose transport emits outbound HTTP spans.
func WrapHTTPClient(base *http.Client, upstream string) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	clone := *base
	clone.Transport = WrapHTTPTransport(base.Transport, upstream)
	return &clone
}

// WrapHTTPTransport decorates a RoundTripper with outbound tracing.
func WrapHTTPTransport(base http.RoundTripper, upstream string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &tracedTransport{
		base:     base,
		upstream: upstream,
		tracer:   otel.Tracer("watchtower.client"),
	}
}

type tracedTransport struct {
	base     http.RoundTripper
	upstream string
	tracer   trace.Tracer
}

func (t *tracedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := t.tracer.Start(req.Context(), t.upstream+".http", trace.WithAttributes(
		attribute.String("watchtower.upstream", t.upstream),
		semconv.HTTPRequestMethodKey.String(req.Method),
		semconv.ServerAddress(hostname(req.URL.Host)),
		semconv.URLScheme(req.URL.Scheme),
		semconv.URLPath(sanitizePath(req.URL.Path)),
	))
	defer span.End()

	cloned := req.Clone(ctx)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(cloned.Header))

	resp, err := t.base.RoundTrip(cloned)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	if resp.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, http.StatusText(resp.StatusCode))
	}
	return resp, nil
}

func sanitizePath(path string) string {
	if path == "" {
		return "/"
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if pathSegmentVariable(part) {
			parts[i] = ":id"
		}
	}
	out := strings.Join(parts, "/")
	if out == "" || out[0] != '/' {
		return "/" + out
	}
	return out
}

func pathSegmentVariable(part string) bool {
	if len(part) >= 2 && (part[0] == 'v' || part[0] == 'V') {
		allDigits := true
		for _, r := range part[1:] {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return false
		}
	}
	if len(part) > 24 {
		return true
	}
	hasDigit := false
	for _, r := range part {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '-', r == '_', r == '.', r == '~', r == '+':
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			return true
		}
	}
	return hasDigit
}

func hostname(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return host
	}
	return hostport
}
