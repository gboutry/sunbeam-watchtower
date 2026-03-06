# Sunbeam Watchtower — Plan

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
- `GET /api/v1/packages/*` / `POST /api/v1/packages/cache/sync`
- `GET /api/v1/bugs*` / `POST /api/v1/bugs/sync`
- `GET /api/v1/reviews*`
- `GET /api/v1/commits*`
- `POST /api/v1/builds/*` / `GET /api/v1/builds`
- `POST /api/v1/projects/sync`
- `POST /api/v1/cache/sync/git` / `POST /api/v1/cache/sync/upstream` / `POST /api/v1/cache/sync/bugs`
- `DELETE /api/v1/cache/{type}` (git, packages-index, upstream-repos, bugs)
- `GET /api/v1/cache/status`
- `GET /api/v1/config`

## Recent refactor outcomes

- migrated the old `internal/api`, `internal/cli`, `internal/service`, and `internal/port` layout into the new `internal/adapter/*` and `internal/core/*` split
- moved reusable client and contract packages to root `pkg/`
- introduced public config DTOs under `pkg/dto/v1`, so `pkg/client` no longer leaks `internal/config`
- removed the remaining core-service boundary leak by making bug sync consume `port.CommitSource` instead of commit-service types
- preserved existing command and API behavior while tightening architecture linting

## Validation

The refactor is currently validated by all of the following:

- `go test ./...`
- `golangci-lint run ./...`
- `arch-go --color no`
- `pre-commit run --all-files`

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

All four types are wired through `internal/adapter/primary/cli/cache.go` and rendered
via `internal/adapter/primary/cli/output.go`.

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

## Build system design

The build system supports two distinct modes: **local** (development/testing) and
**remote** (official builds).

### Local mode (`--source local`)
- Resolves the LP owner from the authenticated user via `repoManager.GetCurrentUser()`
  when no explicit owner is configured.
- Pushes local git HEAD to a temporary LP repo/branch.
- All recipes share the same git ref.
- Recipe names are rewritten to temp names via `Strategy.TempRecipeName`.
- Build paths use the original artifact name (not the temp recipe name).

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

### Owner resolution (source-aware)
1. `opts.Owner` takes precedence.
2. Falls back to `pb.Owner` from config.
3. **Local mode only**: if still empty, resolves via `repoManager.GetCurrentUser()`.
4. Returns an error if owner is still empty.

### Configuration

Per-project build settings live in `ProjectBuildConfig`:

| Field                 | YAML key                | Purpose                                                          |
|-----------------------|-------------------------|------------------------------------------------------------------|
| `Owner`               | `owner`                 | LP owner for recipe operations (optional for local-only builds)  |
| `Recipes`             | `recipes`               | Explicit recipe names to build                                   |
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
  - Local mode: full pipeline (push + create + request), owner resolution via `GetCurrentUser`.
  - `ProjectBuilder.RecipeProject()` fallback logic.
  - `List()`: active-only, all-builds, project filter, graceful degradation, sorting.

## Remaining follow-ups

These are still the main gaps before TUI and MCP work:

- auth is still CLI-driven rather than application-surface driven
- long-running operations do not yet expose reusable async/progress/event primitives
- `internal/app` is still the shared composition root; TUI/MCP will likely want a dedicated application facade on top of the core services
