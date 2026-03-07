// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	distro "github.com/gboutry/sunbeam-watchtower/pkg/distro/v1"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

// PackagesDiffRequest describes one package diff workflow.
type PackagesDiffRequest struct {
	Set             string
	Distros         []string
	Releases        []string
	Suites          []string
	Backports       []string
	Merge           bool
	UpstreamRelease string
	BehindUpstream  bool
	OnlyIn          string
	Constraints     string
}

// PackagesDiffResponse contains diff results plus resolved source metadata for rendering.
type PackagesDiffResponse struct {
	Results     []dto.PackageDiffResult
	Sources     []dto.PackageSource
	Merge       bool
	HasUpstream bool
}

// PackagesShowVersionRequest describes one package show-version workflow.
type PackagesShowVersionRequest struct {
	Package         string
	Distros         []string
	Releases        []string
	Backports       []string
	Merge           bool
	UpstreamRelease string
}

// PackagesShowVersionResponse contains one package version result plus resolved source metadata.
type PackagesShowVersionResponse struct {
	Result      dto.PackageDiffResult
	Sources     []dto.PackageSource
	Merge       bool
	HasUpstream bool
}

// PackagesDetailRequest describes one package detail workflow.
type PackagesDetailRequest struct {
	Package   string
	Version   string
	Distros   []string
	Releases  []string
	Suites    []string
	Backports []string
}

// PackagesListRequest describes one package list workflow.
type PackagesListRequest struct {
	Distros    []string
	Releases   []string
	Suites     []string
	Components []string
	Backports  []string
}

// PackagesDscRequest describes one package dsc workflow.
type PackagesDscRequest struct {
	Packages  []string
	Distros   []string
	Releases  []string
	Backports []string
}

// PackagesRdependsRequest describes one reverse-dependency workflow.
type PackagesRdependsRequest struct {
	Package   string
	Distros   []string
	Releases  []string
	Suites    []string
	Backports []string
}

// PackagesExcusesListRequest describes one excuses-list workflow.
type PackagesExcusesListRequest struct {
	Trackers    []string
	Name        string
	Component   string
	Team        string
	FTBFS       bool
	Autopkgtest bool
	BlockedBy   string
	Bugged      bool
	MinAge      int
	MaxAge      int
	Limit       int
	Reverse     bool
}

// PackagesExcusesShowRequest describes one excuses-show workflow.
type PackagesExcusesShowRequest struct {
	Package string
	Tracker string
	Version string
}

// PackagesClientWorkflow exposes reusable client-side package workflows for CLI/TUI/MCP frontends.
type PackagesClientWorkflow struct {
	client      *client.Client
	application *app.App
}

// NewPackagesClientWorkflow creates a client-side packages workflow.
func NewPackagesClientWorkflow(apiClient *client.Client, application *app.App) *PackagesClientWorkflow {
	return &PackagesClientWorkflow{
		client:      apiClient,
		application: application,
	}
}

// Diff compares package versions and resolves source metadata for rendering.
func (w *PackagesClientWorkflow) Diff(ctx context.Context, req PackagesDiffRequest) (*PackagesDiffResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}

	results, err := apiClient.PackagesDiff(ctx, client.PackagesDiffOptions{
		Set:             req.Set,
		Distros:         req.Distros,
		Releases:        req.Releases,
		Suites:          req.Suites,
		Backports:       req.Backports,
		Merge:           req.Merge,
		UpstreamRelease: req.UpstreamRelease,
		BehindUpstream:  req.BehindUpstream,
		OnlyIn:          req.OnlyIn,
		Constraints:     req.Constraints,
	})
	if err != nil {
		return nil, err
	}

	return &PackagesDiffResponse{
		Results:     results,
		Sources:     w.buildSources(req.Distros, req.Releases, req.Suites, req.Backports),
		Merge:       req.Merge,
		HasUpstream: req.UpstreamRelease != "" || req.Constraints != "",
	}, nil
}

// ShowVersion returns version information for one package plus resolved source metadata.
func (w *PackagesClientWorkflow) ShowVersion(ctx context.Context, req PackagesShowVersionRequest) (*PackagesShowVersionResponse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}

	result, err := apiClient.PackagesShow(ctx, req.Package, client.PackagesShowOptions{
		Distros:         req.Distros,
		Releases:        req.Releases,
		Backports:       req.Backports,
		Merge:           req.Merge,
		UpstreamRelease: req.UpstreamRelease,
	})
	if err != nil {
		return nil, err
	}

	return &PackagesShowVersionResponse{
		Result:      *result,
		Sources:     w.buildSources(req.Distros, req.Releases, nil, req.Backports),
		Merge:       req.Merge,
		HasUpstream: req.UpstreamRelease != "",
	}, nil
}

// Detail returns full package metadata.
func (w *PackagesClientWorkflow) Detail(ctx context.Context, req PackagesDetailRequest) (*distro.SourcePackageInfo, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.PackagesDetail(ctx, req.Package, client.PackagesDetailOptions{
		Version:   req.Version,
		Distros:   req.Distros,
		Releases:  req.Releases,
		Suites:    req.Suites,
		Backports: req.Backports,
	})
}

// List lists source packages for the requested distro filters.
func (w *PackagesClientWorkflow) List(ctx context.Context, req PackagesListRequest) ([]distro.SourcePackage, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.PackagesList(ctx, client.PackagesListOptions{
		Distros:    req.Distros,
		Releases:   req.Releases,
		Suites:     req.Suites,
		Components: req.Components,
		Backports:  req.Backports,
	})
}

// Dsc resolves .dsc URLs for source package/version pairs.
func (w *PackagesClientWorkflow) Dsc(ctx context.Context, req PackagesDscRequest) ([]dto.PackageDscResult, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.PackagesDsc(ctx, client.PackagesDscOptions{
		Packages:  req.Packages,
		Distros:   req.Distros,
		Releases:  req.Releases,
		Backports: req.Backports,
	})
}

// Rdepends finds source packages that build-depend on the given package.
func (w *PackagesClientWorkflow) Rdepends(ctx context.Context, req PackagesRdependsRequest) ([]distro.SourcePackageDetail, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.PackagesRdepends(ctx, req.Package, client.PackagesRdependsOptions{
		Distros:   req.Distros,
		Releases:  req.Releases,
		Suites:    req.Suites,
		Backports: req.Backports,
	})
}

// ExcusesList lists package migration excuses.
func (w *PackagesClientWorkflow) ExcusesList(ctx context.Context, req PackagesExcusesListRequest) ([]dto.PackageExcuseSummary, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.PackagesExcusesList(ctx, client.PackagesExcusesListOptions{
		Trackers:    req.Trackers,
		Name:        req.Name,
		Component:   req.Component,
		Team:        req.Team,
		FTBFS:       req.FTBFS,
		Autopkgtest: req.Autopkgtest,
		BlockedBy:   req.BlockedBy,
		Bugged:      req.Bugged,
		MinAge:      req.MinAge,
		MaxAge:      req.MaxAge,
		Limit:       req.Limit,
		Reverse:     req.Reverse,
	})
}

// ExcusesShow returns one package migration excuse.
func (w *PackagesClientWorkflow) ExcusesShow(ctx context.Context, req PackagesExcusesShowRequest) (*dto.PackageExcuse, error) {
	apiClient, err := w.resolveClient()
	if err != nil {
		return nil, err
	}
	return apiClient.PackagesExcusesShow(ctx, req.Package, client.PackagesExcusesShowOptions{
		Tracker: req.Tracker,
		Version: req.Version,
	})
}

func (w *PackagesClientWorkflow) resolveClient() (*client.Client, error) {
	if w.client == nil {
		return nil, errors.New("packages client workflow requires an API client")
	}
	return w.client, nil
}

func (w *PackagesClientWorkflow) buildSources(distros, releases, suites, backports []string) []dto.PackageSource {
	if w.application == nil || w.application.Config == nil {
		return nil
	}
	return w.application.BuildPackageSources(distros, releases, suites, backports)
}
