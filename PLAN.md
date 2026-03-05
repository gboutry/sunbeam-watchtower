# Sunbeam Watchtower - Go Rewrite Plan

## Context

The current `sunbeam-tooling` is a Python CLI (~3500 lines) that builds OpenStack rocks and charms on Launchpad. It needs to become a "watchtower" for the entire Sunbeam project: tracking builds, merge proposals/PRs/reviews, commits across repos, and Launchpad bugs — with both a CLI and a live-updating TUI dashboard. Go is chosen for speed, concurrency, and single-binary distribution.

Sunbeam repos live across **three forges**: GitHub, Launchpad, and OpenDev (Gerrit). The tool must provide a unified view across all three.

This will be a **new repository** (`sunbeam-watchtower`).

## Architecture: Hexagonal (Ports & Adapters)

```
cmd/watchtower/main.go          ← entry point, wires everything

internal/
├── adapter/                    ← concrete implementations of ports
│   ├── distrocache/            ← bbolt-backed APT Sources cache (download, parse, index, query)
│   ├── git/                    ← go-git v5 based GitClient (push, remotes, HEAD)
│   ├── gitcache/               ← local bare-clone cache for commit history
│   ├── launchpad/              ← LP recipe builders (rock/charm/snap) + repo manager + project manager
│   └── openstack/              ← OpenStack upstream provider (deliverables, constraints, name mapping)
│
├── cli/                        ← cobra command tree + factory wiring
│   ├── root.go                 ← global flags: --config, --verbose, --output, --no-color
│   ├── auth.go                 ← watchtower auth login|status
│   ├── build.go                ← watchtower build trigger|list|download|cleanup
│   ├── bug.go                  ← watchtower bug list|show|sync
│   ├── cache.go                ← watchtower cache sync|clear|status (git + packages-index + upstream-repos types)
│   ├── commit.go               ← watchtower commit log|track
│   ├── config_cmd.go           ← watchtower config show
│   ├── factory.go              ← builds forge clients, commit sources, bug trackers, distro cache, etc.
│   ├── output.go               ← shared table/json/yaml rendering
│   ├── packages.go             ← watchtower packages diff|show|list|rdepends|dsc
│   ├── project.go              ← watchtower project sync
│   ├── review.go               ← watchtower review list|show
│   └── version.go              ← watchtower version
│
├── config/                     ← viper-based config loading + validation
│   ├── config.go               ← Config, ProjectConfig, CodeConfig structs + Load/Validate
│   └── giturl.go               ← CloneURL() and CommitURL() methods on CodeConfig
│
├── pkg/
│   ├── distro/v1/              ← APT source package domain types + Debian version comparison
│   ├── forge/v1/               ← forge implementations + shared types
│   │   ├── forge.go            ← Forge interface, ForgeType, MergeRequest, Commit, ListCommitsOpts
│   │   ├── bugrefs.go          ← ExtractBugRefs() — typed LP bug references (BugRef with Closes/Partial/Related)
│   │   ├── bugtracker.go       ← BugTracker interface, Bug, BugTask types
│   │   ├── github.go           ← GitHubForge implementation
│   │   ├── gerrit.go           ← GerritForge implementation
│   │   ├── launchpad.go        ← LaunchpadForge implementation
│   │   └── launchpad_bugs.go   ← LaunchpadBugTracker implementation
│   └── launchpad/v1/           ← raw LP REST + OAuth 1.0 client
│
├── port/                       ← interfaces (the hexagonal boundary)
│   ├── forge.go                ← Forge interface (unified across GH/LP/Gerrit)
│   ├── bugtracker.go           ← BugTracker interface
│   ├── build.go                ← RecipeBuilder, RepoManager, ArtifactType, BuildState, etc.
│   ├── cache.go                ← GitRepoCache interface
│   ├── distro.go               ← DistroCache interface (APT Sources download, index, query)
│   ├── git.go                  ← GitClient interface (local git ops)
│   └── project.go              ← ProjectManager interface (series, dev focus)
│
└── service/                    ← use-case layer (business logic)
    ├── bug/                    ← Bug aggregation across LP trackers
    ├── bugsync/                ← Bug status sync from commits to LP (ref-type-aware + project task addition)
    ├── build/                  ← Build triggering, listing, downloading, cleanup
    ├── commit/                 ← Commit aggregation via CommitSource abstraction
    ├── package/                ← APT source package diff, show, list across distros/backports
    ├── project/                ← LP project series and dev focus sync
    └── review/                 ← MR aggregation across forges
```

## Multi-Forge Design

### Unified types (`internal/pkg/forge/v1/`)

Forge-specific concepts are normalized into common types:

```go
type ForgeType int
const (
    ForgeGitHub    ForgeType = iota
    ForgeLaunchpad
    ForgeGerrit
)

type MergeRequest struct {
    Forge        ForgeType
    Repo         string
    ID           string       // "#123" for GH, MP self_link for LP, change number for Gerrit
    Title        string
    Description  string
    Author       string
    SourceBranch string
    TargetBranch string
    State        MergeState   // Open, Merged, Closed, Abandoned, WIP
    ReviewState  ReviewState  // Pending, Approved, ChangesRequested, Rejected
    Checks       []Check
    URL          string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type CommitMergeRequest struct {
    ID    string     // "#123" for GH, change number for Gerrit, etc.
    State MergeState // Open, Merged, Closed, Abandoned, WIP
    URL   string     // web link to the merge request
}

type Commit struct {
    Forge        ForgeType
    Repo         string
    SHA          string
    Message      string
    Author       string
    Date         time.Time
    URL          string
    BugRefs      []BugRef             // typed LP bug references (Closes/Partial/Related)
    MergeRequest *CommitMergeRequest  // non-nil: Merged for branch commits, Open/etc. for MR refs
}
```

### Forge interface

Each forge implements this common interface. Services operate on the interface, never on concrete forges:

```go
// internal/port/forge.go
type Forge interface {
    Type() forge.ForgeType
    ListMergeRequests(ctx context.Context, repo string, opts forge.ListMergeRequestsOpts) ([]forge.MergeRequest, error)
    GetMergeRequest(ctx context.Context, repo string, id string) (*forge.MergeRequest, error)
    ListCommits(ctx context.Context, repo string, opts forge.ListCommitsOpts) ([]forge.Commit, error)
}
```

GitHub, Launchpad, and Gerrit adapters all implement `Forge`.

### CommitSource abstraction

The commit service uses a `CommitSource` interface that decouples from `port.Forge`, enabling both forge-based and git-cache-based commit reading:

```go
// internal/service/commit/source.go
type CommitSource interface {
    ListCommits(ctx context.Context, opts forge.ListCommitsOpts) ([]forge.Commit, error)
    ListMRCommits(ctx context.Context) ([]forge.Commit, error)
    ListBranches(ctx context.Context) ([]string, error)
}
```

Two implementations:
- **`ForgeCommitSource`** — wraps `port.Forge` + `ProjectID` (backward compat with REST APIs; `ListMRCommits` returns nil)
- **`CachedGitSource`** — wraps `port.GitRepoCache` + `CloneURL` + `CodeConfig` (reads from local bare clones; `ListMRCommits` reads MR ref heads from cache)

### Local Git Cache

Commits are read from local bare git clones instead of forge REST APIs, avoiding rate limits and improving performance:

```
$XDG_CACHE_HOME/sunbeam-watchtower/repos/   (fallback: ~/.cache/sunbeam-watchtower/repos/)
├── github.com/<owner>/<project>.git
├── review.opendev.org/<project-path>.git
└── git.launchpad.net/<project>.git
```

The `port.GitRepoCache` interface manages clone, fetch, log, and MR metadata operations. The `adapter/gitcache` package implements it using `go-git/v5`.

### Merge Request Tracking

Commits are annotated with MR status and links via the `CommitMergeRequest` struct:

- **Branch commits** are always annotated as `Merged` (being on the branch means the change has landed)
- **MR ref commits** (from `--include-mrs`) show their actual state: Open, WIP, Closed, Abandoned, etc.

During `cache sync`, MR refs are fetched into the bare clone and MR metadata is stored as a sidecar JSON file:
- GitHub: `+refs/pull/*/head:refs/pull/*/head` refspec
- Gerrit: `+refs/changes/*:refs/changes/*` refspec
- Launchpad: skipped (source branches live in separate repos)

The forge API is called to fetch MR metadata (state, URL, ID) which is stored as `.watchtower-mrs.json` inside each bare repo directory.

When `--include-mrs` is used with `commit log` or `commit track`:
1. Branch commits are listed as usual (all marked Merged)
2. MR head commits are resolved from cached refs and annotated with metadata
3. MR commits whose SHA already appears on the branch are deduplicated — the branch commit gets enriched with the MR's ID and URL
4. Remaining MR commits (not yet merged) are added with their original state

Output columns: `PROJECT FORGE SHA AUTHOR DATE STATUS LINK MESSAGE`

### Launchpad has dual roles

Launchpad is both a forge (merge proposals) and a build system (rock/charm/snap recipes). These are separate interfaces:
- `port.Forge` — merge proposals (implemented by `pkg/forge/v1/LaunchpadForge`)
- `port.RecipeBuilder` — recipes, builds (implemented by `adapter/launchpad/`)
- `port.BugTracker` — bugs (implemented by `pkg/forge/v1/LaunchpadBugTracker`)

### Config: per-project forge mapping

```yaml
launchpad:
  default_owner: my-team
  use_keyring: true
  series:
    - "2024.1"
    - "2024.2"
    - "2025.1"
  development_focus: "2025.1"

github:
  use_keyring: false

gerrit:
  hosts:
    - url: https://review.opendev.org

projects:
  - name: snap-openstack
    code:
      forge: github
      owner: canonical
      project: snap-openstack
      # git_url: https://custom.example.com/repo.git  # optional explicit clone URL
    bugs:
      - forge: launchpad
        project: snap-openstack
    # Per-project override of series and development focus (overrides global launchpad settings)
    series:
      - "2024.2"
      - "2025.1"
    development_focus: "2025.1"

  - name: sunbeam-charms
    code:
      forge: gerrit
      host: https://review.opendev.org
      project: openstack/sunbeam-charms

  - name: charm-keystone
    artifact_type: charm
    code:
      forge: launchpad
      project: charm-keystone-k8s
    build:
      owner: my-team
      recipes:
        - charm-keystone-recipe

build:
  default_prefix: sunbeam
  timeout_minutes: 45
  artifacts_dir: /tmp/artifacts

packages:
  upstream:
    provider: openstack
    releases_repo: https://opendev.org/openstack/releases.git
    requirements_repo: https://opendev.org/openstack/requirements.git
  distros:
    ubuntu:
      mirror: http://archive.ubuntu.com/ubuntu
      suites: [jammy, jammy-updates, noble, noble-updates, noble-proposed]
      components: [main, universe]
      backports:
        yoga:
          sources:
            - mirror: https://ppa.launchpadcontent.net/ubuntu-cloud-archive/yoga-staging/ubuntu
              suites: [focal]
              components: [main]
        caracal:
          sources:
            - mirror: https://ppa.launchpadcontent.net/ubuntu-cloud-archive/caracal-staging/ubuntu
              suites: [jammy]
              components: [main]
        gazpacho:
          sources:
            - mirror: https://ppa.launchpadcontent.net/ubuntu-cloud-archive/gazpacho-staging/ubuntu
              suites: [noble]
              components: [main]
    debian:
      mirror: http://ftp.debian.org/debian
      suites: [trixie, trixie-backports, bookworm, unstable, experimental]
      components: [main]
      backports:
        gazpacho:
          sources:
            - mirror: http://osbpo.debian.net/debian
              suites: [trixie-gazpacho-backports, trixie-gazpacho-backports-nochange]
              components: [main]
  sets:
    openstack-services:
      - nova
      - keystone
      - glance
      - neutron
      - cinder
    openstack-clients:
      - python-novaclient
      - python-keystoneclient
```

## Key Design Decisions

### Logging

All services and adapters accept a `*slog.Logger` with nil-safe defaults. Structured logging uses key-value pairs at three levels:
- **Debug** — detailed tracing (project queries, filter decisions, commit counts, path resolution)
- **Info** — state-changing operations (cloning repos, creating LP resources)
- **Warn** — non-fatal errors (fetch failures, missing auth, per-project query failures)

Enable with `--verbose` flag. Output goes to stderr.

### Build pipeline
Direct port of the 6-phase pipeline from Python, using `ArtifactStrategy` interface:
- `RockStrategy` / `CharmStrategy` / `SnapStrategy` implement artifact-specific logic
- `BuildService` orchestrates: check → create recipes → request builds → wait → monitor → download

### Graceful degradation
All `List` operations collect per-project errors and continue, surfacing errors as warnings to stderr without aborting the entire operation.

### Bug correlation
- Parse commit messages for LP bug references (`LP: #NNNNN`, `Closes-Bug: #NNNNN`, `Partial-Bug:`, `Related-Bug:`)
- `commit track --bug-id` finds commits referencing a specific bug across all projects
- `bug sync` command walks recent commits across all forges, updates LP bug task status to "Fix Committed" and assigns bugs to series (main/master→development, stable/X→X)

## Go Dependencies

| Purpose | Library |
|---------|---------|
| CLI | `spf13/cobra` + `spf13/viper` |
| GitHub API | `google/go-github/v68` |
| Gerrit API | `andygrunwald/go-gerrit` |
| Git operations | `go-git/go-git/v5` |
| YAML | `gopkg.in/yaml.v3` |
| Debian version comparison | `pault.ag/go/debian` |
| XZ decompression | `github.com/ulikunitz/xz` |
| Embedded key-value store | `go.etcd.io/bbolt` |
| Logging | `log/slog` (stdlib) |
| Testing | stdlib `testing` package |

## CLI Command Tree

```
watchtower
├── auth
│   ├── login                   Interactive Launchpad OAuth flow
│   └── status                  Show authentication status
├── build
│   ├── trigger <project>       --source --wait --timeout --owner --prefix --local-path
│   ├── list                    --project --all --state
│   ├── download <project>      --artifacts-dir
│   └── cleanup                 --project --owner --prefix --dry-run
├── bug
│   ├── list                    --project --status --importance --assignee --tag
│   ├── show <id>
│   └── sync                    --project --dry-run --days
├── cache
│   ├── sync [git|packages-index|upstream-repos]   --project --distro --backport
│   ├── clear [git|packages-index|upstream-repos]  --project
│   └── status
├── commit
│   ├── log                     --project --forge --branch --author --include-mrs
│   └── track                   --bug-id --project --forge --branch --include-mrs
├── config
│   └── show
├── packages
│   ├── diff <set>              --distro --backport --suite --component --merge --release --behind-upstream --only-in --constraints
│   ├── show <pkg>              --distro --backport --merge --release
│   ├── list                    --distro --suite --component
│   ├── dsc <pkg> <ver> [...]   --distro --backport
│   └── rdepends <pkg>          --distro --backport --suite
├── review
│   ├── list                    --project --forge --state --author
│   └── show <id>               --project
└── version

Global flags: --config, --verbose, --output=table|json|yaml, --no-color
```

## TUI: Menu-Driven Hierarchical Navigation (Future)

The TUI will be a full second UX to the CLI — a menu-driven interface where users navigate through hierarchical menus to perform all major operations using `charmbracelet/bubbletea` + `lipgloss`.

### Planned screen flow

```
Main Menu
├── Builds        → trigger, view active/history, cleanup
├── Reviews       → cross-forge MR table, detail view, filter by forge/repo
├── Commits       → cross-forge log, search by bug ref
├── Bugs          → LP bug list, detail with linked commits/reviews, sync
└── Config        → show config, auth wizards
```

## Phased Implementation

### Phase 1: Foundation — DONE
- Go module, directory structure
- Domain types in `internal/pkg/forge/v1/`
- `Forge` interface + GitHub/Gerrit/Launchpad implementations
- `BugTracker` interface + Launchpad implementation
- Launchpad REST client with OAuth
- Config loading with viper
- CLI root + `version` + `config show` + `auth login/status`

### Phase 2: Build Pipeline — DONE
- `BuildService` with assess→execute pipeline
- `RockStrategy`, `CharmStrategy`, `SnapStrategy`
- `RecipeBuilder` adapters for rocks, charms, snaps
- `RepoManager` for temp LP project/repo management
- `GitClient` adapter (go-git v5)
- CLI: `build trigger`, `build list`, `build download`, `build cleanup`
- Comprehensive tests (13 service, 21 strategy, 16 CLI)

### Phase 3: Multi-Forge Integration — DONE
- Review service with cross-forge aggregation + graceful degradation
- Bug service with cross-tracker aggregation + deduplication
- Commit service with `CommitSource` abstraction + MR commit inclusion/dedup
- Local git cache (`adapter/gitcache/`) for fast commit reads + MR ref/metadata caching
- `GitRepoCache` port interface with `SyncOptions`, `MRMetadata`, `StoreMRMetadata`, `LoadMRMetadata`, `ListMRCommits`, `ListBranches`
- `CodeConfig.CloneURL()` / `CommitURL()` derivation
- MR ref fetching: GitHub PR refs, Gerrit change refs (Launchpad skipped)
- MR metadata sidecar JSON storage (`.watchtower-mrs.json`)
- Branch commits always annotated as Merged; MR commits show actual state (Open, WIP, etc.)
- Bug sync service (`service/bugsync/`) — 2-phase sync: scan commits for LP bug refs → update task status + assign to series
- `BugTracker` interface extended with `UpdateBugTaskStatus()`, `AddBugTask()`, `GetProjectSeries()`, `GetProject()`
- CLI: `review list/show`, `bug list/show/sync`, `commit log/track` (`--include-mrs`), `cache sync/clear/status`
- Factory wiring: `buildForgeClients()`, `buildBugTrackers()`, `buildCommitSources()`, `buildGitCache()`, `mrRefSpecs()`, `mrGitRef()`
- Structured `slog` logging across all layers (CLI, services, adapters)

### Phase 4: Packages — DONE
- APT source package comparison across distros (backports nested under distro config; `--backport` flag defaults to none)
- Domain types: `SourcePackage`, `VersionComparison` in `internal/pkg/distro/v1/`
- Debian version comparison using `pault.ag/go/debian` (compare, strip revision, pick highest)
- `DistroCache` port interface — download, index, query APT Sources data
- bbolt-backed cache adapter (`internal/adapter/distrocache/`) with RFC822 Sources parser
- Downloads Sources.xz (fallback .gz) from APT mirrors, stores raw files + bbolt index
- Package service (`internal/service/package/`) — Diff, Show, List, UpdateCache, CacheStatus
- Config: `PackagesConfig` with distros (each with nested backports) and named package sets
- CLI: `packages cache update/status`, `packages diff <set>`, `packages show <pkg>`, `packages list`
- Dynamic table columns based on queried sources (SOURCE:suite headers)
- Table/JSON/YAML output for all package data types

### Phase 5: TUI (Future)
- Navigation stack, menu component, breadcrumbs
- Reusable components: table, form, progress, confirm dialog, status bar
- Views for all major features (builds, reviews, commits, bugs)
- EventNotifier/TUINotifier bridge for live build progress
- `watchtower dashboard` command

### Phase 6: Additional Features (Future)
- Release tracking (snap/charm store APIs)
- Issue tracking (GitHub issues)
- Concurrent cross-forge fan-out

### Phase 7: Polish (Future)
- Integration tests (mock servers for each forge)
- Error handling review
- Performance profiling
- CI/CD, goreleaser
- **Deliverable: v1.0**

## Implementation Progress

### Completed

#### Foundation
- [x] Go module, directory structure
- [x] Config loading with viper (`internal/config/config.go`)
- [x] `LaunchpadConfig.Series` and `DevelopmentFocus` fields with validation
- [x] CLI root + `version` + `config show` (`internal/cli/root.go`, `internal/cli/config_cmd.go`)
- [x] Launchpad REST client (`internal/pkg/launchpad/v1/`)
- [x] `CreateProjectSeries()`, `GetSeries()`, `SetDevelopmentFocus()` LP client methods (`project.go`)
- [x] `auth login` + `auth status` commands (`internal/cli/auth.go`)

#### Forge Abstraction (`internal/pkg/forge/v1/`)
- [x] `Forge` interface — `Type()`, `ListMergeRequests()`, `GetMergeRequest()`, `ListCommits()`
- [x] Unified types: `MergeRequest`, `Commit`, `CommitMergeRequest`, `ForgeType`, `MergeState`, `ReviewState`, `Check`
- [x] `LaunchpadForge` implementation
- [x] `GitHubForge` implementation
- [x] `GerritForge` implementation
- [x] `BugTracker` interface — `Type()`, `GetBug()`, `ListBugTasks()`, `UpdateBugTaskStatus()`, `AddBugTask()`, `GetProjectSeries()`, `GetProject()` (`bugtracker.go`)
- [x] `LaunchpadBugTracker` implementation (`launchpad_bugs.go`)
- [x] Unified `Bug`, `BugTask`, `ProjectSeries`, `Project` types with `ListBugTasksOpts`
- [x] `ExtractBugRefs()` — typed bug reference extraction (BugRefCloses, BugRefPartial, BugRefRelated)

#### Review Service (`internal/service/review/`)
- [x] `Service.List()` aggregates MRs across projects with graceful degradation
- [x] `Service.Get()` fetches a single MR by project + ID
- [x] Structured logging with `*slog.Logger`
- [x] Tests: aggregation, project/forge/author filtering, graceful degradation, Get

#### Bug Service (`internal/service/bug/`)
- [x] `Service.List()` aggregates bug tasks across trackers with deduplication by `(forge, project)` key
- [x] `Service.Get()` fetches a single bug by ID with its tasks
- [x] Multiple watchtower projects can share a single LP bug project without duplicate queries
- [x] Structured logging with `*slog.Logger`
- [x] Tests: aggregation, project filtering, deduplication, graceful degradation, Get

#### Bug Sync Service (`internal/service/bugsync/`)
- [x] `Service.Sync()` — 2-phase: scan commits for LP bug refs → update task status + assign to series
- [x] Phase 1: collect typed bug refs (Closes/Partial/Related) from main/master/stable/* branch commits, filter by `since` via `ListBugTasksOpts.CreatedSince`
- [x] Phase 2: determine target status from ref type (Closes→Fix Committed, Partial→In Progress, Related→skip), add missing project tasks, update bug task status, assign to series
- [x] `SyncOptions` — projects, dryRun, since filter
- [x] `SyncResult` + `SyncAction` — detailed reporting with status_update, series_assignment, and add_project_task actions
- [x] LP project mapping: watchtower project → LP bug projects, ensures bugs have tasks on all associated projects
- [x] Status priority: never downgrades (Fix Committed not overwritten by In Progress)
- [x] Project and series caching to avoid redundant LP API calls
- [x] Structured logging with `*slog.Logger`
- [x] Tests: commit scanning, bug collection, Closes→Fix Committed, Partial→In Progress, Related→skip, no-downgrade, missing project task addition, series assignment
- [x] `CommitSource` interface — `ListCommits` + `ListMRCommits` + `ListBranches` decoupling commit reading from forge API
- [x] `ForgeCommitSource` — backward-compat wrapper around `port.Forge` (`ListMRCommits` returns nil)
- [x] `CachedGitSource` — reads commits from local git cache, MR commits from cached refs
- [x] `ProjectSource` pairs a `CommitSource` with `ForgeType` metadata
- [x] `Service.List()` aggregates commits across projects with graceful degradation
- [x] Branch commits always annotated as Merged (being on branch = landed)
- [x] `IncludeMRs` option — fetches MR head commits, deduplicates against branch, annotates with MR metadata
- [x] Dedup logic: branch commits matching MR SHAs get enriched with MR ID/URL (state forced to Merged)
- [x] Bug ID filtering via `BugRefs` field
- [x] Structured logging with `*slog.Logger`
- [x] Tests: aggregation, project/forge/author/bugID filtering, graceful degradation, MR inclusion, MR dedup with Merged annotation

#### Local Git Cache (`internal/adapter/gitcache/`)
- [x] `port.GitRepoCache` interface — `EnsureRepo`, `Fetch`, `ListCommits`, `StoreMRMetadata`, `LoadMRMetadata`, `ListMRCommits`, `ListBranches`, `Remove`, `RemoveAll`, `CacheDir`
- [x] `port.SyncOptions` — extra refspecs for MR ref fetching
- [x] `port.MRMetadata` — sidecar metadata (ID, State, URL, HeadSHA, GitRef)
- [x] `adapter/gitcache/Cache` — bare clone, fetch, log, MR sidecar JSON, MR commit resolution via go-git v5
- [x] URL-to-path mapping: `cloneURL` → `<baseDir>/<host>/<path>.git`
- [x] Branch resolution with main/master fallback
- [x] Since/Author filtering during log iteration
- [x] `ExtractBugRefs()` applied to each commit message
- [x] Extra refspec fetching for GitHub PR refs and Gerrit change refs
- [x] `.watchtower-mrs.json` sidecar storage inside bare repo directories
- [x] `ListMRCommits` — resolves MR head commits by git ref or SHA, annotates with metadata
- [x] Structured logging with `*slog.Logger`
- [x] Tests: clone+list, fetch existing, Since filter, Remove, RemoveAll, repoPath, MR metadata round-trip, MR commit listing

#### Config Extensions
- [x] `CodeConfig.GitURL` field — explicit clone URL override
- [x] `CodeConfig.CloneURL()` — derives clone URL from forge/owner/host/project
- [x] `CodeConfig.CommitURL(sha)` — derives web URL for a commit
- [x] Tests: URL derivation for all forge types + explicit override

#### CLI Commands
- [x] `commit log` — `--project`, `--forge`, `--branch`, `--author`, `--include-mrs`
- [x] `commit track` — `--bug-id`, `--project`, `--forge`, `--branch`, `--include-mrs`
- [x] `cache sync` — `--project`, clones missing + fetches all, fetches MR refs + metadata via forge API
- [x] `cache sync upstream-repos` — clones/fetches upstream version repos (releases, requirements) as bare clones
- [x] `cache clear` — `--project`, removes cached repos
- [x] `cache clear upstream-repos` — removes upstream repos cache directory
- [x] `cache status` — lists cached repos with disk usage, upstream repos with sizes
- [x] `review list` — `--project`, `--forge`, `--state`, `--author`
- [x] `review show <id>` — `--project` (required), detail view with description + checks
- [x] `bug list` — `--project`, `--status`, `--importance`, `--assignee`, `--tag`
- [x] `bug show <id>` — detail view with description + task table
- [x] `bug sync` — `--project`, `--dry-run`, `--days`, updates LP bug status from cached commits
- [x] `project sync` — `--project`, `--dry-run`, ensures LP project series and development focus
- [x] Debug logging in all CLI commands via `opts.Logger`
- [x] Table/JSON/YAML output for all data types (`internal/cli/output.go`)
- [x] Commit table includes STATUS and LINK columns for MR annotation

#### Hexagonal Port Interfaces (`internal/port/`)
- [x] `Forge` interface — hexagonal boundary for forge operations
- [x] `BugTracker` interface — hexagonal boundary for bug tracking (`UpdateBugTaskStatus`, `AddBugTask`, `GetProjectSeries`, `GetProject`)
- [x] `RecipeBuilder` interface — CRUD + build operations for LP recipes
- [x] `RepoManager` interface — temp LP project/repo/ref management
- [x] `GitClient` interface — local git operations
- [x] `GitRepoCache` interface — local bare-clone cache for commits + MR metadata + branch listing
- [x] `ProjectManager` interface — LP project series management + development focus
- [x] `SyncOptions`, `MRMetadata` types — MR ref fetching and sidecar storage
- [x] Core build types: `ArtifactType`, `BuildState`, `Recipe`, `Build`, `BuildRequest`

#### Build Pipeline — Adapters
- [x] `adapter/launchpad/RockBuilder` — `RecipeBuilder` for rocks
- [x] `adapter/launchpad/CharmBuilder` — `RecipeBuilder` for charms
- [x] `adapter/launchpad/SnapBuilder` — `RecipeBuilder` for snaps
- [x] `adapter/launchpad/RepoManager` — temp LP project/repo/ref with exponential backoff
- [x] `adapter/git/Client` — go-git v5 based GitClient with structured logging

#### Build Pipeline — Service (`internal/service/build/`)
- [x] `ArtifactStrategy` interface + `RockStrategy`, `CharmStrategy`, `SnapStrategy`
- [x] Build state machine: `ParseBuildState` + `IsTerminal()`, `IsActive()`, `IsFailure()`
- [x] Re-entrant `Service.Trigger()` — assess→execute pipeline
- [x] `Service.List()` — aggregates builds across projects with graceful degradation
- [x] `Service.Download()` — retrieves succeeded build artifacts
- [x] `Service.Cleanup()` — deletes temporary recipes by prefix with dry-run support
- [x] Tests: 13 service tests, 21 strategy tests

#### Build Pipeline — CLI & Config
- [x] `build trigger <project> [recipes...]` — `--source`, `--wait`, `--timeout`, `--owner`, `--prefix`
- [x] `build list` — `--project`, `--all`, `--state`
- [x] `build download <project> [recipes...]` — `--artifacts-dir`
- [x] `build cleanup` — `--project`, `--owner`, `--prefix`, `--dry-run`
- [x] Factory functions: `buildRecipeBuilders()`, `buildRepoManager()`, `buildGitCache()`, `buildCommitSources()`, `mrRefSpecs()`, `mrGitRef()`
- [x] CLI tests: 16 tests covering command registration, flag parsing, defaults

#### Project Sync (`internal/service/project/`)
- [x] `adapter/launchpad/ProjectManager` — wraps LP client for series CRUD + dev focus
- [x] `service/project/Service.Sync()` — ensures series exist on LP projects, sets development focus
- [x] Action types: `ActionCreateSeries`, `ActionSetDevFocus`, `ActionDevFocusUnchanged`
- [x] `SyncOptions` — `Projects []string` filter, `DryRun bool`
- [x] Config: `Series []string`, `DevelopmentFocus string` on `LaunchpadConfig` (global) and `ProjectConfig` (per-project override) with validation
- [x] Per-project overrides: project-level `series`/`development_focus` take precedence over global `launchpad` settings
- [x] Tests: 7 service tests (create series, set focus, unchanged, dry-run, project filter, per-project overrides, multi-project)
- [x] CLI tests: project sync command registration + flag parsing

#### Cross-Cutting
- [x] Structured `slog` logging across all layers (CLI, services, adapters, cache)
- [x] `README.md` — installation, config, all commands, debug logging guide
- [x] `CONTRIBUTING.md` — project structure, architecture, logging guidelines, development workflow

#### Packages — APT Source Package Comparison (`internal/pkg/distro/v1/`, `internal/adapter/distrocache/`, `internal/service/package/`)
- [x] Domain types: `SourcePackage`, `VersionComparison` in `internal/pkg/distro/v1/distro.go`
- [x] Debian version comparison: `CompareVersions()`, `StripDebianRevision()`, `PickHighest()` using `pault.ag/go/debian`
- [x] Tests: version comparison, revision stripping, pick highest (6 tests)
- [x] `DistroCache` port interface: `Update`, `Query`, `Status`, `CacheDir`, `Close` (`internal/port/distro.go`)
- [x] `SourceEntry`, `QueryOpts`, `CacheStatus` port types
- [x] RFC822 Sources parser: handles `.xz` and `.gz` decompression, continuation lines, missing trailing newline
- [x] Tests: parser with sample Sources data (3 tests)
- [x] bbolt-backed cache adapter (`internal/adapter/distrocache/cache.go`): download, parse, index, query
- [x] Sources download with `.xz` fallback to `.gz`, raw file storage under `sources/{name}/`
- [x] bbolt schema: bucket per source name, key = `<package>/<suite>/<component>`, value = JSON-encoded `SourcePackage`
- [x] `meta.json` timestamp tracking per source group
- [x] Query filtering by package name, suite, component
- [x] Tests: query all, query by package, query by suite, nonexistent bucket, status, meta round-trip (7 tests)
- [x] Package service (`internal/service/package/`): `Diff`, `Show`, `List`, `UpdateCache`, `CacheStatus`
- [x] Diff logic: queries cache per source, groups by package name, sorts alphabetically
- [x] Tests: diff across sources, show single package, show missing, list with suite filter (4 tests)
- [x] Config: `PackagesConfig` with `Distros`, `Sets`, `Upstream` fields added to `Config`
- [x] `DistroConfig` (with nested `Backports`), `BackportConfig`, `DistroSourceConfig`, `UpstreamConfig` types
- [x] Backports are children of their parent distro — `--distro ubuntu` only includes Ubuntu + UCA backports, not Debian/OSBPO
- [x] bbolt bucket names qualified as `distro/backport` to avoid collision when backport names match across distros
- [x] `UpstreamConfig` (optional pointer): `Provider`, `ReleasesRepo`, `RequirementsRepo`
- [x] Validation: upstream requires `Provider`; openstack provider requires `ReleasesRepo`
- [x] CLI: `cache sync packages-index` — `--distro` (defaults to all configured), `--backport` (defaults to none; pass names to include backports)
- [x] CLI: `cache status` — shows source name, package count, last updated, disk size
- [x] CLI: `packages diff <set>` — `--distro`, `--backport`, `--suite`, `--component`, `--merge`, `--release`, `--behind-upstream`, `--only-in`, `--constraints`
- [x] CLI: `packages show <pkg>` — `--distro`, `--backport`, `--merge`, `--release`
- [x] CLI: `packages list` — `--distro` (required), `--suite`, `--component`
- [x] `buildDistroCache()` factory function in `factory.go`
- [x] `buildPackageSources()` resolves --distro and --backport flags against config (backports nested under distro; qualified bucket names as `distro/backport`; default --backport=none skips backports unless explicitly requested)
- [x] Dynamic diff table columns: `SOURCE:suite` headers generated from query results
- [x] Merged diff table: `--merge` flag consolidates suites per source into one column with highest version + origin markers (R/U/P/S/E)
- [x] Table/JSON/YAML output renderers: `renderDiffResults`, `renderSourcePackages`, `renderCacheStatus`
- [x] Human-readable byte formatting for disk size display
- [x] CLI: `packages dsc <pkg> <version> [...]` — `--distro`, `--backport`, looks up .dsc URLs from cached Sources files
- [x] CLI: `packages rdepends <pkg>` — `--distro`, `--backport`, `--suite`, finds source packages that build-depend on a given package; warns on binary package names; annotates backport results with `suite/backport-name`
- [x] Upstream version provider: `UpstreamProvider` interface (`ListDeliverables`, `GetConstraints`, `MapPackageName`)
- [x] OpenStack adapter (`internal/adapter/openstack/`): parses deliverable YAML from releases repo, upper-constraints.txt from requirements repo, name mapping heuristics + override map
- [x] `--release` flag: annotates diff/show output with upstream version column
- [x] `--behind-upstream` filter: keeps only packages where distro version < upstream version
- [x] `--only-in <source>` filter: keeps only packages present in the named source
- [x] `--constraints <release>` flag: merges upper-constraints packages into diff results
- [x] `buildUpstreamProvider()` factory function in `factory.go`
- [x] Wired into root command via `newPackagesCmd(opts)`
- [x] rdepends perf: `QueryDetailed` reads from bbolt (not re-parsing xz from disk); parses `Build-Depends-Indep`

### Next Steps
- [ ] MCP server
- [ ] TUI dashboard (`charmbracelet/bubbletea`)
- [ ] Release tracking (snap/charm store APIs)
- [ ] Issue tracking (GitHub issues)
- [ ] Concurrent cross-forge fan-out (`FanOut[T,R]` helper)
- [ ] CI/CD, goreleaser

## Verification

- Unit tests for all services (with mocked ports/sources)
- Adapter tests for git cache (temp bare repos, clone/fetch/log, MR metadata round-trip, MR commit listing)
- Config tests (loading, validation, URL derivation)
- CLI tests (command registration, flag parsing, argument validation)
- Commit service tests for MR inclusion, dedup with Merged annotation, disabled flag
- Bugsync service tests for commit scanning, bug collection, ref-type status mapping, no-downgrade, project task addition, series assignment
- Project service tests for series creation, dev focus management, dry-run, project filtering
- All tests: `go test ./...`
- All vet: `go vet ./...`
- Manual: `watchtower cache sync` → clones repos + fetches MR refs + stores MR metadata + clones upstream repos
- Manual: `watchtower cache sync upstream-repos` → clones/fetches upstream version repos only
- Manual: `watchtower commit log` shows branch commits with STATUS=Merged
- Manual: `watchtower commit log --include-mrs` shows both branch and MR commits with STATUS/LINK columns
- Manual: `watchtower cache sync packages-index --distro ubuntu` downloads Sources.xz and builds bbolt index (backports excluded by default)
- Manual: `watchtower cache sync packages-index --distro ubuntu --backport gazpacho` syncs distro + specific backport
- Manual: `watchtower cache status` shows git repos and packages index freshness
- Manual: `watchtower packages diff openstack-services --distro ubuntu,debian` shows version comparison table
- Manual: `watchtower packages show nova --distro ubuntu` shows all suites for a package
- Manual: `watchtower packages list --distro debian --suite trixie` lists all packages in a suite
- Manual: `watchtower packages rdepends python-oslo-config --distro ubuntu` finds reverse build-depends (backports excluded by default)
- Manual: `watchtower packages rdepends python-oslo-config --distro ubuntu --backport gazpacho` includes backport results with `suite/backport-name` annotation
- Manual: `watchtower packages dsc nova 1:28.0.0-0ubuntu1 --distro ubuntu` finds .dsc URLs
- Manual: `watchtower packages diff services --release 2025.1 --merge` shows upstream column with merged suites
- Manual: `watchtower packages diff services --release 2025.1 --behind-upstream` shows only packages behind upstream
- Manual: `watchtower packages diff services --only-in uca` shows only packages in UCA
- Manual: `watchtower packages diff services --constraints 2025.1` includes upper-constraints packages in diff
