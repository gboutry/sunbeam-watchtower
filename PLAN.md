# HTTP Server Refactor Plan

## Phase 1: Complete ✅
Server skeleton + packages + bugs domains migrated.

## Phase 2: Complete ✅
All remaining domains migrated to HTTP API: reviews, commits, builds, projects, cache, config.
- `factory.go` deleted — all wiring via `app.App` → API handlers
- Custom Huma `schemaNamer` resolves cross-package type name collisions
- Only `auth` remains CLI-only (interactive OAuth flow)

## New Package Layout
```
internal/
├── api/                        ← NEW: HTTP API server
│   ├── server.go               ← Huma app setup, chi router, listener config
│   ├── middleware.go            ← Logging, error mapping, request ID
│   ├── packages.go             ← /api/v1/packages/* handlers
│   ├── bugs.go                 ← /api/v1/bugs/* handlers
│   └── types.go                ← Shared request/response types (OpenAPI-tagged)
│
├── appclient/                  ← NEW: HTTP client for CLI→server
│   ├── client.go               ← HTTP client (unix socket or TCP), JSON codec
│   ├── packages.go             ← Typed methods: Diff(), Show(), List(), Rdepends(), Dsc()
│   └── bugs.go                 ← Typed methods: List(), Get(), Sync()
│
├── app/                        ← NEW: Application wiring (extracted from cli/factory.go)
│   └── app.go                  ← App struct: holds config, builds services, shared lifecycle
│
├── cli/                        ← REFACTORED: thin cobra wrapper calling appclient
│   ├── root.go                 ← Global flags + server address, env vars
│   ├── packages.go             ← Flag parsing → appclient.Packages*() → render output
│   ├── bugs.go                 ← Flag parsing → appclient.Bugs*() → render output
│   ├── output.go               ← Table/JSON/YAML renderers (stays here)
│   └── ...
```

## Phase 1 Todos

### 1. server-skeleton
Install huma v2 + chi. Create `internal/api/server.go` with:
- `NewServer(app *app.App, opts ServerOptions) *Server`
- `ServerOptions{ListenAddr, UnixSocket}`
- Chi router with huma adapter
- `/openapi.json` endpoint (auto from huma)
- Health check at `/api/v1/health`
- Graceful shutdown

### 2. app-wiring
Extract factory logic from `cli/factory.go` into `internal/app/app.go`:
- `App` struct holding config, logger, lazy-initialized services
- `NewApp(cfg *config.Config, logger *slog.Logger) *App`
- Methods: `PackageService()`, `BugService()`, `BugSyncService()`, etc.
- `BuildPackageSources(distros, releases, suites, backports)` — moved from cli
- `Close()` for cleanup

### 3. api-packages
Huma handlers in `internal/api/packages.go`:
- `GET /api/v1/packages/diff/{set}` → service.Diff()
- `GET /api/v1/packages/show/{name}` → service.Show()
- `GET /api/v1/packages/list` → service.List()
- `GET /api/v1/packages/dsc` → service.FindDsc()
- `GET /api/v1/packages/rdepends/{name}` → service.ReverseDepends()
- `GET /api/v1/packages/cache/status` → service.CacheStatus()
- `POST /api/v1/packages/cache/sync` → service.UpdateCache()
Request/response types with huma tags for OpenAPI generation.

### 4. api-bugs
Huma handlers in `internal/api/bugs.go`:
- `GET /api/v1/bugs` → bug.Service.List()
- `GET /api/v1/bugs/{id}` → bug.Service.Get()
- `POST /api/v1/bugs/sync` → bugsync.Service.Sync()
Request/response types with huma tags.

### 5. appclient
`internal/appclient/client.go`:
- `NewClient(addr string)` — supports `unix:///path` and `http://host:port`
- JSON request/response codec
- Typed methods matching API endpoints

### 6. cli-packages-refactor
Refactor `internal/cli/packages.go`:
- Remove `buildPackageSources()` and factory calls
- Each command's RunE: parse flags → build request → `appclient.PackagesDiff(req)` → render response
- Keep output.go renderers unchanged

### 7. cli-bugs-refactor
Refactor `internal/cli/bug.go`:
- Remove `buildBugTrackers()` factory calls
- Each command's RunE: parse flags → `appclient.BugsList(req)` → render response

### 8. serve-command
New `watchtower serve` command:
- Starts the HTTP server (foreground)
- `--listen` flag: `unix:///tmp/watchtower.sock` (default) or `tcp://0.0.0.0:8080`
- Loads config, creates App, starts server

### 9. cli-server-integration
- CLI auto-starts server in background if not running (for UX)
- Or: CLI requires `--server` flag / `WATCHTOWER_SERVER` env var
- Connect via unix socket by default

### 10. verify-openapi
- Run server, fetch `/openapi.json`
- Validate spec is complete and correct
- Ensure `go test ./...` passes
- Ensure `go vet ./...` passes

## Dependency Order
```
server-skeleton ─┐
app-wiring ──────┼→ api-packages ─→ appclient ─→ cli-packages-refactor ─┐
                 └→ api-bugs ─────→ appclient ─→ cli-bugs-refactor ─────┼→ serve-command → cli-server-integration → verify-openapi
```

## API Design Principles
- RESTful where natural (GET for reads, POST for mutations)
- Query params for filters (distro, release, suite, backport, etc.)
- JSON responses always (no content negotiation needed server-side; CLI handles table rendering)
- Consistent error format: `{"title": "...", "status": 404, "detail": "..."}`
- All types have `json` + `doc` tags for OpenAPI spec quality

---

## Previous Plan (Complete ✅)
<details>
<summary>Releases Restructure (completed)</summary>

### Suite expansion rules (distro-level)
- `release` → `<release-name>`, `updates` → `<release-name>-updates`, `proposed` → `<release-name>-proposed`, `backports` → `<release-name>-backports`

### Backport suite expansion rules
- `release` → `<release-name>`, `updates` → `releaseName-updates/backportName`, `proposed` → `releaseName-proposed/backportName`, default → literal

### Features delivered
- Config restructure, backport suite expansion, parent release inference, per-source filtering
- 3-state backport filter, --only-in auto-inference, env var support
- Structured JSON/YAML output for all commands
</details>
