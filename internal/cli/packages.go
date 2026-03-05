// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	distro "github.com/gboutry/sunbeam-watchtower/internal/pkg/distro/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
	pkg "github.com/gboutry/sunbeam-watchtower/internal/service/package"
	"github.com/spf13/cobra"
)

func newPackagesCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Compare source package versions across distros",
	}
	cmd.AddCommand(
		newPackagesDiffCmd(opts),
		newPackagesShowCmd(opts),
		newPackagesListCmd(opts),
		newPackagesDscCmd(opts),
		newPackagesRdependsCmd(opts),
	)
	return cmd
}

func newPackagesDiffCmd(opts *Options) *cobra.Command {
	var distros, backports []string
	var suites, components []string
	var merge bool
	var release string
	var behindUpstream bool
	var onlyIn string
	var constraints string

	cmd := &cobra.Command{
		Use:   "diff <set>",
		Short: "Compare package versions across distros",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			setName := args[0]
			packages, ok := opts.Config.Packages.Sets[setName]
			if !ok {
				return fmt.Errorf("unknown package set %q", setName)
			}

			cache, err := buildDistroCache(opts)
			if err != nil {
				return err
			}
			defer cache.Close()

			sources := buildPackageSources(opts, distros, backports)
			if len(sources) == 0 {
				return fmt.Errorf("no distros configured")
			}

			svc := pkg.NewService(cache, opts.Logger)
			results, err := svc.Diff(cmd.Context(), pkg.DiffOpts{
				Packages:   packages,
				Sources:    sources,
				Suites:     suites,
				Components: components,
			})
			if err != nil {
				return err
			}

			// --constraints: merge upper-constraints packages into the diff.
			if constraints != "" {
				provider, pErr := buildUpstreamProvider(opts)
				if pErr != nil {
					return fmt.Errorf("upstream provider: %w", pErr)
				}
				if provider == nil {
					return fmt.Errorf("--constraints requires upstream provider configuration")
				}
				constraintMap, cErr := provider.GetConstraints(cmd.Context(), constraints)
				if cErr != nil {
					return fmt.Errorf("fetching constraints for %s: %w", constraints, cErr)
				}

				// Build set of already-present packages.
				existing := make(map[string]bool, len(results))
				for _, r := range results {
					existing[r.Package] = true
				}

				// Add constraint packages that aren't in the set.
				for pypiName, ver := range constraintMap {
					pkgName := provider.MapPackageName(pypiName, port.DeliverableOther)
					if existing[pkgName] {
						continue
					}
					// Re-query the cache for this package across sources.
					extra, qErr := svc.Show(cmd.Context(), pkgName, sources)
					if qErr != nil {
						continue
					}
					extra.Upstream = ver
					results = append(results, *extra)
					existing[pkgName] = true
				}

				// Sort after additions.
				sort.Slice(results, func(i, j int) bool {
					return results[i].Package < results[j].Package
				})
			}

			hasUpstream := false
			effectiveRelease := release
			if effectiveRelease == "" && constraints != "" {
				effectiveRelease = constraints
			}
			if effectiveRelease != "" {
				if err := annotateUpstream(cmd.Context(), opts, results, effectiveRelease); err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: upstream lookup: %v\n", err)
				} else {
					hasUpstream = true
				}
			}

			// --behind-upstream: keep only packages where distro < upstream.
			if behindUpstream {
				if !hasUpstream {
					return fmt.Errorf("--behind-upstream requires --release or --constraints")
				}
				results = filterBehindUpstream(results, sources)
			}

			// --only-in: keep only packages present in the named source.
			if onlyIn != "" {
				results = filterOnlyIn(results, onlyIn)
			}

			return renderDiffResults(opts.Out, opts.Output, results, sources, merge, hasUpstream)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")
	cmd.Flags().StringSliceVar(&suites, "suite", nil, "filter by suite")
	cmd.Flags().StringSliceVar(&components, "component", nil, "filter by component")
	cmd.Flags().BoolVar(&merge, "merge", false, "merge suites per source into a single column showing highest version")
	cmd.Flags().StringVar(&release, "release", "", "upstream release to compare against (e.g. 2025.1)")
	cmd.Flags().BoolVar(&behindUpstream, "behind-upstream", false, "show only packages where distro version is behind upstream")
	cmd.Flags().StringVar(&onlyIn, "only-in", "", "show only packages present in the named source")
	cmd.Flags().StringVar(&constraints, "constraints", "", "include upper-constraints packages for the given release")

	return cmd
}

func newPackagesShowCmd(opts *Options) *cobra.Command {
	var distros, backports []string
	var merge bool
	var release string

	cmd := &cobra.Command{
		Use:   "show <package>",
		Short: "Show all versions of a package across distros",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgName := args[0]

			cache, err := buildDistroCache(opts)
			if err != nil {
				return err
			}
			defer cache.Close()

			sources := buildPackageSources(opts, distros, backports)
			svc := pkg.NewService(cache, opts.Logger)

			result, err := svc.Show(cmd.Context(), pkgName, sources)
			if err != nil {
				return err
			}

			results := []pkg.DiffResult{*result}
			hasUpstream := false
			if release != "" {
				if err := annotateUpstream(cmd.Context(), opts, results, release); err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: upstream lookup: %v\n", err)
				} else {
					hasUpstream = true
				}
			}

			return renderDiffResults(opts.Out, opts.Output, results, sources, merge, hasUpstream)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")
	cmd.Flags().BoolVar(&merge, "merge", false, "merge suites per source into a single column showing highest version")
	cmd.Flags().StringVar(&release, "release", "", "upstream release to compare against (e.g. 2025.1)")

	return cmd
}

func newPackagesListCmd(opts *Options) *cobra.Command {
	var distro string
	var suites, components []string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages in a distro",
		RunE: func(cmd *cobra.Command, args []string) error {
			if distro == "" {
				return fmt.Errorf("--distro is required")
			}

			cache, err := buildDistroCache(opts)
			if err != nil {
				return err
			}
			defer cache.Close()

			svc := pkg.NewService(cache, opts.Logger)
			pkgs, err := svc.List(cmd.Context(), distro, port.QueryOpts{
				Suites:     suites,
				Components: components,
			})
			if err != nil {
				return err
			}

			return renderSourcePackages(opts.Out, opts.Output, pkgs)
		},
	}

	cmd.Flags().StringVar(&distro, "distro", "", "distro to list from (required)")
	cmd.Flags().StringSliceVar(&suites, "suite", nil, "filter by suite")
	cmd.Flags().StringSliceVar(&components, "component", nil, "filter by component")

	return cmd
}

// annotateUpstream populates the Upstream field on each DiffResult by querying
// the configured upstream provider for deliverable versions and constraints.
func annotateUpstream(ctx context.Context, opts *Options, results []pkg.DiffResult, release string) error {
	provider, err := buildUpstreamProvider(opts)
	if err != nil {
		return err
	}
	if provider == nil {
		return fmt.Errorf("no upstream provider configured")
	}

	deliverables, err := provider.ListDeliverables(ctx, release)
	if err != nil {
		return fmt.Errorf("listing deliverables: %w", err)
	}

	// Build mapping: distro package name → upstream version.
	upstreamVersions := make(map[string]string, len(deliverables))
	for _, d := range deliverables {
		pkgName := provider.MapPackageName(d.Name, d.Type)
		if d.Version != "" {
			upstreamVersions[pkgName] = d.Version
		}
	}

	// Also try constraints for packages not found in deliverables.
	constraints, cErr := provider.GetConstraints(ctx, release)
	if cErr != nil {
		opts.Logger.Debug("upstream constraints unavailable", "error", cErr)
	}

	for i := range results {
		if v, ok := upstreamVersions[results[i].Package]; ok {
			results[i].Upstream = v
		} else if constraints != nil {
			// Try constraint mapping: constraints use PyPI names, so try the
			// package name directly and common prefixes.
			name := results[i].Package
			if v, ok := constraints[name]; ok {
				results[i].Upstream = v
			} else {
				// Try mapping constraint names through the provider.
				for cName, cVer := range constraints {
					mapped := provider.MapPackageName(cName, port.DeliverableOther)
					if mapped == name {
						results[i].Upstream = cVer
						break
					}
				}
			}
		}
	}
	return nil
}

// filterBehindUpstream keeps only results where the highest distro version
// across all sources is strictly less than the upstream version.
func filterBehindUpstream(results []pkg.DiffResult, sources []pkg.ProjectSource) []pkg.DiffResult {
	var filtered []pkg.DiffResult
	for _, r := range results {
		if r.Upstream == "" {
			continue
		}
		// Find highest distro version across all sources.
		var allVersions []distro.SourcePackage
		for _, src := range sources {
			allVersions = append(allVersions, r.Versions[src.Name]...)
		}
		if len(allVersions) == 0 {
			// Package only exists upstream → it's "behind".
			filtered = append(filtered, r)
			continue
		}
		highest := distro.PickHighest(allVersions)
		// Strip epoch and debian revision for upstream comparison.
		distroVer := stripDebianVersion(highest.Version)
		if distroVer < r.Upstream {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// stripDebianVersion removes the epoch prefix and Debian revision suffix from
// a Debian version string, leaving just the upstream version component for
// rough comparison with upstream versions.
func stripDebianVersion(v string) string {
	// Remove epoch (everything before first ':').
	if idx := strings.Index(v, ":"); idx >= 0 {
		v = v[idx+1:]
	}
	// Remove Debian revision (everything after last '-').
	if idx := strings.LastIndex(v, "-"); idx >= 0 {
		v = v[:idx]
	}
	return v
}

// filterOnlyIn keeps only results where the package has versions in the named source.
func filterOnlyIn(results []pkg.DiffResult, sourceName string) []pkg.DiffResult {
	var filtered []pkg.DiffResult
	for _, r := range results {
		if len(r.Versions[sourceName]) > 0 {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// buildPackageSources resolves --distro and --backport flags against config to produce source entries.
// Backports are nested under their parent distro. The backport filter selects which backports to include:
//   - empty/nil: include all backports
//   - ["none"]: skip all backports
//   - ["gazpacho", "flamingo"]: include only those backports
func buildPackageSources(opts *Options, distros, backports []string) []pkg.ProjectSource {
	cfg := opts.Config.Packages
	var sources []pkg.ProjectSource

	bpFilter := make(map[string]bool, len(backports))
	for _, bp := range backports {
		bpFilter[bp] = true
	}
	skipAllBackports := bpFilter["none"]
	filterBackports := len(bpFilter) > 0 && !skipAllBackports

	// Resolve distros.
	distroNames := distros
	if len(distroNames) == 0 {
		for name := range cfg.Distros {
			distroNames = append(distroNames, name)
		}
	}

	for _, name := range distroNames {
		if name == "none" {
			continue
		}
		d, ok := cfg.Distros[name]
		if !ok {
			opts.Logger.Warn("unknown distro in config, skipping", "distro", name)
			continue
		}
		var entries []port.SourceEntry
		for _, suite := range d.Suites {
			for _, comp := range d.Components {
				entries = append(entries, port.SourceEntry{
					Mirror:    d.Mirror,
					Suite:     suite,
					Component: comp,
				})
			}
		}
		sources = append(sources, pkg.ProjectSource{
			Name:    name,
			Entries: entries,
		})

		if skipAllBackports {
			continue
		}

		// Include backports belonging to this distro.
		for bpName, bp := range d.Backports {
			if filterBackports && !bpFilter[bpName] {
				continue
			}
			qualifiedName := name + "/" + bpName
			var bpEntries []port.SourceEntry
			for _, src := range bp.Sources {
				for _, suite := range src.Suites {
					for _, comp := range src.Components {
						bpEntries = append(bpEntries, port.SourceEntry{
							Mirror:    src.Mirror,
							Suite:     suite,
							Component: comp,
						})
					}
				}
			}
			sources = append(sources, pkg.ProjectSource{
				Name:    qualifiedName,
				Entries: bpEntries,
			})
		}
	}

	return sources
}

// renderDiffResults writes diff results in the requested format.
func renderDiffResults(w io.Writer, format string, results []pkg.DiffResult, sources []pkg.ProjectSource, merge bool, hasUpstream bool) error {
	switch format {
	case "json":
		return renderJSON(w, results)
	case "yaml":
		return renderYAML(w, results)
	default:
		if merge {
			return renderMergedDiffTable(w, results, sources, hasUpstream)
		}
		return renderDiffTable(w, results, sources, hasUpstream)
	}
}

// renderDiffTable renders a table with dynamic columns based on queried sources.
func renderDiffTable(w io.Writer, results []pkg.DiffResult, sources []pkg.ProjectSource, hasUpstream bool) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No results found.")
		return nil
	}

	type column struct {
		source string
		suite  string
		header string
	}

	var columns []column
	for _, src := range sources {
		suites := make(map[string]bool)
		for _, r := range results {
			for _, p := range r.Versions[src.Name] {
				suites[p.Suite] = true
			}
		}
		if len(suites) == 0 {
			columns = append(columns, column{
				source: src.Name,
				suite:  "",
				header: strings.ToUpper(src.Name),
			})
			continue
		}
		for suite := range suites {
			columns = append(columns, column{
				source: src.Name,
				suite:  suite,
				header: strings.ToUpper(src.Name) + ":" + suite,
			})
		}
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	header := "PACKAGE"
	for _, col := range columns {
		header += "\t" + col.header
	}
	if hasUpstream {
		header += "\t" + "UPSTREAM"
	}
	fmt.Fprintln(tw, header)

	for _, r := range results {
		row := r.Package
		for _, col := range columns {
			version := "-"
			for _, p := range r.Versions[col.source] {
				if col.suite == "" || p.Suite == col.suite {
					version = p.Version
					break
				}
			}
			row += "\t" + version
		}
		if hasUpstream {
			up := r.Upstream
			if up == "" {
				up = "—"
			}
			row += "\t" + up
		}
		fmt.Fprintln(tw, row)
	}

	return tw.Flush()
}

// suiteMarker returns a short origin marker for a suite name.
func suiteMarker(suite string) string {
	s := strings.ToLower(suite)
	switch {
	case strings.Contains(s, "updates"):
		return "U"
	case strings.Contains(s, "proposed"):
		return "P"
	case strings.Contains(s, "security"):
		return "S"
	case strings.Contains(s, "staging"):
		return "S"
	case strings.Contains(s, "unstable"):
		return "U"
	case strings.Contains(s, "experimental"):
		return "E"
	default:
		return "R"
	}
}

// renderMergedDiffTable renders a table with one column per source, showing the
// highest version with origin markers indicating which suites carry it.
func renderMergedDiffTable(w io.Writer, results []pkg.DiffResult, sources []pkg.ProjectSource, hasUpstream bool) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No results found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	header := "PACKAGE"
	for _, src := range sources {
		header += "\t" + strings.ToUpper(src.Name)
	}
	if hasUpstream {
		header += "\t" + "UPSTREAM"
	}
	fmt.Fprintln(tw, header)

	for _, r := range results {
		row := r.Package
		for _, src := range sources {
			versions := r.Versions[src.Name]
			if len(versions) == 0 {
				row += "\t" + "—"
				continue
			}

			highest := distro.PickHighest(versions)
			var markers []string
			for _, p := range versions {
				if distro.CompareVersions(p.Version, highest.Version) == 0 {
					markers = append(markers, suiteMarker(p.Suite))
				}
			}
			sort.Strings(markers)
			// Deduplicate sorted markers.
			deduped := markers[:0]
			for i, m := range markers {
				if i == 0 || m != markers[i-1] {
					deduped = append(deduped, m)
				}
			}
			row += "\t" + highest.Version + " (" + strings.Join(deduped, ",") + ")"
		}
		if hasUpstream {
			up := r.Upstream
			if up == "" {
				up = "—"
			}
			row += "\t" + up
		}
		fmt.Fprintln(tw, row)
	}

	return tw.Flush()
}

// renderSourcePackages writes source packages in the requested format.
func renderSourcePackages(w io.Writer, format string, pkgs []distro.SourcePackage) error {
	switch format {
	case "json":
		return renderJSON(w, pkgs)
	case "yaml":
		return renderYAML(w, pkgs)
	default:
		return renderSourcePackagesTable(w, pkgs)
	}
}

func renderSourcePackagesTable(w io.Writer, pkgs []distro.SourcePackage) error {
	if len(pkgs) == 0 {
		fmt.Fprintln(w, "No packages found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PACKAGE\tVERSION\tSUITE\tCOMPONENT")
	for _, p := range pkgs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Package, p.Version, p.Suite, p.Component)
	}
	return tw.Flush()
}

// renderCacheStatus writes cache status in the requested format.
func renderCacheStatus(w io.Writer, format string, statuses []port.CacheStatus) error {
	switch format {
	case "json":
		return renderJSON(w, statuses)
	case "yaml":
		return renderYAML(w, statuses)
	default:
		return renderCacheStatusTable(w, statuses)
	}
}

func renderCacheStatusTable(w io.Writer, statuses []port.CacheStatus) error {
	if len(statuses) == 0 {
		fmt.Fprintln(w, "No cached sources found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SOURCE\tPACKAGES\tLAST UPDATED\tSIZE")
	for _, s := range statuses {
		updated := "-"
		if !s.LastUpdated.IsZero() {
			updated = s.LastUpdated.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			s.Name,
			s.EntryCount,
			updated,
			formatBytes(s.DiskSize),
		)
	}
	return tw.Flush()
}

// formatBytes formats a byte count into a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func newPackagesDscCmd(opts *Options) *cobra.Command {
	var distros, backports []string

	cmd := &cobra.Command{
		Use:   "dsc <pkg> <version> [<pkg> <version> ...]",
		Short: "Look up .dsc file URLs for source package/version pairs",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || len(args)%2 != 0 {
				return fmt.Errorf("arguments must be <package> <version> pairs")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cache, err := buildDistroCache(opts)
			if err != nil {
				return err
			}
			defer cache.Close()

			sources := buildPackageSources(opts, distros, backports)
			if len(sources) == 0 {
				return fmt.Errorf("no distros configured")
			}

			var pairs []pkg.PackageVersionPair
			for i := 0; i < len(args); i += 2 {
				pairs = append(pairs, pkg.PackageVersionPair{
					Package: args[i],
					Version: args[i+1],
				})
			}

			svc := pkg.NewService(cache, opts.Logger)
			results, err := svc.FindDsc(cmd.Context(), pairs, sources)
			if err != nil {
				return err
			}

			return renderDscResults(opts.Out, opts.Output, results)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")

	return cmd
}

// renderDscResults writes dsc lookup results in the requested format.
func renderDscResults(w io.Writer, format string, results []pkg.DscResult) error {
	switch format {
	case "json":
		return renderJSON(w, results)
	case "yaml":
		return renderYAML(w, results)
	default:
		return renderDscTable(w, results)
	}
}

func renderDscTable(w io.Writer, results []pkg.DscResult) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No results.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PACKAGE\tVERSION\tDSC URL")
	for _, r := range results {
		if len(r.URLs) == 0 {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Package, r.Version, "(not found)")
			continue
		}
		for i, u := range r.URLs {
			if i == 0 {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Package, r.Version, u)
			} else {
				fmt.Fprintf(tw, "\t\t%s\n", u)
			}
		}
	}
	return tw.Flush()
}

func newPackagesRdependsCmd(opts *Options) *cobra.Command {
	var distros, backports []string
	var suites []string

	cmd := &cobra.Command{
		Use:   "rdepends <package>",
		Short: "Find source packages that build-depend on a given package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgName := args[0]

			cache, err := buildDistroCache(opts)
			if err != nil {
				return err
			}
			defer cache.Close()

			sources := buildPackageSources(opts, distros, backports)
			if len(sources) == 0 {
				return fmt.Errorf("no distros configured")
			}

			// Collect the set of backport source names for suite annotation.
			backportNames := make(map[string]bool)
			for distroName, d := range opts.Config.Packages.Distros {
				for bpName := range d.Backports {
					backportNames[distroName+"/"+bpName] = true
				}
			}

			svc := pkg.NewService(cache, opts.Logger)
			queryOpts := port.QueryOpts{Suites: suites}

			// Query each source individually to tag backport results.
			var results []distro.SourcePackageDetail
			for _, src := range sources {
				srcResults, err := svc.ReverseDepends(cmd.Context(), pkgName, []pkg.ProjectSource{src}, queryOpts)
				if err != nil {
					return err
				}
				if backportNames[src.Name] {
					// Extract backport name (after the "distro/" prefix).
					bpLabel := src.Name
					if idx := strings.Index(src.Name, "/"); idx >= 0 {
						bpLabel = src.Name[idx+1:]
					}
					for i := range srcResults {
						srcResults[i].Suite = srcResults[i].Suite + "/" + bpLabel
					}
				}
				results = append(results, srcResults...)
			}

			if len(results) == 0 {
				// Check whether the queried name exists as a source package.
				found := false
				for _, src := range sources {
					srcPkgs, qErr := cache.Query(cmd.Context(), src.Name, port.QueryOpts{
						Packages: []string{pkgName},
						Suites:   suites,
					})
					if qErr == nil && len(srcPkgs) > 0 {
						found = true
						break
					}
				}
				if !found {
					fmt.Fprintf(opts.Out, "Warning: %q was not found as a source package; it may be a binary package name.\n", pkgName)
					fmt.Fprintln(opts.Out, "Hint: rdepends searches Build-Depends which reference source package names.")
				}
			}

			return renderSourcePackageDetails(opts.Out, opts.Output, results)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")
	cmd.Flags().StringSliceVar(&suites, "suite", nil, "filter by suite")

	return cmd
}

// renderSourcePackageDetails writes SourcePackageDetail entries in the requested format.
func renderSourcePackageDetails(w io.Writer, format string, pkgs []distro.SourcePackageDetail) error {
	switch format {
	case "json":
		return renderJSON(w, pkgs)
	case "yaml":
		return renderYAML(w, pkgs)
	default:
		return renderSourcePackageDetailsTable(w, pkgs)
	}
}

func renderSourcePackageDetailsTable(w io.Writer, pkgs []distro.SourcePackageDetail) error {
	if len(pkgs) == 0 {
		fmt.Fprintln(w, "No reverse dependencies found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PACKAGE\tVERSION\tSUITE\tCOMPONENT")
	for _, p := range pkgs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Package, p.Version, p.Suite, p.Component)
	}
	return tw.Flush()
}
