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
- shared frontend facade for auth, operations, project, build, cache, package, bug, review, commit, release, and config workflows
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

## Current Gaps

These are the main known gaps that still matter:

- Launchpad and GitHub auth are implemented, but the same authenticated-flow model is not yet extended to other forges such as Gerrit
- the TUI now covers the main read-only workflows and cache/project/bug mutation entrypoints, but it still does not expose direct build retry/cancel flows
- the `Packages` and `Commits` TUI tabs now have read-only submodes, but deeper workflow actions remain CLI/API-first
- some forge/package bootstrap paths in `internal/app` still contain logic that should continue moving into narrower builders/factories
- some tests still have environment-sensitive assumptions and need further hardening

## Active Roadmap

### Near term

- continue shrinking the remaining forge/package bootstrap paths in `internal/app` so it stays a composition root instead of absorbing feature logic

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
