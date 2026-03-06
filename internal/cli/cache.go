package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
	pkg "github.com/gboutry/sunbeam-watchtower/internal/service/package"
	"github.com/spf13/cobra"
)

const (
	cacheTypeGit           = "git"
	cacheTypePackagesIndex = "packages-index"
	cacheTypeUpstreamRepos = "upstream-repos"
)

var allCacheTypes = []string{cacheTypeGit, cacheTypePackagesIndex, cacheTypeUpstreamRepos}

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

			if wantCacheType(args, cacheTypeGit) {
				if err := syncGitCache(cmd, opts, project); err != nil {
					return err
				}
			}

			if wantCacheType(args, cacheTypePackagesIndex) {
				if err := syncPackagesIndex(cmd, opts, distros, releases, backports); err != nil {
					return err
				}
			}

			if wantCacheType(args, cacheTypeUpstreamRepos) {
				if err := syncUpstreamRepos(cmd, opts); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "sync only this project (git only)")
	cmd.Flags().StringSliceVar(&distros, "distro", nil, "distros to update (packages-index only, default: all configured)")
	cmd.Flags().StringSliceVar(&releases, "release", nil, "distro releases to sync (packages-index only, default: all configured)")
	cmd.Flags().StringSliceVar(&backports, "backport", nil, "backports to sync (packages-index only, default: all)")

	return cmd
}

func syncGitCache(cmd *cobra.Command, opts *Options, project string) error {
	opts.Logger.Debug("starting git cache sync")
	cache, err := buildGitCache(opts)
	if err != nil {
		return err
	}

	cfg := opts.Config
	if cfg == nil {
		return fmt.Errorf("no configuration loaded")
	}

	forgeClients, err := buildForgeClients(opts)
	if err != nil {
		opts.Logger.Warn("could not build forge clients for MR sync", "error", err)
	}

	// Progress output goes to stderr when structured output is requested.
	progressOut := opts.Out
	if opts.Output == "json" || opts.Output == "yaml" {
		progressOut = opts.ErrOut
	}

	synced := 0
	for _, proj := range cfg.Projects {
		if project != "" && proj.Name != project {
			continue
		}

		cloneURL, err := proj.Code.CloneURL()
		if err != nil {
			fmt.Fprintf(opts.ErrOut, "warning: %s: %v\n", proj.Name, err)
			continue
		}

		refSpecs := mrRefSpecs(proj.Code.Forge)
		var syncOpts *port.SyncOptions
		if len(refSpecs) > 0 {
			syncOpts = &port.SyncOptions{ExtraRefSpecs: refSpecs}
		}

		fmt.Fprintf(progressOut, "syncing %s (%s)...\n", proj.Name, cloneURL)
		if _, err := cache.EnsureRepo(cmd.Context(), cloneURL, syncOpts); err != nil {
			fmt.Fprintf(opts.ErrOut, "warning: %s: %v\n", proj.Name, err)
		} else {
			synced++
		}

		if fc, ok := forgeClients[proj.Name]; ok {
			opts.Logger.Debug("fetching MR metadata", "project", proj.Name)
			mrs, mrErr := fc.Forge.ListMergeRequests(cmd.Context(), fc.ProjectID, forge.ListMergeRequestsOpts{})
			if mrErr != nil {
				opts.Logger.Warn("MR metadata fetch failed", "project", proj.Name, "error", mrErr)
			} else if len(mrs) > 0 {
				metadata := convertToMRMetadata(mrs, proj.Code.Forge)
				if storeErr := cache.StoreMRMetadata(cloneURL, metadata); storeErr != nil {
					opts.Logger.Warn("storing MR metadata failed", "project", proj.Name, "error", storeErr)
				} else {
					opts.Logger.Debug("stored MR metadata", "project", proj.Name, "count", len(metadata))
				}
			}
		}
	}

	opts.Logger.Debug("git sync complete", "repos_synced", synced)
	fmt.Fprintln(progressOut, "git sync done.")
	return nil
}

func syncPackagesIndex(cmd *cobra.Command, opts *Options, distros, releases, backports []string) error {
	opts.Logger.Debug("starting packages index sync")
	cache, err := buildDistroCache(opts)
	if err != nil {
		return err
	}
	defer cache.Close()

	sources := buildPackageSources(opts, distros, releases, nil, backports)
	if len(sources) == 0 {
		return fmt.Errorf("no distros configured (check --distro flag and config)")
	}

	svc := pkg.NewService(cache, opts.Logger)
	if err := svc.UpdateCache(cmd.Context(), sources); err != nil {
		return err
	}

	progressOut := opts.Out
	if opts.Output == "json" || opts.Output == "yaml" {
		progressOut = opts.ErrOut
	}
	fmt.Fprintln(progressOut, "packages index sync done.")
	return nil
}

// convertToMRMetadata converts forge MergeRequests to port.MRMetadata entries.
func convertToMRMetadata(mrs []forge.MergeRequest, forgeName string) []port.MRMetadata {
	return app.ConvertToMRMetadata(mrs, forgeName)
}

func newCacheClearCmd(opts *Options) *cobra.Command {
	var project string

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

			if wantCacheType(args, cacheTypeGit) {
				if err := clearGitCache(opts, project); err != nil {
					return err
				}
			}

			if wantCacheType(args, cacheTypePackagesIndex) {
				if err := clearPackagesIndex(opts); err != nil {
					return err
				}
			}

			if wantCacheType(args, cacheTypeUpstreamRepos) {
				if err := clearUpstreamRepos(opts); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "clear only this project (git only)")
	return cmd
}

func clearGitCache(opts *Options, project string) error {
	opts.Logger.Debug("clearing git cache")
	cache, err := buildGitCache(opts)
	if err != nil {
		return err
	}

	progressOut := opts.Out
	if opts.Output == "json" || opts.Output == "yaml" {
		progressOut = opts.ErrOut
	}

	if project == "" {
		fmt.Fprintf(progressOut, "removing all cached git repos from %s\n", cache.CacheDir())
		return cache.RemoveAll()
	}

	cfg := opts.Config
	if cfg == nil {
		return fmt.Errorf("no configuration loaded")
	}

	for _, proj := range cfg.Projects {
		if proj.Name != project {
			continue
		}
		cloneURL, err := proj.Code.CloneURL()
		if err != nil {
			return fmt.Errorf("%s: %w", proj.Name, err)
		}
		fmt.Fprintf(progressOut, "removing cached git repo for %s\n", proj.Name)
		return cache.Remove(cloneURL)
	}

	return fmt.Errorf("project %q not found in config", project)
}

func clearPackagesIndex(opts *Options) error {
	opts.Logger.Debug("clearing packages index cache")
	cache, err := buildDistroCache(opts)
	if err != nil {
		return err
	}

	progressOut := opts.Out
	if opts.Output == "json" || opts.Output == "yaml" {
		progressOut = opts.ErrOut
	}
	fmt.Fprintf(progressOut, "removing packages index cache from %s\n", cache.CacheDir())
	return cache.RemoveAll()
}

func newCacheStatusCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show size and freshness of all cached data",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Logger.Debug("listing cache status")

			status := cacheFullStatus{}

			// Git repos status.
			gitCache, err := buildGitCache(opts)
			if err != nil {
				return err
			}

			cacheDir := gitCache.CacheDir()
			status.Git.Directory = cacheDir
			if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
				_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if !info.IsDir() || !isBarGitRepo(path) {
						return nil
					}
					rel, _ := filepath.Rel(cacheDir, path)
					size, _ := dirSize(path)
					status.Git.Repos = append(status.Git.Repos, cacheEntry{Name: rel, Size: formatSize(size)})
					return filepath.SkipDir
				})
			}

			// Packages index status.
			distroCache, err := buildDistroCache(opts)
			if err != nil {
				status.Packages.Error = err.Error()
			} else {
				defer distroCache.Close()
				status.Packages.Directory = distroCache.CacheDir()

				svc := pkg.NewService(distroCache, opts.Logger)
				statuses, err := svc.CacheStatus()
				if err != nil {
					return err
				}
				status.Packages.Sources = statuses
			}

			// Upstream repos status.
			upDir, err := upstreamCacheDir()
			if err != nil {
				return err
			}
			status.Upstream.Directory = upDir
			if _, err := os.Stat(upDir); !os.IsNotExist(err) {
				entries, err := os.ReadDir(upDir)
				if err != nil {
					return err
				}
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					size, _ := dirSize(filepath.Join(upDir, e.Name()))
					status.Upstream.Repos = append(status.Upstream.Repos, cacheEntry{Name: e.Name(), Size: formatSize(size)})
				}
			}

			return renderCacheFullStatus(opts.Out, opts.Output, &status)
		},
	}
}

func isBarGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, "HEAD"))
	return err == nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func upstreamCacheDir() (string, error) {
	return app.UpstreamCacheDir()
}

func upstreamRepoPath(cacheDir, repoURL string) string {
	return app.UpstreamRepoPath(cacheDir, repoURL)
}

func syncUpstreamRepos(cmd *cobra.Command, opts *Options) error {
	opts.Logger.Debug("starting upstream repos sync")

	cfg := opts.Config
	if cfg == nil {
		return fmt.Errorf("no configuration loaded")
	}
	if cfg.Packages.Upstream == nil {
		opts.Logger.Debug("upstream repos: not configured, skipping")
		return nil
	}

	upDir, err := upstreamCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(upDir, 0o755); err != nil {
		return fmt.Errorf("creating upstream cache dir: %w", err)
	}

	progressOut := opts.Out
	if opts.Output == "json" || opts.Output == "yaml" {
		progressOut = opts.ErrOut
	}

	repos := map[string]string{}
	if u := cfg.Packages.Upstream.ReleasesRepo; u != "" {
		repos["releases"] = u
	}
	if u := cfg.Packages.Upstream.RequirementsRepo; u != "" {
		repos["requirements"] = u
	}

	for label, repoURL := range repos {
		localPath := upstreamRepoPath(upDir, repoURL)
		if _, err := os.Stat(localPath); err == nil {
			fmt.Fprintf(progressOut, "fetching %s (%s)...\n", label, repoURL)
			gitCmd := exec.CommandContext(cmd.Context(), "git", "-C", localPath, "fetch", "--all")
			gitCmd.Stdout = progressOut
			gitCmd.Stderr = opts.ErrOut
			if err := gitCmd.Run(); err != nil {
				fmt.Fprintf(opts.ErrOut, "warning: fetch %s: %v\n", label, err)
			}
		} else {
			fmt.Fprintf(progressOut, "cloning %s (%s)...\n", label, repoURL)
			gitCmd := exec.CommandContext(cmd.Context(), "git", "clone", "--bare", repoURL, localPath)
			gitCmd.Stdout = progressOut
			gitCmd.Stderr = opts.ErrOut
			if err := gitCmd.Run(); err != nil {
				fmt.Fprintf(opts.ErrOut, "warning: clone %s: %v\n", label, err)
			}
		}
	}

	fmt.Fprintln(progressOut, "upstream repos sync done.")
	return nil
}

func clearUpstreamRepos(opts *Options) error {
	opts.Logger.Debug("clearing upstream repos cache")
	upDir, err := upstreamCacheDir()
	if err != nil {
		return err
	}
	progressOut := opts.Out
	if opts.Output == "json" || opts.Output == "yaml" {
		progressOut = opts.ErrOut
	}
	fmt.Fprintf(progressOut, "removing upstream repos cache from %s\n", upDir)
	if err := os.RemoveAll(upDir); err != nil {
		return fmt.Errorf("removing upstream cache: %w", err)
	}
	return nil
}
