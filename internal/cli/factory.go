package cli

import (
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/distrocache"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/gitcache"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	forge "github.com/gboutry/sunbeam-watchtower/internal/pkg/forge/v1"
	lp "github.com/gboutry/sunbeam-watchtower/internal/pkg/launchpad/v1"
	"github.com/gboutry/sunbeam-watchtower/internal/port"
	"github.com/gboutry/sunbeam-watchtower/internal/service/bug"
	"github.com/gboutry/sunbeam-watchtower/internal/service/build"
	"github.com/gboutry/sunbeam-watchtower/internal/service/commit"
	pkg "github.com/gboutry/sunbeam-watchtower/internal/service/package"
	"github.com/gboutry/sunbeam-watchtower/internal/service/review"
)

// Thin wrappers that delegate to app.App methods.

func buildForgeClients(opts *Options) (map[string]review.ProjectForge, error) {
	return opts.App.BuildForgeClients()
}

func buildBugTrackers(opts *Options) (map[string]bug.ProjectBugTracker, map[string][]string, error) {
	return opts.App.BuildBugTrackers()
}

func newLaunchpadClient(lpCfg config.LaunchpadConfig, opts *Options) *lp.Client {
	return app.NewLaunchpadClient(lpCfg, opts.Logger)
}

func newLaunchpadForge(lpCfg config.LaunchpadConfig, opts *Options) *forge.LaunchpadForge {
	return app.NewLaunchpadForge(lpCfg, opts.Logger)
}

func buildRecipeBuilders(opts *Options) (map[string]build.ProjectBuilder, error) {
	return opts.App.BuildRecipeBuilders()
}

func buildRepoManager(opts *Options) (port.RepoManager, error) {
	return opts.App.BuildRepoManager()
}

func buildUpstreamProvider(opts *Options) (port.UpstreamProvider, error) {
	return opts.App.BuildUpstreamProvider()
}

func resolveCacheDir() (string, error) {
	return app.ResolveCacheDir()
}

func buildDistroCache(opts *Options) (*distrocache.Cache, error) {
	return opts.App.DistroCache()
}

func buildGitCache(opts *Options) (*gitcache.Cache, error) {
	return opts.App.GitCache()
}

func buildCommitSources(opts *Options) (map[string]commit.ProjectSource, error) {
	return opts.App.BuildCommitSources()
}

func forgeTypeFromConfig(forgeName string) forge.ForgeType {
	return app.ForgeTypeFromConfig(forgeName)
}

func mrRefSpecs(forgeName string) []string {
	return app.MRRefSpecs(forgeName)
}

func mrGitRef(forgeName string, mrID string) string {
	return app.MRGitRef(forgeName, mrID)
}

func buildPackageSources(opts *Options, distros, releases, suites, backports []string) []pkg.ProjectSource {
	return opts.App.BuildPackageSources(distros, releases, suites, backports)
}
