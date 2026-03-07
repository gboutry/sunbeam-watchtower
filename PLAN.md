# Sunbeam Watchtower — Plan

## Foreword

You LLM reader, these next section (up to bug correlation included) are the original plan for Watchtower. You are not allowed to edit them, but you can read them for context. The "Current architecture" section and everything below is a summary of the recent refactor outcomes, which you can edit and update as needed.

## Goal of the project

Watchtower is a unified tracking and management tool for Canonical OpenStack (Sunbeam) across its full lifecycle — development, release, and maintenance. It integrates with git repos, bug trackers, build systems, and package archives to provide a single pane of glass for monitoring project health, synchronizing data, and triggering actions.

### Why Watchtower exists

Canonical OpenStack (based on Sunbeam) is a complex ecosystem composed of many build artifacts spread across multiple forges, bug trackers, and git repositories. A single Sunbeam deployment involves:

| Artifact type    | Count | Description                                                        |
|------------------|-------|--------------------------------------------------------------------|
| Packages         | many  | APT metadata, source packages, and binary packages                 |
| Rocks            | 40+   | OCI images built from Ubuntu packages                              |
| Snaps            | 5+    | Snap packages built from Ubuntu packages                           |
| Charms           | 40+   | Juju operators that manage the Rocks and Snaps                     |
| snap-openstack   | 1     | Orchestration layer that ties everything together                  |

This adds up to hundreds of individually tracked items. Without a unified tool, monitoring and managing them requires manual coordination across many separate systems. Watchtower eliminates that burden.

### Scope and design constraints

1. **General-purpose core, Sunbeam-specific adapters.** The API and core services are not Sunbeam-specific. Any software ecosystem with similar complexity (many artifacts, multiple forges, multiple trackers) should be manageable through Watchtower. All Canonical OpenStack–specific logic lives in the adapter layer; core services and ports remain forge-agnostic and reusable.

2. **Forge-pluggable architecture.** Launchpad is the primary forge today (bug tracker, build system). The architecture must allow adding other forges (GitHub, Gerrit, etc.) without changing the core application logic.

3. **Hexagonal architecture.** Ports define interfaces; services implement domain logic; adapters bridge external systems. Primary adapters (API, CLI) and secondary adapters (Launchpad, git, caches) never cross-import.

### Features in scope for the initial implementation

- **Unified query API** — package metadata, bug status, build status, project health, and migration excuses from a single endpoint surface.
- **Cache synchronization** — local caches synced with upstream data sources (git repos, bug trackers, build systems, package archives, excuses feeds).
- **Build triggering** — request, monitor, retry, cancel, and download builds via the API and CLI.
- **CLI** — command-line interface for common operations (sync, build trigger, status checks, cache management).
- **Authentication and credential management** — OAuth and credential flows for upstream services (Launchpad today; extensible to others).

### Bug correlation (high-priority feature)

The bug correlation system is one of the most important features. Its purpose is to identify where a bug was fixed (which commit, which repo, which artifact) and ensure the fix is properly recorded in the relevant trackers. This is critical for maintaining Canonical OpenStack, where a single upstream fix may need to be tracked across multiple packages, charms, and rocks. The system must be accurate and reliable, as incorrect or missing correlation directly impacts release quality.

## Current architecture

Sunbeam Watchtower now follows a stricter hexagonal layout:

- **Entrypoint**: `cmd/watchtower`
- **Primary adapters**: `internal/adapter/primary/api` and `internal/adapter/primary/cli`
- **Composition root**: `internal/app`
- **Core ports**: `internal/core/port` (interfaces only)
- **Core services**: `internal/core/service/*`
- **Secondary adapters**: `internal/adapter/secondary/*`
- **Public reusable packages**: `pkg/client`, `pkg/dto/v1`, `pkg/distro/v1`, `pkg/forge/v1`, `pkg/launchpad/v1`
- **Configuration loading**: `internal/config`

The practical request from this refactor is in place:

- reusable DTOs and client-facing contracts live under `pkg/`
- `internal/core/service/*` depends only on `internal/core/port/*` and `pkg/*`
- `internal/core/port/*` contains interfaces only
- primary adapters do not import secondary adapters directly
- public `pkg/*` code no longer imports `internal/*`
- `internal/app` remains the shared wiring layer used by the CLI and HTTP API

## Package layout

```text
cmd/
└── watchtower/
    └── main.go

internal/
├── adapter/
│   ├── primary/
│   │   ├── api/
│   │   └── cli/
│   └── secondary/
│       ├── bugcache/
│       ├── distrocache/
│       ├── git/
│       ├── gitcache/
│       ├── launchpad/
│       └── openstack/
├── app/
├── config/
└── core/
    ├── port/
    └── service/
        ├── bug/
        ├── bugsync/
        ├── build/
        ├── commit/
        ├── package/
        ├── project/
        └── review/

pkg/
├── client/
├── distro/v1/
├── dto/v1/
├── forge/v1/
└── launchpad/v1/
```

## Architecture rules enforced in CI

- `arch-go` enforces the package dependency model above with 100% compliance and coverage.
- `depguard` mirrors the same boundaries in `golangci-lint`:
  - `cmd/watchtower` enters through primary adapters only
  - primary adapters do not import secondary adapters
  - core services do not import adapters, config, or app wiring
  - secondary adapters do not import primary adapters or core services
  - public `pkg/*` packages stay independent from `internal/*`
- `internal/adapter/*` packages are implementation packages and must not define interfaces.
- `internal/core/port/*` is reserved for interfaces only.

## API surface

The HTTP API remains the application boundary for non-CLI consumers.

- `GET /openapi.json` — OpenAPI 3.1 spec
- `GET /docs` — interactive docs UI
- `GET /api/v1/health` — health check
- `GET /api/v1/auth/status`
- `POST /api/v1/auth/launchpad/begin`
- `POST /api/v1/auth/launchpad/finalize`
- `POST /api/v1/auth/launchpad/logout`
- `GET /api/v1/packages/*` / `POST /api/v1/packages/cache/sync`
  - `GET /api/v1/packages/detail/{name}` — full APT metadata for a package
  - `GET /api/v1/packages/excuses` / `GET /api/v1/packages/excuses/{name}` — normalized migration excuses from Ubuntu/Debian trackers
- `GET /api/v1/bugs*` / `POST /api/v1/bugs/sync`
- `GET /api/v1/reviews*`
- `GET /api/v1/commits*`
- `POST /api/v1/builds/*` / `GET /api/v1/builds`
- `POST /api/v1/projects/sync`
- `POST /api/v1/cache/sync/git` / `POST /api/v1/cache/sync/upstream` / `POST /api/v1/cache/sync/bugs` / `POST /api/v1/cache/sync/excuses`
- `DELETE /api/v1/cache/{type}` (git, packages-index, upstream-repos, bugs, excuses)
- `GET /api/v1/cache/status`
- `GET /api/v1/config`

## Recent refactor outcomes

- migrated the old `internal/api`, `internal/cli`, `internal/service`, and `internal/port` layout into the new `internal/adapter/*` and `internal/core/*` split
- moved reusable client and contract packages to root `pkg/`
- introduced public config DTOs under `pkg/dto/v1`, so `pkg/client` no longer leaks `internal/config`
- removed the remaining core-service boundary leak by making bug sync consume `port.CommitSource` instead of commit-service types
- preserved existing command and API behavior while tightening architecture linting
- moved Launchpad auth behind core ports and an `internal/core/service/auth` application service
- added Launchpad auth API/client flows with server-side pending auth state, opaque flow IDs, and no token secrets in API DTOs
- made CLI `auth login|status|logout` a thin adapter over the application/API auth surface instead of directly owning OAuth + credential persistence
- added direct adapter tests for Launchpad credential persistence, pending auth flow storage, and the Launchpad auth adapter so the new auth boundary has focused secondary-adapter coverage
- added HTTP tests for the Launchpad auth API endpoints so begin/finalize/logout/status flows now have primary-adapter coverage too
- added focused Launchpad repo/project manager tests covering current-user lookup, default-repo fallback, project/repo create-or-reuse flows, git-ref resolution, project series handling, and development-focus updates
- added focused Launchpad snap/charm/rock builder tests covering recipe creation/listing, build requests, build listing, artifact URL lookup, and retry/cancel/delete actions
- added primary-adapter tests for build/cache/project endpoints covering build-list success, invalid timeout validation, invalid excuses tracker validation, cache-status wiring, and project-sync auth-required handling
- added CLI execution tests for `auth login|status|logout` plus a build-list rendering path backed by a stubbed HTTP API

## Validation

The refactor is currently validated by all of the following:

- `go test ./...`
- `golangci-lint run ./...`
- `arch-go --color no`
- `pre-commit run --all-files`

## Deferred contract-test plan

We are **not** adopting broad go-vcr coverage now.

If we later add cassette-backed contract tests, keep them deliberately small and easy to refresh:

- scope them to a handful of high-value `pkg/launchpad/v1` or `pkg/forge/v1` client methods whose real payloads are hard to model with `httptest`
- prefer read-only or safely repeatable endpoints first; avoid OAuth handshakes and destructive write flows
- store cassettes under package-local `testdata/vcr/`
- default tests to replay mode in normal `go test` / CI runs
- enable re-recording only behind an explicit env var such as `WATCHTOWER_RECORD=1`
- when implemented, add a single helper script (for example `hack/rerecord-contract-tests.sh`) so cassette refresh is one command rather than a manual sequence
- redact or normalize auth headers, OAuth parameters, cookies, timestamps, and request IDs before saving cassettes so diffs stay reviewable

## Contributor readiness

- `README.md` — up-to-date with current architecture and commands
- `CONTRIBUTING.md` — synced with hexagonal layout, dependency rules, and architecture guidelines
- `AGENTS.md` — Launchpad API quirks for AI agent consumers
- `arch-go.yml` + `.golangci.yml` — machine-enforced boundaries (zero manual review burden)

## CLI cache types

The CLI `cache sync|clear|status` subcommands support the following cache types:

- `git` — local git repo mirrors
- `packages-index` — APT package sources
- `upstream-repos` — upstream OpenStack repos
- `bugs` — bug/task caches from forges (Launchpad, etc.)
- `excuses` — normalized migration excuses from Ubuntu/Debian tracker feeds

All five types are wired through `internal/adapter/primary/cli/cache.go` and rendered
via `internal/adapter/primary/cli/output.go`.

## Excuses integration

Watchtower now includes a first packaging-focused integration for migration excuses:

- **CLI**:
  - `packages excuses list`
  - `packages excuses show <package>`
  - `cache sync excuses`
  - `cache clear excuses`
- **API**:
  - `GET /api/v1/packages/excuses`
  - `GET /api/v1/packages/excuses/{name}`
  - `POST /api/v1/cache/sync/excuses`
- **Providers**:
  - `ubuntu` → `update_excuses.yaml.xz` + `update_excuses_by_team.yaml`
  - `debian` → `excuses.yaml`

The implementation keeps excuses in a dedicated cache domain (`ExcusesCache`) rather
than overloading `DistroCache`. Raw tracker files are stored on disk and normalized
records are indexed in bbolt for fast list/show queries. For Ubuntu, cache sync also
fetches the companion `update_excuses_by_team.yaml` feed so `packages excuses list
--team ...` and `packages excuses show ...` can surface ownership information.

Excuses sources are now intended to live under each distro config (`packages.distros.*.excuses`),
with `provider`, `url`, and optional `team_url`. Compression is auto-detected from the
downloaded payload instead of being configured explicitly.

## Bug cache architecture

The bug cache uses a **decorator pattern**: `CachedBugTracker` wraps the live `BugTracker`
port and a `BugCache` port. On miss, the decorator falls through to the live tracker and
back-fills the cache; on hit, it serves directly from the local bbolt store.

- **Port**: `internal/core/port/bugcache.go` — defines the `BugCache` interface
- **Adapter**: `internal/adapter/secondary/bugcache/` — bbolt-backed implementation
- **Decorator**: `internal/adapter/secondary/bugcache/tracker.go` — `CachedBugTracker`

## Test coverage

### Bug cache (`internal/adapter/secondary/bugcache/`)

- `cache_test.go` — tests for the bbolt storage layer (`Cache`): store/get bugs, store/list tasks, filtering, last-sync round-trip, remove, remove-all, status, and cache-dir.
- `tracker_test.go` — tests for the `CachedBugTracker` decorator: cache-miss fallback, post-sync cache hits, write-through status updates, type delegation, and pass-through for GetProjectSeries/GetProject.

## ProjectBuilder series support

`ProjectBuilder` (`internal/core/service/build/project_builder.go`) now carries
series-aware fields alongside the original code-project metadata:

| Field                 | Purpose                                                       |
|-----------------------|---------------------------------------------------------------|
| `LPProject`           | LP project for recipe operations (may differ from `Project`) |
| `Series`              | Series this project builds for (e.g. `["2024.1", "2025.1"]`) |
| `DevFocus`            | Development-focus series (e.g. `"2025.1"`)                   |
| `OfficialCodehosting` | Whether the project uses LP's official code mirror           |

`RecipeProject()` helper returns `LPProject` when set, falling back to `Project`.

All callers in `Trigger()`, `assessRecipe()`, `executeAction()`, `List()`,
`Download()`, and `Cleanup()` use `pb.RecipeProject()`.

## RepoManager port

The `port.RepoManager` interface (`internal/core/port/build.go`) abstracts the
temporary git repo / branch lifecycle on Launchpad:

1. `GetCurrentUser(ctx)` — returns the authenticated LP username via `client.Me`.
2. `GetDefaultRepo(ctx, projectName)` — returns the default git repo self-link and default branch for a project.
3. `GetOrCreateProject(ctx, owner)` — ensures a `-sunbeam-remote-build` project exists.
4. `GetOrCreateRepo(ctx, owner, project, repoName)` — ensures a git repo exists.
5. `GetGitRef(ctx, repoSelfLink, refPath)` — fetches a git ref.
6. `WaitForGitRef(ctx, repoSelfLink, refPath, timeout)` — polls until a ref appears.

The sole implementation lives in `internal/adapter/secondary/launchpad/repo_manager.go`.

## ArtifactStrategy series-aware naming

`ArtifactStrategy` (`internal/core/service/build/strategy.go`) now exposes two
series-aware helpers used by callers that create or look up recipes:

| Method              | Signature                                              | Behaviour                                                                 |
|---------------------|--------------------------------------------------------|---------------------------------------------------------------------------|
| `OfficialRecipeName`| `(artifactName, series, devFocus string) string`       | Returns `artifactName` for the dev-focus series; `artifactName-series` otherwise |
| `BranchForSeries`   | `(series, devFocus, defaultBranch string) string`      | Returns `defaultBranch` for the dev-focus series; `stable/<series>` otherwise    |

All three concrete strategies (`RockStrategy`, `CharmStrategy`, `SnapStrategy`)
share the same implementation today. Individual strategies can override the
behaviour independently in the future.

## Config: build settings

Per-project build configuration is described in the
[Build system design → Configuration](#configuration) section above.

## Terminology: projects, artifacts, and recipes

The build system uses three distinct concepts:

| Term         | Scope        | Description                                                                     |
|--------------|--------------|---------------------------------------------------------------------------------|
| **Project**  | User-facing  | Top-level entity configured in `watchtower.yaml` (e.g. `ubuntu-openstack-rocks`)|
| **Artifact** | User-facing  | A buildable unit within a project (e.g. `keystone`, `nova-consolidated`)        |
| **Recipe**   | LP internal  | A Launchpad object created to build an artifact; includes prefix/SHA/series info|

**User-facing surfaces** (CLI positional args, config YAML, API input fields, client options)
use "artifact" terminology. **Internal implementation** (RecipeBuilder port, LP API calls,
recipe prefix/name filtering, output table `RECIPE` column) uses "recipe" because it refers
to the LP object directly.

CLI examples:
- `build trigger <project> [artifacts...]` — request builds for specific artifacts
- `build list [projects...]` — list builds (output shows recipe names in RECIPE column)
- `build download <project> [artifacts...]` — download build results
- `build cleanup [projects...]` — delete LP recipe objects (explicitly about recipes)

## Build system design

The build system supports two distinct modes: **local** (development/testing) and
**remote** (official builds).

### Local mode (`--source local`)
- **All git + LP setup runs in the CLI adapter** (`internal/adapter/primary/cli/build.go`),
  before any API call. The service and API never see filesystem paths.
- The CLI resolves the LP owner from the authenticated user via `repoManager.GetCurrentUser()`.
- Pushes local git HEAD to a temporary LP repo/branch (both `main` and `tmp-<sha>`).
- Computes temp recipe names, build paths, and git ref links locally.
- Calls the API with pre-resolved LP resource identifiers:
  `RepoSelfLink`, `GitRefLinks`, `BuildPaths`, `LPProject`, `Owner`.
- The service receives these and creates recipes / requests builds — no git operations.
- The `port.GitClient` dependency has been removed from the build service.

### Remote mode (`--source remote`)
- Uses the project's official LP git repo (code mirror) discovered via
  `repoManager.GetDefaultRepo(ctx, projectName)`.
- Creates official recipes on a per-series basis:
  - **Dev-focus series**: recipe named `<artifact>`, built from the default branch.
  - **Other series**: recipe named `<artifact>-<series>`, built from `stable/<series>`.
- `ArtifactStrategy.OfficialRecipeName(artifactName, series, devFocus)` and
  `ArtifactStrategy.BranchForSeries(series, devFocus, defaultBranch)` encapsulate
  the naming and branch logic.
- If no series are configured, all recipes use the default branch.
- Build paths use the artifact name (without series suffix).
- When `OfficialCodehosting` is false (legacy): expects recipes to already exist;
  no git repo resolution or recipe creation is performed.

### Owner resolution
1. `opts.Owner` (CLI flag `--owner`) takes precedence.
2. Falls back to `pb.Owner` from config.
3. **Local mode only (CLI)**: if still empty, resolves via `repoManager.GetCurrentUser()`.
4. Returns an error if owner is still empty.

### Configuration

Per-project build settings live in `ProjectBuildConfig`:

| Field                 | YAML key                | Purpose                                                          |
|-----------------------|-------------------------|------------------------------------------------------------------|
| `Owner`               | `owner`                 | LP owner for recipe operations (optional for local-only builds)  |
| `Artifacts`           | `artifacts`             | Explicit artifact names to build                                 |
| `PrepareCommand`      | `prepare_command`       | Shell command run before each build                              |
| `OfficialCodehosting` | `official_codehosting`  | When true, use LP's default git repo for remote builds           |
| `LPProject`           | `lp_project`            | LP project name for recipe ops (defaults to code.project)        |

`build.owner` is only required when `official_codehosting` is true.
For local-only builds the owner is resolved at runtime via the LP `Me()` API.

### `executeAction` details
- Accepts per-recipe `gitRefLink` and `buildPath` parameters.
- Recipe creation is gated on having valid git ref info (not just source mode).
- Uses `pb.RecipeProject()` for all LP project references.

### Build service test coverage

- `strategy_test.go` — tests for all three strategies (`RockStrategy`, `CharmStrategy`,
  `SnapStrategy`): `ArtifactType`, `MetadataFileName`, `BuildPath`, `ParsePlatforms`,
  `TempRecipeName`, `OfficialRecipeName`, and `BranchForSeries`.
- `service_test.go` — tests for `Trigger()`:
  - Remote mode: request-builds, all-succeeded, retry-failed, monitor-active, create-recipe,
    official-repo series expansion, failure without `OfficialCodehosting`, multiple recipes.
  - Pre-resolved mode: full pipeline with pre-resolved LP resources, owner override.
  - `ProjectBuilder.RecipeProject()` fallback logic.
  - `List()`: active-only, all-builds, project filter, graceful degradation, sorting.

### Build listing and download modes

Both `build list` and `build download` share the same discovery parameters:

| Flag            | Purpose                                               |
|-----------------|-------------------------------------------------------|
| `--source`      | `remote` (default) or `local`                         |
| `--sha`         | Narrow prefix to a specific commit (`<prefix><sha>-`) |
| `--prefix`      | Recipe name prefix (default `tmp-build-`)             |
| `--owner`       | Override LP owner                                     |
| `--project`     | Filter by project name (also positional args)         |
| `--artifacts-dir` | (download only) Output directory                    |

**Listing modes:**

1. **Remote** (`--source remote`, default): Lists builds for configured project recipes.
2. **Local with SHA** (`--source local --sha <commit>`): Discovers recipes matching
   `<prefix><sha>-` via `findByOwner`, narrowing to an exact commit.
3. **Local with prefix** (`--source local` without `--sha`): Discovers all recipes matching
   `--prefix` (default `tmp-build-`) via `findByOwner`.

**Download** uses the same discovery logic: in local mode it resolves the LP owner and
project automatically, discovers recipes by prefix, and downloads artifacts from all
succeeded builds.

The prefix-based discovery is implemented via:
- `RecipeBuilder.ListRecipesByOwner(ctx, owner)` port method
- LP's `findByOwner` web-service operation on `+rock-recipes`, `+charm-recipes`, `+snaps`
- Client-side filtering by prefix and LP project in `Service.List()` / `Service.Download()`

### Remaining follow-ups

These are still the main gaps before TUI and MCP work:

- Launchpad auth now has an application/API surface, but it is still Launchpad-only; future work should extend the same model to GitHub/Gerrit when authenticated workflows are needed
- long-running operations now have an initial reusable in-memory async/progress/event foundation via `internal/core/service/operation` plus `internal/adapter/secondary/operationstore`
- `internal/app` should remain the composition root (wiring config, caches, clients, and services), not become the runtime API for every frontend
- API/CLI now adopt that foundation through `internal/adapter/primary/frontend`, with async build trigger + project sync wrappers and `/api/v1/operations` inspection/cancel endpoints; MCP/TUI still need to adopt the same model
- TUI/MCP will likely want the new dedicated frontend facade layer on top of the core services to become their main entrypoint, exposing frontend-friendly workflows rather than raw service-by-service access
- that facade would be the right place for cross-cutting concerns that frontends need but core services should not own directly: auth/session state, progress/events, async orchestration, cancellation, and view-oriented aggregation
