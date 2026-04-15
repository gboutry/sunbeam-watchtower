# Sunbeam Watchtower — Plan

## Goal

Watchtower is a unified tracking and management tool for Canonical OpenStack (Sunbeam) across development, release, and maintenance. It brings package, bug, build, review, release, and cache state behind one application surface so operators do not need to coordinate across multiple external systems manually.

The long-term design constraints remain:

- keep the core forge-agnostic and push Sunbeam-specific behavior to adapters
- preserve the hexagonal boundary between primary adapters, core ports/services, and secondary adapters
- support server-first operation, with CLI, TUI, and future MCP surfaces reusing the same application/runtime seams

## Architecture Snapshot

### Main packages

- Entrypoints: `cmd/watchtower`, `cmd/watchtower-tui`
- Primary adapters: `internal/adapter/primary/api`, `cli`, `frontend`, `runtime`, `tui`
- Composition root: `internal/app`
- Core interfaces: `internal/core/port`
- Core services: `internal/core/service/*`
- Secondary adapters: `internal/adapter/secondary/*`
- Public client/contracts: `pkg/client`, `pkg/dto/v1`, `pkg/distro/v1`, `pkg/forge/v1`, `pkg/launchpad/v1`

### Enforced boundaries

- `internal/core/port/*` contains interfaces only
- `internal/adapter/*` packages are implementation packages and must not define interfaces
- primary adapters do not import secondary adapters directly
- public `pkg/*` packages do not import `internal/*`
- `internal/app` remains the wiring layer, not a grab-bag frontend API

### Shared frontend/runtime model

- CLI, TUI, and API reuse the frontend workflow layer under `internal/adapter/primary/frontend`
- shared bootstrap/runtime concerns live in `internal/adapter/primary/runtime`
- stateful frontend code should prefer the shared facade/runtime seams over raw `pkg/client` usage

## Runtime Model

Watchtower is now explicitly server-first.

- Persistent server mode is the durable coordination boundary for auth, async operations, and multi-client workflows.
- Embedded mode exists for convenience, but it is ephemeral and must not pretend to offer durable state across invocations.
- Split workflows are allowed: local preparation happens on the client side, durable execution happens on the server side.
- Local filesystem paths stay local. The server should receive prepared references, not direct local-path access.

## Current State

The following are implemented and should be treated as the current baseline:

- strict hexagonal layout under `internal/adapter/*`, `internal/core/*`, and `pkg/*`
- HTTP API for auth, builds, releases, cache, packages, bugs, reviews, commits, config, and project sync
- shared frontend facade for auth, operations, project, build, cache, package, bug, review, commit, release, config, and team workflows
- shared runtime/bootstrap layer for env defaults, logger setup, config loading, embedded server startup, local daemon management, and target resolution
- TUI sessions now prefer an already running local daemon on startup and only fall back to an embedded session when no persistent daemon is available
- shared runtime session target policies for CLI and TUI, covering embedded, discovered-daemon, and persistent-daemon resolution
- shared action access catalog and runtime access mode plumbing for CLI, TUI, and future MCP surfaces
- backend-neutral prepared-build contract using canonical `target_ref`, `repository_ref`, and `recipes` fields
- narrower internal/app build/runtime factory helpers for recipe builders, repo managers, auth-flow stores, and operation stores
- shared release target presentation/filtering for CLI and TUI, including base-aware revision formatting and config-driven visibility profiles
- release target filtering normalizes snap `coreXX` bases against Ubuntu release generations so shared target profiles work across snaps and charms
- release tracking keeps same-name snap and charm artifacts as distinct cached/listed entries and requires type narrowing only for ambiguous release-detail lookups
- bug cache sync and bug `since` filtering treat created-or-modified task activity as in-scope, with Launchpad task activity timestamps derived from the latest task state transition and incremental bug sync using a small modified-time overlap to recover recent closed-task transitions
- bug list supports group-aware `--merge` output driven by explicit `bug_groups` config, collapsing same-forge bug IDs within one shared tracker group under that group's common project label
- review browsing is now cache-first across CLI/API/TUI, backed by a dedicated review cache that stores summary rows plus cached comments/files/diff detail for open and recently updated closed reviews
- durable GitHub auth is now implemented via device flow, with aggregated auth status, provider-specific CLI/TUI flows, env/file credential precedence, and automatic authenticated GitHub clients for GitHub-backed reads when credentials are present
- local daemon lifecycle commands and explicit runtime resolution order
- Launchpad auth flows with durable server-side coordination
- durable operations surface for async workflows
- release tracking and release cache support for snaps and charms
- cache-first OpenTelemetry support confined to `internal/adapter/secondary/otel`
- initial `watchtower-tui` shell with `Dashboard`, `Builds`, `Releases`, `Packages`, `Bugs`, `Reviews`, `Commits`, and `Projects`
- TUI meta surfaces for auth, operations, cache, logs, server/about, and shortcuts
- TUI read-only workflow tabs for packages, bugs, reviews, commits, and config-backed project inspection, including filter forms and list/detail layouts
- dense TUI list rows for reviews, bugs, and commits now clamp and truncate long text so narrow panes do not wrap or misalign adjacent rows
- the TUI bug list also strips repeated Launchpad-style `Bug #... in ...:` prefixes from row titles when the row already shows project and bug ID
- the CLI bug list now applies the same cleanup, stripping repeated Launchpad-style `Bug #... in ...:` prefixes and surrounding quotes from bug row titles while leaving bug detail output unchanged
- TUI filters now use centered scrollable modals with wrapped shortcut help, `Enter`-to-apply behavior, `Ctrl+R` reset, mode-specific Packages filter forms, a visible Packages submenu, and picker-style enum fields instead of free-text autocomplete for small known value sets
- `watchtower.yaml` can now declare TUI startup presets, including `tui.default_pane`, per-pane default filters, and explicit startup modes for Packages and Commits
- the TUI now exposes meta-overlay mutation workflows for cache sync/clear plus project and bug sync, while keeping those write actions out of the read-only content tabs
- the TUI form system now supports reusable multi-select fields with `Space` toggles and visual-range `v` + `gg`/`G` motions for known finite multi-value inputs, and cache sync/clear for git/bugs/reviews now accepts multiple projects end to end instead of single-project bodies only
- team collaborator sync is wired end-to-end (CLI/API/TUI): the core sync service diffs LP team members against store collaborators with dry-run/apply semantics; manifest discovery scans local worktrees for `snapcraft.yaml`/`charmcraft.yaml` in both single-artifact and monorepo layouts; the `TeamSyncService()` lazy factory adapts the LP client to `port.LaunchpadTeamProvider` with email override support for members with hidden emails; the server-side `TeamServerWorkflow.Sync` builds sync targets from config and delegates to the service; the async `Facade.StartTeamSync` runs the real sync inside the operation runner with per-artifact progress reporting; the Charmhub collaborator adapter uses the documented publisher endpoints (`GET /v1/charm/{name}/collaborators` for listing and `POST /v1/charm/{name}/collaborators/invites` with a `{"invites":[{"email":...}]}` body for invitations); Snap Store per-snap collaborator management is **intentionally unsupported** — the snapstore adapter returns the typed `port.ErrCollaboratorsUnsupported` sentinel (there is no public per-snap collaborator API, only the closed-source `dashboard.snapcraft.io/snaps/<name>/collaboration/` UI; see `docs/agents/specs/snapstore-collaborator-api.md`), the core teamsync service tolerates this by marking the artifact `Unsupported=true` with `UnsupportedURL` pointing at the dashboard, and the CLI/TUI/API surfaces render "snap `<name>`: manage collaborators at `<dashboard_url>`" instead of failing the sync for sibling charm artifacts; the Charmhub adapter decodes non-2xx responses through a shared helper (`internal/adapter/secondary/charmhub/errors.go`) that reads a bounded body, parses both documented shapes (`{"error-list":[{code,message}]}` and `{"error":{code,message}}`), and falls back to raw body text, producing a typed `HTTPError` that unwraps to `port.ErrStoreAuthExpired` on 401 or on any code in the auth set (`macaroon-needs-refresh`, `macaroon-expired`, `permission-required`, ...); `charmhub.ErrUnauthorized` is an alias of `port.ErrStoreAuthExpired`; the teamsync service branches on `port.ErrStoreAuthExpired` and marks the artifact `AuthExpired=true` with an `AuthHint` pointing at the matching `watchtower auth <store> login` command (no Error set, sibling artifacts keep syncing); CLI output renders `auth: <project>/<store> (<type>): authentication expired — run "watchtower auth charmhub login"`
- Snap Store and Charmhub authentication uses client-side httpbakery macaroon discharge: the server requests a root macaroon from the store and returns it to the client; the client runs `httpbakery.DischargeAll` locally (so the browser opens on the user's machine) via the Candid identity provider; the discharged credential is sent back to the server for storage; `SNAPCRAFT_STORE_CREDENTIALS` and `CHARMCRAFT_AUTH` environment variables take precedence over file-cached credentials; login/logout is exposed through CLI, API, and TUI
- Charmhub login also runs a server-side **exchange** step before persisting: `auth.Service.SaveCharmhubCredential` calls `CharmhubAuthenticator.ExchangeToken` against `POST https://api.charmhub.io/v1/tokens/exchange` with the discharged slice re-encoded as the `Macaroons` header (`base64.StdEncoding` of the JSON array of macaroon dicts, matching craft-store's convention) and stores the returned short-lived publisher token as `rec.Macaroon`; without this exchange every `/v1/charm/...` call returns `HTTP 400: api-error: Invalid macaroon`; `port.CharmhubAuthenticator` is now its own interface (not an alias of `StoreAuthenticator`) so the snap-store flow — which has no exchange step — stays on the plain `BeginAuth` surface; see `docs/agents/specs/charmhub-auth-flow.md`
- The persisted Charmhub record keeps **both** the long-lived discharged bundle (`rec.DischargedBundle`) and the short-lived exchanged publisher token (`rec.Macaroon`) so the token can be silently re-exchanged on expiry without a new browser discharge; `port.CharmhubCredentialStore.Save(ctx, dischargedBundle, exchangedMacaroon)` now takes both inputs and the file-backed store serialises them into `charmhub-credentials.json` as `{"macaroon":..., "discharged_bundle":...}` (legacy records missing the bundle still load — the refresh path surfaces `port.ErrCharmhubReloginRequired` for them); the new `port.CharmhubCredentialProvider` port serves the current token on every publisher call and exposes `Refresh(ctx)` that re-runs the exchange with the stored bundle and persists the new token; the Charmhub `CollaboratorManager` consumes that provider and, on any `port.ErrStoreAuthExpired` response from `/v1/charm/...`, calls `Refresh` and retries the original request exactly once — a second auth-class failure (or a refresh failure) surfaces `port.ErrCharmhubReloginRequired` (wraps `ErrStoreAuthExpired`) telling the operator to run `watchtower auth charmhub login` instead of looping
- excuses autopkgtest rendering parses britney HTML at the parser level (`excusescache/html.go`) using `golang.org/x/net/html` tokenizer with a whitelist approach and control-character sanitization; the parser extracts per-architecture structured data (package, architecture, status, log URL) into `ExcuseAutopkgtest` fields and strips HTML from `Messages`; both CLI and TUI render grouped autopkgtest results per triggering package with colored statuses; the CLI supports OSC 8 terminal hyperlinks for log URLs
- TUI UX improvements: cursor auto-tracks viewport on j/k navigation so the selected item stays visible; per-view scroll positions are preserved across tab switches; loading indicators show "Loading..." instead of empty-state text while data is being fetched; `esc` key works in global context to reset scroll; `gg` pending state shows visual feedback in the status bar; GitHub auth keybinding changed from `g` to `h` to avoid conflicting with `gg` vim navigation; help text corrected for submode cycling keys
- excuses list supports `--set` and `--blocked-by-set` filters that resolve named package sets from config (`packages.sets`) to filter excuses by migrating package membership or blocked-by relationship; propagated through CLI, API, HTTP client, frontend workflow, DTO, cache, and TUI filter form with autocomplete suggestions
- excuses sync uses provider-owned URLs: Ubuntu and Debian feed URLs are hardcoded in their respective providers, removing the need for configurable `excuses` blocks in `watchtower.yaml`; the cache performs HEAD requests to check `Last-Modified` timestamps before downloading, skipping the entire download+parse+store cycle when feeds haven't changed; each feed (main and team) is checked independently
- build trigger now supports branch-based local preparation: `--local-path` creates a named temp branch (`tmp-<prefix>-<sha>`), optionally runs a configured `prepare_command` (e.g. `./repository.py prepare` for charm monorepos), commits the result, and pushes to the user's personal LP repo; if the temp branch already exists on LP, the prepare+push cycle is skipped entirely (branch acts as cache key); the `--source` flag has been removed from CLI — `--local-path` presence determines local mode
- build cleanup now deletes both temporary recipes and their source branches via LP API; `Service.Cleanup` uses `ListRecipesByOwner` with prefix filtering (fixing a bug where it iterated configured artifacts which missed temp recipe names); branch cleanup resolves the user's LP repo and deletes refs matching the prefix pattern
- `GitClient` port extended with branch, commit, and reset operations (`CreateBranch`, `CheckoutBranch`, `CurrentBranch`, `DeleteLocalBranch`, `AddAll`, `Commit`, `ResetHard`), all implemented via go-git
- `RepoManager` port extended with `ListBranches` and `DeleteGitRef` for branch lifecycle management via LP API
- `CommandRunner` port and `ShellCommandRunner` implementation handle prepare-command execution on the frontend side
- `CleanupResult` type propagated through build service, API, HTTP client, and frontend workflow layers, reporting both deleted recipes and deleted branches
- config hot-reload: `App.Config` is private behind `GetConfig()` (read-locked) and `ReloadConfig(path)` (write-lock swap); the persistent server watches `watchtower.yaml` via fsnotify `ConfigWatcher`, handles SIGHUP for manual reload, and exposes `POST /api/v1/config/reload`; CLI `config reload` command calls the endpoint; per-request services pick up changes immediately; `sync.Once` services (Telemetry, TeamSyncService) require server restart
- minimal client config: `Session.Config` is now a `*ConfigResolver` that lazily resolves configuration from a local file, remote server, or both; `NewSession` performs tolerant config loading (missing config file is not an error), resolves `server_address`/`server_token` from config when flags/env are absent, and skips `App` creation entirely for remote and daemon targets; `NewClientWithToken` injects bearer auth into every request; the server enforces token auth via middleware on TCP listeners; embedded mode still requires a local config file; CLI and TUI `session.Config` call sites use `session.Config.LocalConfig()` for backward compatibility
- TUI Builds tab now supports retry (`R`) and cancel (`X`) keybindings on selected builds, a cleanup form (`C`) with dry-run/apply, and auto-refresh polling (30s tick) while non-terminal builds are visible; retry/cancel are backed by `POST /api/v1/builds/retry` and `/cancel` endpoints with `build.retry` and `build.cancel` action IDs
- `internal/app` bootstrap cleanup: `BuildPackageSources` filter logic extracted into a stateless pure function (`buildPackageSources`) with characterization tests locking down the 3-state backport contract (nil/empty/named); release helper functions (YAML parsers, track resolution, dedup, skip-artifact logic) moved from `release_bootstrap.go` to `release_helpers.go`; forge client builders, bug tracker assembly, telemetry snapshot methods, and team-sync wiring intentionally kept in `internal/app` as legitimate composition-root concerns per adversarial architecture review
- local build preparation now preserves nested monorepo layouts: `ArtifactStrategy.DiscoverRecipes` returns `[]DiscoveredRecipe{Name, RelPath}` instead of bare names, and `CharmStrategy`/`RockStrategy` walk the `charms/`/`rocks/` subtrees with `filepath.WalkDir` (stopping at the first metadata hit per subtree) so charms like `charms/storage/foo/charmcraft.yaml` report `RelPath = "charms/storage/foo"`. `LocalBuildPreparer.PrepareTrigger` always runs discovery and uses the discovered `RelPath` directly as `PreparedBuildRecipe.BuildPath` — there is no name-based fallback path anymore. When the caller passes an explicit artifact list (e.g. `watchtower build trigger sunbeam cinder-volume-hitachi`), discovery is still run and the list is used as a filter against the discovered names; any name missing from the local repo fails the call with a clear error so typos or stale CI inputs surface loudly instead of silently building the wrong directory. `SnapStrategy` still reports single-artifact repos with an empty `RelPath`, which Launchpad treats as "build from repo root".
- `build trigger` supports per-build automatic retry via `--retry N` where `N` is the total number of attempts per individual build (default `1` = no retry). The flag requires `--wait` and is rejected with `--async`; validation is duplicated at the CLI and API layers (`/api/v1/builds/trigger` returns 422 on `retry_count > 1 && !wait`, `/api/v1/builds/trigger/async` returns 422 on `retry_count > 1`). `Service.waitForBuilds` owns the full retry lifecycle: it tracks a per-`b.SelfLink` budget, issues `pb.Builder.RetryBuild` on terminal failures with retries remaining, and refreshes the snapshot after issuing retries so a timeout mid-retry returns the post-retry pending state rather than the stale pre-retry failure state. To avoid double-counting with the existing one-shot `ActionRetryFailed` path in `executeAction`, the eager retry is suppressed when `RetryCount > 1` — the wait loop becomes the single retry owner. The legacy one-shot retry behavior is preserved unchanged for callers that don't set `RetryCount`, so existing clients (CLI without `--retry`, TUI, API callers omitting `retry_count`) see no behavior change. Retry events are logged at `Info` level with `recipe`/`build`/`arch`/`attempt`/`max_attempts` fields for visibility in both normal and verbose output.
- snap builds auto-detect target architectures from `snapcraft.yaml`. The local preparer reads `snap/snapcraft.yaml` (or root `snapcraft.yaml`), reuses `SnapStrategy.ParsePlatforms` to extract the platform set, and attaches it to `PreparedBuildRecipe.Processors`. The server threads processors through `executeAction`: on `ActionCreateRecipe` they are passed to `CreateSnap` (LP `processors` form param); on `ActionRequestBuilds` `RecipeBuilder.SetProcessors` is called before `RequestBuilds` so an updated `snapcraft.yaml` platforms list re-syncs the existing LP snap. The contract is snap-only: rock and charm builders implement `SetProcessors` as a no-op because LP `rock_recipe`/`charm_recipe` carry architectures per-build via `requestBuilds`, not as recipe state. The LP snap `requestBuilds` operation requires `archive` and `pocket` (verified against `devel.html`), so `SnapBuilder` falls back to `/ubuntu/+archive/primary` + `Updates` and `RequestSnapBuilds` rejects empty values at the client boundary.
- artifact-manifest name extraction is now centralised in `internal/core/service/artifactdiscovery.ParseManifestName`, shared by `build.walkRecipes`/`SnapStrategy.DiscoverRecipes` and reusable by the forthcoming canonical discovery service. `walkRecipes` still walks the prepared worktree (build prepare needs in-tree state, not HEAD of a bare repo), but the leaf parser is now authoritative for the declared name; an empty name falls back to `filepath.Base(path)` to preserve existing single-directory layouts. `arch-go.yml` gained an allowlist entry for `core.service` → `core.service` imports so future cross-service sharing stays compliant.
- build prepare is now isolated in a temporary linked `git worktree` rather than running against the live repo. When `prepare_command` is set, `LocalBuildPreparer.prepareAndPush` materialises a worktree via the new `GitClient.CreateDetachedWorktree(ctx, repoPath, branch, sha) (path, cleanup, err)` port method (shells out to `git worktree add -b <branch> <path> <sha>` with fixed argv, honouring `$TMPDIR` so snap-confined invocations get the right temp location), runs the prepare command with cwd set to the worktree, force-stages its outputs via the new `GitClient.ForceAddAll(ctx, path)` (shells out to `git add -f -A`), commits, and pushes from the worktree. This closes a silent correctness hole: before this change, outputs written into conventionally-ignored paths (`build/`, `dist/`, generated tarballs) were dropped by go-git's ignore-aware staging and LP built against an incomplete tree. `DiscoverRecipes` and `snapProcessorsFromRepo` now read the prepared tree so LP and discovery see the same source. The no-`prepare_command` path is preserved byte-for-byte via the new `pushFromLocalPath` helper. The push-skip-on-rerun behaviour is preserved: when the LP ref already exists, the worktree is still materialised and prepare still runs (so discovery has a correct tree), but the push is skipped. Cleanup is deferred and idempotent (`sync.Once`): `worktree remove --force`, `branch -D`, opportunistic `worktree prune --expire now` to recover from past SIGKILLs, and `os.RemoveAll`. All internal shell-outs use `exec.CommandContext` with fixed argv; `cmdRunner`'s `sh -c` semantics remain only for user-supplied `prepare_command` values. Security scope is narrow — isolation prevents accidental inclusion of pre-existing ignored files (`.env`, IDE caches, credentials) from the live tree; it does not sandbox the prepare command itself. The git adapter also now opens every repo via a centralised `openRepo` helper that sets `PlainOpenOptions.EnableDotGitCommonDir = true`, so go-git operations resolve the `.git` gitdir pointer file in the linked worktree.

## Current Gaps

These are the main known gaps that still matter:

- Launchpad, GitHub, Snap Store, and Charmhub auth are implemented with interactive flows, but the same authenticated-flow model is not yet extended to other forges such as Gerrit
- the `Packages` and `Commits` TUI tabs now have read-only submodes, but deeper workflow actions remain CLI/API-first
- `internal/app` telemetry snapshot methods (312 lines) remain large but are intentionally kept as composition-root glue per adversarial review — they mix cache-backed and opt-in live collectors and return adapter-facing DTOs
- some tests still have environment-sensitive assumptions and need further hardening

## Active Roadmap

### Near term

- if telemetry snapshot methods grow further, extract pure reducer helpers over already-fetched DTO slices (keep orchestration in `internal/app`)
- consolidate artifact discovery: a canonical `internal/core/service/artifactdiscovery` now exists (charm/snap/rock, HEAD-tree based via a `TreeReader` seam). Team sync has been migrated to it: `TeamServerWorkflow.Sync` fans out one `SyncTarget` per discovered artifact instead of one per project (fixing `HTTP 404: Name sunbeam-charms not found` for mono-repos), honours an optional `proj.Team.SkipArtifacts` filter, and surfaces discovery errors as per-project warnings without failing the whole sync. Authorization surface: `team.sync` ActionID unchanged, but the fan-out widens reach — a single `team sync` invocation may now issue one `InviteCollaborator` call per charm in a mono-repo where previously it would fail before any store call was made. Release bootstrap and build strategy still use their ad-hoc discoverers; their migration and the removal of legacy helpers in `internal/app/release_helpers.go` and `internal/adapter/primary/frontend/team_discovery.go` are follow-up tasks.

### Frontend/runtime

- keep future frontends such as MCP on the same frontend/runtime seams now shared by CLI and TUI
- keep the shared operation access catalog authoritative so future MCP surfaces can expose read-only actions by default and require explicit override for writes
- keep release target filtering and target-aware release rendering in the shared frontend layer so CLI, TUI, and future MCP surfaces stay aligned

### TUI

- expose cache mutation and richer config inspection where the frontend/API contracts are ready
- extend the new meta-overlay mutation surfaces carefully rather than pushing write actions into every tab by default
- add direct retry/cancel workflows for builds where the server/API model is settled
- continue improving dense keyboard UX, list/detail layouts, and responsive rendering

### API and test contracts

- keep the Huma optional-field guard in place and add regression tests when request shapes change
- keep Launchpad bug-task reads aligned with the full documented `searchTasks.status` enum so default bug syncs do not silently omit task states such as `Deferred` or `Does Not Exist`
- keep Launchpad URL construction multi-value safe so repeated query keys like `status` survive request building instead of collapsing to the last value
- keep bug cache syncs best-effort but parallelize bug-detail hydration with a small bounded worker pool so cache refresh stays responsive without aggressive Launchpad fan-out
- keep review browsing cache-first so the TUI and `review list/show` do not fan out live forge calls by default, with explicit `cache sync reviews` for refresh and bounded detail hydration during sync
- keep snap release syncs requesting `channel-map,base,revision,version` so cached/listed snap targets expose base and revision metadata like charms do
- keep shared release target rendering concise by suppressing duplicate `/version` suffixes when the version equals the revision string
- keep handler-focused API tests on ephemeral runtime helpers and shared local fixtures so test speed improves without weakening dedicated persistence coverage
- continue removing host-environment assumptions from tests
- keep changed-package coverage enforcement healthy by raising tests with feature work instead of bypassing the guard

### Auth and forge expansion

- keep the current durable auth-flow model shared across Launchpad and GitHub, and extend it to other forges such as Gerrit when authenticated workflows become necessary
- keep new TUI selection-heavy workflows on the shared centered-form system so multi-select and vim-style range motions can be reused consistently instead of reimplemented per overlay

## Validation Baseline

The expected validation baseline remains:

- `go test ./...`
- `golangci-lint run ./...`
- `arch-go --color no`
- `go run ./tools/coverageguard --config .coverage-policy.yaml $(git diff --cached --name-only -- '*.go')`
- `pre-commit run --all-files`

Notes:

- architecture boundaries are mechanically enforced and should be updated intentionally, not worked around
- changed-package coverage is part of the merge contract
- host-specific failures should be treated as test-environment hardening work, not as a reason to weaken the boundary or quality guards

## Roadmap Delivery

When implementing roadmap work, each chunk must end with:

- a `PLAN.md` sync in the same chunk
- the chunk's validation commands
- one clean commit
- only then the next chunk

## Deferred Testing Note

Broad go-vcr adoption is still intentionally deferred.

If cassette-backed contract tests are added later:

- keep them small and focused on endpoints whose real payloads are hard to model with `httptest`
- prefer replay-by-default and explicit rerecording
- store cassettes under package-local `testdata/vcr/`
- normalize secrets and unstable metadata before saving cassettes
