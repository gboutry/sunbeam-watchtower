// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gboutry/sunbeam-watchtower/internal/config"
)

type collectorSpec struct {
	name      string
	cfg       config.OTelCollectorConfig
	fallback  time.Duration
	defaultOn bool
	refresh   func(context.Context) error
}

func (t *Telemetry) startCollectors(parent context.Context) {
	if t.source == nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	t.cancel = cancel
	for _, spec := range t.collectorSpecs() {
		enabled, interval := collectorRuntimeConfig(spec.cfg, spec.fallback, spec.defaultOn)
		if !enabled {
			continue
		}
		t.wg.Add(1)
		go func(spec collectorSpec, interval time.Duration) {
			defer t.wg.Done()
			t.runCollector(ctx, spec, interval)
		}(spec, interval)
	}
}

func (t *Telemetry) collectorSpecs() []collectorSpec {
	return []collectorSpec{
		{name: "auth", cfg: t.cfg.Metrics.Collectors.Auth, fallback: time.Minute, defaultOn: true, refresh: t.refreshAuth},
		{name: "operations", cfg: t.cfg.Metrics.Collectors.Operations, fallback: 30 * time.Second, defaultOn: true, refresh: t.refreshOperations},
		{name: "projects", cfg: t.cfg.Metrics.Collectors.Projects, fallback: 10 * time.Minute, defaultOn: true, refresh: t.refreshProjects},
		{name: "builds", cfg: t.cfg.Metrics.Collectors.Builds, fallback: 5 * time.Minute, defaultOn: false, refresh: t.refreshBuilds},
		{name: "releases", cfg: t.cfg.Metrics.Collectors.Releases, fallback: 2 * time.Minute, defaultOn: true, refresh: t.refreshReleases},
		{name: "reviews", cfg: t.cfg.Metrics.Collectors.Reviews, fallback: 5 * time.Minute, defaultOn: true, refresh: t.refreshReviews},
		{name: "commits", cfg: t.cfg.Metrics.Collectors.Commits, fallback: 10 * time.Minute, defaultOn: false, refresh: t.refreshCommits},
		{name: "bugs", cfg: t.cfg.Metrics.Collectors.Bugs, fallback: 5 * time.Minute, defaultOn: false, refresh: t.refreshBugs},
		{name: "packages", cfg: t.cfg.Metrics.Collectors.Packages, fallback: 10 * time.Minute, defaultOn: true, refresh: t.refreshPackages},
		{name: "excuses", cfg: t.cfg.Metrics.Collectors.Excuses, fallback: 10 * time.Minute, defaultOn: true, refresh: t.refreshExcuses},
		{name: "cache", cfg: t.cfg.Metrics.Collectors.Cache, fallback: time.Minute, defaultOn: true, refresh: t.refreshCaches},
	}
}

func collectorRuntimeConfig(cfg config.OTelCollectorConfig, fallback time.Duration, defaultOn bool) (bool, time.Duration) {
	interval := fallback
	if cfg.RefreshInterval != "" {
		if parsed, err := time.ParseDuration(cfg.RefreshInterval); err == nil && parsed > 0 {
			interval = parsed
		}
	}
	enabled := defaultOn
	if cfg.Enabled {
		enabled = true
	}
	if !cfg.Enabled && cfg.RefreshInterval != "" && !defaultOn {
		enabled = true
	}
	return enabled, interval
}

func (t *Telemetry) runCollector(ctx context.Context, spec collectorSpec, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		start := time.Now()
		err := spec.refresh(ctx)
		result := "success"
		if err != nil {
			result = "error"
			t.collectorErrors.WithLabelValues(spec.name).Inc()
			t.logger.Warn("telemetry collector refresh failed", "collector", spec.name, "error", err)
		} else {
			t.collectorLastRun.WithLabelValues(spec.name).Set(float64(time.Now().Unix()))
		}
		t.collectorRefresh.WithLabelValues(spec.name, result).Inc()
		t.collectorLatency.WithLabelValues(spec.name, result).Observe(time.Since(start).Seconds())

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (t *Telemetry) refreshAuth(ctx context.Context) error {
	snapshot, err := t.source.AuthSnapshot(ctx)
	if err != nil {
		return err
	}
	t.authAuthenticated.Reset()
	for _, provider := range snapshot.Providers {
		value := 0.0
		if provider.Authenticated {
			value = 1
		}
		t.authAuthenticated.WithLabelValues(provider.Provider).Set(value)
	}
	return nil
}

func (t *Telemetry) refreshOperations(ctx context.Context) error {
	snapshot, err := t.source.OperationSnapshot(ctx)
	if err != nil {
		return err
	}
	t.operationCount.Reset()
	t.operationOldest.Reset()
	for _, metric := range snapshot.Operations {
		t.operationCount.WithLabelValues(metric.Kind, metric.State).Set(float64(metric.Count))
		t.operationOldest.WithLabelValues(metric.Kind, metric.State).Set(metric.OldestAge.Seconds())
	}
	return nil
}

func (t *Telemetry) refreshProjects(ctx context.Context) error {
	snapshot, err := t.source.ProjectSnapshot(ctx)
	if err != nil {
		return err
	}
	t.projectConfigured.Reset()
	t.projectRepoCached.Reset()
	for _, metric := range snapshot.Projects {
		t.projectConfigured.WithLabelValues(metric.Project, metric.Forge, metric.ArtifactType).Set(1)
		if metric.RepoCached {
			t.projectRepoCached.WithLabelValues(metric.Project).Set(1)
		} else {
			t.projectRepoCached.WithLabelValues(metric.Project).Set(0)
		}
	}
	return nil
}

func (t *Telemetry) refreshBuilds(ctx context.Context) error {
	snapshot, err := t.source.BuildSnapshot(ctx)
	if err != nil {
		return err
	}
	t.buildCount.Reset()
	t.buildOldest.Reset()
	for _, metric := range snapshot.Builds {
		t.buildCount.WithLabelValues(metric.Project, metric.ArtifactType, metric.Backend, metric.State).Set(float64(metric.Count))
		t.buildOldest.WithLabelValues(metric.Project, metric.ArtifactType, metric.Backend, metric.State).Set(metric.OldestAge.Seconds())
	}
	return nil
}

func (t *Telemetry) refreshReleases(ctx context.Context) error {
	snapshot, err := t.source.ReleaseSnapshot(ctx)
	if err != nil {
		return err
	}
	t.releasePresent.Reset()
	t.releaseRevision.Reset()
	t.releaseReleased.Reset()
	t.releaseResource.Reset()
	seen := make(map[[6]string]bool)
	for _, target := range snapshot.Targets {
		channel := [6]string{target.Project, target.ArtifactType, target.Artifact, target.Track, target.Risk, target.Branch}
		if !seen[channel] {
			t.releasePresent.WithLabelValues(channel[:]...).Set(1)
			seen[channel] = true
		}
		t.releaseRevision.WithLabelValues(target.Project, target.ArtifactType, target.Artifact, target.Track, target.Risk, target.Branch, target.Architecture).Set(float64(target.Revision))
		if !target.ReleasedAt.IsZero() {
			t.releaseReleased.WithLabelValues(target.Project, target.ArtifactType, target.Artifact, target.Track, target.Risk, target.Branch, target.Architecture).Set(float64(target.ReleasedAt.Unix()))
		}
	}
	for _, resource := range snapshot.Resources {
		t.releaseResource.WithLabelValues(resource.Project, resource.ArtifactType, resource.Artifact, resource.Track, resource.Risk, resource.Branch, resource.Resource).Set(float64(resource.Revision))
	}
	return nil
}

func (t *Telemetry) refreshReviews(ctx context.Context) error {
	snapshot, err := t.source.ReviewSnapshot(ctx)
	if err != nil {
		return err
	}
	t.reviewCount.Reset()
	t.reviewOldest.Reset()
	for _, metric := range snapshot.Reviews {
		t.reviewCount.WithLabelValues(metric.Project, metric.Forge, metric.State).Set(float64(metric.Count))
		t.reviewOldest.WithLabelValues(metric.Project, metric.Forge, metric.State).Set(metric.OldestAge.Seconds())
	}
	return nil
}

func (t *Telemetry) refreshCommits(ctx context.Context) error {
	snapshot, err := t.source.CommitSnapshot(ctx)
	if err != nil {
		return err
	}
	t.commitCount.Reset()
	for _, metric := range snapshot.Commits {
		t.commitCount.WithLabelValues(metric.Project, metric.MergeRequestState, metric.HasBugRef).Set(float64(metric.Count))
	}
	return nil
}

func (t *Telemetry) refreshBugs(ctx context.Context) error {
	snapshot, err := t.source.BugSnapshot(ctx)
	if err != nil {
		return err
	}
	t.bugCount.Reset()
	for _, metric := range snapshot.Bugs {
		t.bugCount.WithLabelValues(metric.Project, metric.Forge, metric.Assigned).Set(float64(metric.Count))
	}
	return nil
}

func (t *Telemetry) refreshPackages(ctx context.Context) error {
	snapshot, err := t.source.PackageSnapshot(ctx)
	if err != nil {
		return err
	}
	t.packageCount.Reset()
	for _, metric := range snapshot.Packages {
		t.packageCount.WithLabelValues(metric.Source, metric.Distro, metric.Release, metric.Component).Set(float64(metric.Count))
	}
	return nil
}

func (t *Telemetry) refreshExcuses(ctx context.Context) error {
	snapshot, err := t.source.ExcusesSnapshot(ctx)
	if err != nil {
		return err
	}
	t.excusesCount.Reset()
	for _, metric := range snapshot.Trackers {
		t.excusesCount.WithLabelValues(metric.Tracker).Set(float64(metric.Count))
	}
	return nil
}

func (t *Telemetry) refreshCaches(ctx context.Context) error {
	snapshot, err := t.source.CacheSnapshot(ctx)
	if err != nil {
		return err
	}
	t.cacheEntries.Reset()
	t.cacheLastUpdated.Reset()
	for _, metric := range snapshot.Caches {
		t.cacheEntries.WithLabelValues(metric.Kind, metric.Scope).Set(float64(metric.Entries))
		if !metric.LastUpdated.IsZero() {
			t.cacheLastUpdated.WithLabelValues(metric.Kind, metric.Scope).Set(float64(metric.LastUpdated.Unix()))
		}
	}
	return nil
}

var _ prometheus.Collector
