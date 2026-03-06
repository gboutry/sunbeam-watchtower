// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	distro "github.com/gboutry/sunbeam-watchtower/pkg/distro/v1"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
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
	var distros, releases, backports []string
	var suites, components []string
	var merge bool
	var upstreamRelease string
	var behindUpstream bool
	var onlyIn string
	var constraints string

	cmd := &cobra.Command{
		Use:   "diff <set>",
		Short: "Compare package versions across distros",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			setName := args[0]
			ctx := cmd.Context()

			results, err := opts.Client.PackagesDiff(ctx, client.PackagesDiffOptions{
				Set:             setName,
				Distros:         distros,
				Releases:        releases,
				Suites:          suites,
				Backports:       backports,
				Merge:           merge,
				UpstreamRelease: upstreamRelease,
				BehindUpstream:  behindUpstream,
				OnlyIn:          onlyIn,
				Constraints:     constraints,
			})
			if err != nil {
				return err
			}

			sources := opts.App.BuildPackageSources(distros, releases, suites, backports)
			hasUpstream := upstreamRelease != "" || constraints != ""

			return renderDiffResults(opts.Out, opts.Output, results, sources, merge, hasUpstream)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&releases, "release", nil, "distro releases to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")
	cmd.Flags().StringSliceVar(&suites, "suite", nil, "filter by suite type within release (release, updates, proposed)")
	cmd.Flags().StringSliceVar(&components, "component", nil, "filter by component")
	cmd.Flags().BoolVar(&merge, "merge", false, "merge suites per source into a single column showing highest version")
	cmd.Flags().StringVar(&upstreamRelease, "upstream-release", "", "upstream release to compare against (e.g. 2025.1)")
	cmd.Flags().BoolVar(&behindUpstream, "behind-upstream", false, "show only packages where distro version is behind upstream")
	cmd.Flags().StringVar(&onlyIn, "only-in", "", "show only packages present in the named source")
	cmd.Flags().StringVar(&constraints, "constraints", "", "include upper-constraints packages for the given release")

	return cmd
}

func newPackagesShowCmd(opts *Options) *cobra.Command {
	var distros, releases, backports []string
	var merge bool
	var upstreamRelease string

	cmd := &cobra.Command{
		Use:   "show-version <package>",
		Short: "Show all versions of a package across distros",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgName := args[0]
			ctx := cmd.Context()

			result, err := opts.Client.PackagesShow(ctx, pkgName, client.PackagesShowOptions{
				Distros:         distros,
				Releases:        releases,
				Backports:       backports,
				Merge:           merge,
				UpstreamRelease: upstreamRelease,
			})
			if err != nil {
				return err
			}

			sources := opts.App.BuildPackageSources(distros, releases, nil, backports)
			hasUpstream := upstreamRelease != ""

			return renderDiffResults(opts.Out, opts.Output, []dto.PackageDiffResult{*result}, sources, merge, hasUpstream)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&releases, "release", nil, "distro releases to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")
	cmd.Flags().BoolVar(&merge, "merge", false, "merge suites per source into a single column showing highest version")
	cmd.Flags().StringVar(&upstreamRelease, "upstream-release", "", "upstream release to compare against (e.g. 2025.1)")

	return cmd
}

func newPackagesListCmd(opts *Options) *cobra.Command {
	var distros, releases, backports []string
	var suites, components []string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages in a distro",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			result, err := opts.Client.PackagesList(ctx, client.PackagesListOptions{
				Distros:    distros,
				Releases:   releases,
				Suites:     suites,
				Components: components,
				Backports:  backports,
			})
			if err != nil {
				return err
			}

			return renderSourcePackages(opts.Out, opts.Output, result)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&releases, "release", nil, "distro releases to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")
	cmd.Flags().StringSliceVar(&suites, "suite", nil, "filter by suite type within release (release, updates, proposed)")
	cmd.Flags().StringSliceVar(&components, "component", nil, "filter by component")

	return cmd
}

// renderDiffResults writes diff results in the requested format.
func renderDiffResults(w io.Writer, format string, results []dto.PackageDiffResult, sources []dto.PackageSource, merge bool, hasUpstream bool) error {
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
func renderDiffTable(w io.Writer, results []dto.PackageDiffResult, sources []dto.PackageSource, hasUpstream bool) error {
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
func renderMergedDiffTable(w io.Writer, results []dto.PackageDiffResult, sources []dto.PackageSource, hasUpstream bool) error {
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

func renderCacheStatusTable(w io.Writer, statuses []dto.CacheStatus) error {
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
	var distros, releases, backports []string

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
			ctx := cmd.Context()

			var packages []string
			for i := 0; i < len(args); i += 2 {
				packages = append(packages, args[i]+"="+args[i+1])
			}

			results, err := opts.Client.PackagesDsc(ctx, client.PackagesDscOptions{
				Packages:  packages,
				Distros:   distros,
				Releases:  releases,
				Backports: backports,
			})
			if err != nil {
				return err
			}

			return renderDscResults(opts.Out, opts.Output, results)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&releases, "release", nil, "distro releases to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")

	return cmd
}

// renderDscResults writes dsc lookup results in the requested format.
func renderDscResults(w io.Writer, format string, results []dto.PackageDscResult) error {
	switch format {
	case "json":
		return renderJSON(w, results)
	case "yaml":
		return renderYAML(w, results)
	default:
		return renderDscTable(w, results)
	}
}

func renderDscTable(w io.Writer, results []dto.PackageDscResult) error {
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
	var distros, releases, backports []string
	var suites []string

	cmd := &cobra.Command{
		Use:   "rdepends <package>",
		Short: "Find source packages that build-depend on a given package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgName := args[0]
			ctx := cmd.Context()

			results, err := opts.Client.PackagesRdepends(ctx, pkgName, client.PackagesRdependsOptions{
				Distros:   distros,
				Releases:  releases,
				Suites:    suites,
				Backports: backports,
			})
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintf(opts.Out, "Warning: %q was not found as a source package; it may be a binary package name.\n", pkgName)
				fmt.Fprintln(opts.Out, "Hint: rdepends searches Build-Depends which reference source package names.")
			}

			return renderSourcePackageDetails(opts.Out, opts.Output, results)
		},
	}

	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to query (default: all configured)")
	cmd.Flags().StringSliceVar(&releases, "release", nil, "distro releases to query (default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", []string{"none"}, "backports to include (default: none; pass names to include)")
	cmd.Flags().StringSliceVar(&suites, "suite", nil, "filter by suite type within release (release, updates, proposed)")

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
