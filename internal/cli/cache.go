package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCacheCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage local git cache",
	}

	cmd.AddCommand(newCacheSyncCmd(opts))
	cmd.AddCommand(newCacheClearCmd(opts))
	cmd.AddCommand(newCacheStatusCmd(opts))
	return cmd
}

func newCacheSyncCmd(opts *Options) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Clone missing repos and fetch all cached repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			cache, err := buildGitCache(opts)
			if err != nil {
				return err
			}

			cfg := opts.Config
			if cfg == nil {
				return fmt.Errorf("no configuration loaded")
			}

			for _, proj := range cfg.Projects {
				if project != "" && proj.Name != project {
					continue
				}

				cloneURL, err := proj.Code.CloneURL()
				if err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: %s: %v\n", proj.Name, err)
					continue
				}

				fmt.Fprintf(opts.Out, "syncing %s (%s)...\n", proj.Name, cloneURL)
				if _, err := cache.EnsureRepo(cmd.Context(), cloneURL); err != nil {
					fmt.Fprintf(opts.ErrOut, "warning: %s: %v\n", proj.Name, err)
				}
			}

			fmt.Fprintln(opts.Out, "done.")
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "sync only this project")
	return cmd
}

func newCacheClearCmd(opts *Options) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove cached repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			cache, err := buildGitCache(opts)
			if err != nil {
				return err
			}

			if project == "" {
				fmt.Fprintf(opts.Out, "removing all cached repos from %s\n", cache.CacheDir())
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
				fmt.Fprintf(opts.Out, "removing cached repo for %s\n", proj.Name)
				return cache.Remove(cloneURL)
			}

			return fmt.Errorf("project %q not found in config", project)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "clear only this project")
	return cmd
}

func newCacheStatusCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "List cached repos with size info",
		RunE: func(cmd *cobra.Command, args []string) error {
			cache, err := buildGitCache(opts)
			if err != nil {
				return err
			}

			cacheDir := cache.CacheDir()
			if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
				fmt.Fprintln(opts.Out, "no cached repos found")
				return nil
			}

			fmt.Fprintf(opts.Out, "cache directory: %s\n\n", cacheDir)

			return filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}

				// Look for bare git repos (directories ending in .git containing HEAD).
				if !info.IsDir() || !isBarGitRepo(path) {
					return nil
				}

				rel, _ := filepath.Rel(cacheDir, path)
				size, _ := dirSize(path)
				fmt.Fprintf(opts.Out, "  %s  (%s)\n", rel, formatSize(size))

				return filepath.SkipDir
			})
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
