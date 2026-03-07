package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const (
	cacheTypeGit           = "git"
	cacheTypePackagesIndex = "packages-index"
	cacheTypeUpstreamRepos = "upstream-repos"
	cacheTypeBugs          = "bugs"
	cacheTypeExcuses       = "excuses"
)

var allCacheTypes = []string{cacheTypeGit, cacheTypePackagesIndex, cacheTypeUpstreamRepos, cacheTypeBugs, cacheTypeExcuses}

func validateCacheTypes(args []string) error {
	for _, arg := range args {
		if !slices.Contains(allCacheTypes, arg) {
			return fmt.Errorf("unknown cache type %q (valid: %s)", arg, strings.Join(allCacheTypes, ", "))
		}
	}
	return nil
}

func wantCacheType(args []string, typ string) bool {
	return len(args) == 0 || slices.Contains(args, typ)
}

func newCacheCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage local caches (git repos, APT sources)",
	}

	cmd.AddCommand(newCacheSyncCmd(opts))
	cmd.AddCommand(newCacheClearCmd(opts))
	cmd.AddCommand(newCacheStatusCmd(opts))
	return cmd
}

func newCacheSyncCmd(opts *Options) *cobra.Command {
	var project string
	var distros, releases, backports []string
	var trackers []string

	cmd := &cobra.Command{
		Use:   "sync [types...]",
		Short: "Sync caches (git repos and/or APT packages index)",
		Long: fmt.Sprintf("Sync one or more cache types. If no types are given, all are synced.\n\nValid types: %s",
			strings.Join(allCacheTypes, ", ")),
		ValidArgs: allCacheTypes,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCacheTypes(args); err != nil {
				return err
			}
			workflow := opts.Frontend().Cache()

			progressOut := opts.Out
			if opts.Output == "json" || opts.Output == "yaml" {
				progressOut = opts.ErrOut
			}

			if wantCacheType(args, cacheTypeGit) {
				fmt.Fprintln(progressOut, "syncing git caches...")
				result, err := workflow.SyncGit(cmd.Context(), project)
				if err != nil {
					return err
				}
				for _, w := range result.Warnings {
					fmt.Fprintf(opts.ErrOut, "warning: %s\n", w)
				}
				fmt.Fprintf(progressOut, "git sync done (%d repos synced).\n", result.Synced)
			}

			if wantCacheType(args, cacheTypePackagesIndex) {
				fmt.Fprintln(progressOut, "syncing packages index...")
				if err := workflow.SyncPackagesIndex(cmd.Context(), distros, releases, backports); err != nil {
					return err
				}
				fmt.Fprintln(progressOut, "packages index sync done.")
			}

			if wantCacheType(args, cacheTypeUpstreamRepos) {
				fmt.Fprintln(progressOut, "syncing upstream repos...")
				result, err := workflow.SyncUpstream(cmd.Context())
				if err != nil {
					return err
				}
				fmt.Fprintf(progressOut, "upstream repos sync: %s\n", result.Status)
			}

			if wantCacheType(args, cacheTypeBugs) {
				fmt.Fprintln(progressOut, "syncing bug caches...")
				result, err := workflow.SyncBugs(cmd.Context(), project)
				if err != nil {
					return err
				}
				fmt.Fprintf(progressOut, "bug cache sync done (%d tasks synced).\n", result.Synced)
			}

			if wantCacheType(args, cacheTypeExcuses) {
				fmt.Fprintln(progressOut, "syncing excuses caches...")
				result, err := workflow.SyncExcuses(cmd.Context(), trackers)
				if err != nil {
					return err
				}
				fmt.Fprintf(progressOut, "excuses cache sync: %s\n", result.Status)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "sync only this project (git only)")
	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to update (packages-index only, default: all configured)")
	cmd.Flags().StringSliceVar(&releases, "release", nil, "distro releases to sync (packages-index only, default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", nil, "backports to sync (packages-index only, default: all)")
	cmd.Flags().StringSliceVar(&trackers, "tracker", nil, "excuses trackers to sync (excuses only, default: all configured trackers)")

	return cmd
}

func newCacheClearCmd(opts *Options) *cobra.Command {
	var project string
	var trackers []string

	cmd := &cobra.Command{
		Use:   "clear [types...]",
		Short: "Clear cached data (git repos and/or APT packages index)",
		Long: fmt.Sprintf("Clear one or more cache types. If no types are given, all are cleared.\n\nValid types: %s",
			strings.Join(allCacheTypes, ", ")),
		ValidArgs: allCacheTypes,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCacheTypes(args); err != nil {
				return err
			}
			workflow := opts.Frontend().Cache()

			progressOut := opts.Out
			if opts.Output == "json" || opts.Output == "yaml" {
				progressOut = opts.ErrOut
			}

			if wantCacheType(args, cacheTypeGit) {
				fmt.Fprintln(progressOut, "clearing git cache...")
				if err := workflow.Clear(cmd.Context(), "git", project); err != nil {
					return err
				}
				fmt.Fprintln(progressOut, "git cache cleared.")
			}

			if wantCacheType(args, cacheTypePackagesIndex) {
				fmt.Fprintln(progressOut, "clearing packages index cache...")
				if err := workflow.Clear(cmd.Context(), "packages-index", ""); err != nil {
					return err
				}
				fmt.Fprintln(progressOut, "packages index cache cleared.")
			}

			if wantCacheType(args, cacheTypeUpstreamRepos) {
				fmt.Fprintln(progressOut, "clearing upstream repos cache...")
				if err := workflow.Clear(cmd.Context(), "upstream-repos", ""); err != nil {
					return err
				}
				fmt.Fprintln(progressOut, "upstream repos cache cleared.")
			}

			if wantCacheType(args, cacheTypeBugs) {
				fmt.Fprintln(progressOut, "clearing bug cache...")
				if err := workflow.Clear(cmd.Context(), "bugs", project); err != nil {
					return err
				}
				fmt.Fprintln(progressOut, "bug cache cleared.")
			}

			if wantCacheType(args, cacheTypeExcuses) {
				fmt.Fprintln(progressOut, "clearing excuses cache...")
				if err := workflow.ClearExcuses(cmd.Context(), trackers); err != nil {
					return err
				}
				fmt.Fprintln(progressOut, "excuses cache cleared.")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "clear only this project (git only)")
	cmd.Flags().StringSliceVar(&trackers, "tracker", nil, "excuses trackers to clear (excuses only, default: all configured trackers)")
	return cmd
}

func newCacheStatusCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show size and freshness of all cached data",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("listing cache status")

			result, err := opts.Frontend().Cache().Status(cmd.Context())
			if err != nil {
				return err
			}

			status := cacheFullStatus{}
			status.Git.Directory = result.Git.Directory
			for _, r := range result.Git.Repos {
				status.Git.Repos = append(status.Git.Repos, cacheEntry{Name: r.Name, Size: r.Size})
			}
			status.Packages.Directory = result.Packages.Directory
			status.Packages.Sources = result.Packages.Sources
			status.Packages.Error = result.Packages.Error
			status.Upstream.Directory = result.Upstream.Directory
			for _, r := range result.Upstream.Repos {
				status.Upstream.Repos = append(status.Upstream.Repos, cacheEntry{Name: r.Name, Size: r.Size})
			}

			status.Bugs.Entries = append(status.Bugs.Entries, result.Bugs.Entries...)
			status.Bugs.Directory = result.Bugs.Directory
			status.Bugs.Error = result.Bugs.Error
			status.Excuses.Entries = append(status.Excuses.Entries, result.Excuses.Entries...)
			status.Excuses.Directory = result.Excuses.Directory
			status.Excuses.Error = result.Excuses.Error

			return renderCacheFullStatus(opts.Out, opts.Output, &status)
		},
	}
}
