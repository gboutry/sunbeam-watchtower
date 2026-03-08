// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
)

func TestMiddlewareRecordsTemplatedRouteAndStatus(t *testing.T) {
	tel := &Telemetry{}
	tel.initRegistries()

	router := chi.NewMux()
	router.Use(tel.Middleware)
	router.Get("/api/v1/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/items/42", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if got, want := res.Code, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	count := testutil.ToFloat64(tel.requestCounter.WithLabelValues(http.MethodGet, "/api/v1/items/{id}", "201"))
	if count != 1 {
		t.Fatalf("request counter = %v, want 1", count)
	}
}

func TestCollectorRuntimeConfigDefaults(t *testing.T) {
	enabled, interval := collectorRuntimeConfig(collectorSpec{
		name:        "releases",
		cfg:         config.OTelCollectorConfig{},
		fallback:    2 * time.Minute,
		cacheBacked: true,
	}, nil)
	if !enabled {
		t.Fatal("default-on collector should be enabled")
	}
	if interval != 2*time.Minute {
		t.Fatalf("interval = %v, want %v", interval, 2*time.Minute)
	}
}

func TestCollectorRuntimeConfigLiveCollectorsRequireAllowList(t *testing.T) {
	spec := collectorSpec{
		name:     "reviews",
		cfg:      config.OTelCollectorConfig{},
		fallback: time.Minute,
	}
	enabled, _ := collectorRuntimeConfig(spec, nil)
	if enabled {
		t.Fatal("live collector should be disabled by default")
	}

	enabled, _ = collectorRuntimeConfig(spec, []string{"reviews"})
	if !enabled {
		t.Fatal("live collector should be enabled when allow-listed")
	}
}
