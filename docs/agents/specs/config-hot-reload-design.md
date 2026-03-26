# Config Hot-Reload Design

## Goal

Allow the persistent watchtower server to pick up `watchtower.yaml` changes without restarting. Three reload triggers: automatic file watching (fsnotify), SIGHUP signal, and a manual API endpoint / CLI command.

## Context

The server loads `watchtower.yaml` once at startup via `config.Load()`. The `App` struct holds the config as a public field. Most services that consume config do so per-request (build factories, package sources, release tracking, etc.), but two `sync.Once` services (Telemetry, TeamSyncService) read config at init time and cache the result.

Restarting the server to pick up a one-line config change (e.g. adding a `channels:` map to a build block) is a poor developer experience, especially during iterative testing.

## Design

### Config Reload Core

Add a `sync.RWMutex` to `App` protecting the `Config` field.

**New methods on App:**

- `GetConfig() *config.Config` — takes read lock, returns current config pointer. New code should use this accessor instead of reading `App.Config` directly.
- `ReloadConfig(path string) error` — loads config via `config.Load(path)`, validates it, takes write lock, swaps `App.Config`, releases lock. On parse or validation failure, returns the error and keeps the old config. On success, logs an INFO message with project names before/after so the operator can see what changed.

The `App.Config` field is made private (`config`). All existing `a.Config` reads are migrated to `a.GetConfig()` in the same change. This ensures the mutex is always used and prevents accidental unprotected access.

### File Watcher

A `ConfigWatcher` struct in `internal/adapter/primary/runtime/`:

- Takes: config file path, reload callback `func(path string) error`, logger
- `Start()`: creates an `fsnotify.Watcher` on the config file, launches a goroutine
- On Write/Create/Rename events: debounce (100ms) to coalesce editor save patterns (vim writes temp file then renames), then call the reload callback
- On watcher errors: log and continue watching
- `Stop()`: close the fsnotify watcher, stop the goroutine

The watcher is decoupled from `App` — it takes a callback, making it testable and reusable.

Dependency: `github.com/fsnotify/fsnotify` added to `go.mod`.

### Reload Triggers

All three triggers call the same `App.ReloadConfig(path)` — no divergent code paths.

**1. Automatic (fsnotify):** The `serve` command starts `ConfigWatcher` alongside the HTTP server. The watcher's callback is `app.ReloadConfig`.

**2. SIGHUP signal:** The `serve` command registers `signal.Notify` for `syscall.SIGHUP`. When received, calls `app.ReloadConfig(configPath)` and logs success/failure.

**3. API endpoint:** `POST /api/v1/config/reload` calls `app.ReloadConfig(configPath)`. Returns success or the validation error. The CLI exposes `watchtower config reload` which calls this endpoint.

### Error Handling

When config reload fails (parse error, validation error):
- Old config is kept — the server continues operating normally
- The error is logged at WARN level
- The API endpoint returns the error so `watchtower config reload` can display it
- The file watcher continues watching for the next change

### What Reloads and What Doesn't

**Reloads immediately** (per-request factories that re-read `App.Config` on each call):

- Build recipe builders and build service (projects, artifacts, channels, prepare_command, official_codehosting)
- Package sources, commit sources, upstream provider
- Review projects, forge clients, bug trackers
- Release tracking, excuses sources
- Config API endpoint (`GET /api/v1/config`)

**Does NOT reload (phase 2, deferred):**

- **Telemetry/OTel collectors** — initialized via `sync.Once`, reads `Config.OTel`. Would need `sync.Once` reset and collector teardown/re-init.
- **TeamSyncService** — initialized via `sync.Once`, reads `Config.Collaborators`. Same reset requirement.

**Not affected by config** (path-based or credential-based, no reload needed):

- Credential stores (Launchpad, GitHub, Snap Store, Charmhub) — use file/keyring paths, not config fields
- Cache stores (distro, git, bug, review, excuses, release) — use filesystem paths

This boundary must be documented in AGENTS.md so future `sync.Once` services are aware of the reload constraint.

## Files Affected

**New files:**
- `internal/adapter/primary/runtime/config_watcher.go` — ConfigWatcher with fsnotify
- `internal/adapter/primary/runtime/config_watcher_test.go`

**Modified:**
- `internal/app/app.go` — make `Config` private, add `configMu`, `GetConfig()`, `ReloadConfig()`
- All call sites reading `a.Config` — migrated to `a.GetConfig()` (full migration, not gradual)
- `internal/adapter/primary/cli/serve.go` — start watcher, register SIGHUP handler
- `internal/adapter/primary/api/config.go` — add `POST /api/v1/config/reload`
- `internal/adapter/primary/cli/config_cmd.go` — add `config reload` subcommand
- `pkg/client/` — add `ConfigReload()` client method
- `go.mod` / `go.sum` — add `fsnotify` dependency
- `AGENTS.md` — document reload boundary (what reloads, what doesn't, sync.Once constraint)

## Architecture Compliance

- `ConfigWatcher` lives in the runtime layer (server lifecycle infrastructure)
- No core port changes — reload is a runtime/adapter concern
- `App.ReloadConfig` is the single mutation point; all triggers converge on it
- The fsnotify dependency is confined to one file in the runtime adapter
- Action catalog: `config.reload` is a write action (mutates server state)
