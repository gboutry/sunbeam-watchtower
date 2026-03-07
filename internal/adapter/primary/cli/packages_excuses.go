package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	"github.com/spf13/cobra"
)

func newPackagesExcusesCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "excuses",
		Short: "Inspect package migration excuses",
	}
	cmd.AddCommand(
		newPackagesExcusesListCmd(opts),
		newPackagesExcusesShowCmd(opts),
	)
	return cmd
}

func newPackagesExcusesListCmd(opts *Options) *cobra.Command {
	var trackers []string
	var name, component, team, blockedBy string
	var ftbfs, autopkgtest, bugged, reverse bool
	var minAge, maxAge, limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List package migration excuses",
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := frontend.NewPackagesClientWorkflow(opts.Client, opts.App).ExcusesList(cmd.Context(), frontend.PackagesExcusesListRequest{
				Trackers:    trackers,
				Name:        name,
				Component:   component,
				Team:        team,
				FTBFS:       ftbfs,
				Autopkgtest: autopkgtest,
				BlockedBy:   blockedBy,
				Bugged:      bugged,
				MinAge:      minAge,
				MaxAge:      maxAge,
				Limit:       limit,
				Reverse:     reverse,
			})
			if err != nil {
				return err
			}
			return renderPackageExcuseSummaries(opts.Out, opts.Output, results)
		},
	}

	cmd.Flags().StringSliceVar(&trackers, "tracker", nil, "excuses trackers to query (default: configured default tracker)")
	cmd.Flags().StringVar(&name, "name", "", "case-insensitive regex to filter source package names")
	cmd.Flags().StringVar(&component, "component", "", "archive component to filter on")
	cmd.Flags().StringVar(&team, "team", "", "team to filter on")
	cmd.Flags().BoolVar(&ftbfs, "ftbfs", false, "show only FTBFS excuses")
	cmd.Flags().BoolVar(&autopkgtest, "autopkgtest", false, "show only autopkgtest-related excuses")
	cmd.Flags().StringVar(&blockedBy, "blocked-by", "", "show only excuses blocked by the given package")
	cmd.Flags().BoolVar(&bugged, "bugged", false, "show only excuses with an attached bug reference")
	cmd.Flags().IntVar(&minAge, "min-age", 0, "only include excuses at least this many days old")
	cmd.Flags().IntVar(&maxAge, "max-age", 0, "only include excuses no older than this many days")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit the number of results")
	cmd.Flags().BoolVar(&reverse, "reverse", false, "show older excuses first")
	return cmd
}

func newPackagesExcusesShowCmd(opts *Options) *cobra.Command {
	var tracker, version string

	cmd := &cobra.Command{
		Use:   "show <package>",
		Short: "Show a detailed package migration excuse",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := frontend.NewPackagesClientWorkflow(opts.Client, opts.App).ExcusesShow(cmd.Context(), frontend.PackagesExcusesShowRequest{
				Package: args[0],
				Tracker: tracker,
				Version: version,
			})
			if err != nil {
				return err
			}
			return renderPackageExcuse(opts.Out, opts.Output, result)
		},
	}

	cmd.Flags().StringVar(&tracker, "tracker", "", "excuses tracker to query (default: configured default tracker)")
	cmd.Flags().StringVar(&version, "version", "", "exact Debian version string")
	return cmd
}

func renderPackageExcuseSummaries(w io.Writer, format string, results []dto.PackageExcuseSummary) error {
	switch format {
	case "json":
		return renderJSON(w, results)
	case "yaml":
		return renderYAML(w, results)
	default:
		return renderPackageExcuseSummariesTable(w, results)
	}
}

func renderPackageExcuseSummariesTable(w io.Writer, results []dto.PackageExcuseSummary) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No package excuses found.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DAYS\tTRACKER\tPACKAGE\tVERSION\tCOMPONENT\tTEAM\tPRIMARY_REASON\tBLOCKED_BY\tBUG")
	for _, result := range results {
		blockedBy := strings.Join(result.BlockedBy, ", ")
		if len(blockedBy) > 32 {
			blockedBy = blockedBy[:29] + "..."
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			result.AgeDays,
			result.Tracker,
			result.Package,
			result.Version,
			result.Component,
			result.Team,
			result.PrimaryReason,
			blockedBy,
			result.Bug,
		)
	}
	return tw.Flush()
}

func renderPackageExcuse(w io.Writer, format string, excuse *dto.PackageExcuse) error {
	switch format {
	case "json":
		return renderJSON(w, excuse)
	case "yaml":
		return renderYAML(w, excuse)
	default:
		return renderPackageExcuseTable(w, excuse)
	}
}

func renderPackageExcuseTable(w io.Writer, excuse *dto.PackageExcuse) error {
	fmt.Fprintf(w, "Tracker:       %s\n", excuse.Tracker)
	fmt.Fprintf(w, "Package:       %s\n", excuse.Package)
	fmt.Fprintf(w, "Item:          %s\n", excuse.ItemName)
	fmt.Fprintf(w, "Version:       %s\n", excuse.Version)
	if excuse.OldVersion != "" {
		fmt.Fprintf(w, "Old Version:   %s\n", excuse.OldVersion)
	}
	if excuse.Component != "" {
		fmt.Fprintf(w, "Component:     %s\n", excuse.Component)
	}
	fmt.Fprintf(w, "Candidate:     %t\n", excuse.Candidate)
	if excuse.Verdict != "" {
		fmt.Fprintf(w, "Verdict:       %s\n", excuse.Verdict)
	}
	fmt.Fprintf(w, "Age (days):    %d\n", excuse.AgeDays)
	fmt.Fprintf(w, "FTBFS:         %t\n", excuse.FTBFS)
	if excuse.Team != "" {
		fmt.Fprintf(w, "Team:          %s\n", excuse.Team)
	}
	if excuse.Maintainer != "" {
		fmt.Fprintf(w, "Maintainer:    %s\n", excuse.Maintainer)
	}
	if excuse.Bug != "" {
		fmt.Fprintf(w, "Bug:           %s\n", excuse.Bug)
	}
	if excuse.PrimaryReason != "" {
		fmt.Fprintf(w, "Primary:       %s\n", excuse.PrimaryReason)
	}
	if len(excuse.BlockedBy) > 0 {
		fmt.Fprintf(w, "Blocked By:    %s\n", strings.Join(excuse.BlockedBy, ", "))
	}
	if excuse.BlocksCount > 0 {
		fmt.Fprintf(w, "Blocks:        %d\n", excuse.BlocksCount)
	}

	if len(excuse.Reasons) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Reasons:")
		for _, reason := range excuse.Reasons {
			fmt.Fprintf(w, "  - %s", reason.Code)
			if reason.Message != "" && reason.Message != reason.Code {
				fmt.Fprintf(w, ": %s", reason.Message)
			}
			fmt.Fprintln(w)
		}
	}

	if len(excuse.Dependencies) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Dependencies:")
		for _, dep := range excuse.Dependencies {
			fmt.Fprintf(w, "  - %s: %s\n", dep.Kind, dep.Package)
		}
	}

	if len(excuse.ReverseDependencies) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Reverse Dependencies: %s\n", strings.Join(excuse.ReverseDependencies, ", "))
	}

	if len(excuse.BuildFailures) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Build Failures:")
		for _, failure := range excuse.BuildFailures {
			fmt.Fprintf(w, "  - %s", failure.Kind)
			if failure.Architecture != "" {
				fmt.Fprintf(w, " on %s", failure.Architecture)
			}
			if failure.Message != "" {
				fmt.Fprintf(w, ": %s", failure.Message)
			}
			fmt.Fprintln(w)
		}
	}

	if len(excuse.Autopkgtests) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Autopkgtests:")
		for _, test := range excuse.Autopkgtests {
			fmt.Fprintf(w, "  - %s\n", test.Message)
		}
	}

	if len(excuse.Messages) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Messages:")
		for _, message := range excuse.Messages {
			fmt.Fprintf(w, "  - %s\n", message)
		}
	}
	return nil
}
