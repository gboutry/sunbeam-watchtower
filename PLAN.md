# Sunbeam Watchtower — Plan

## Architecture

Hexagonal architecture: CLI → HTTP client → API server → services → adapters/ports.

- **CLI** (`internal/cli/`): Thin Cobra wrappers. Parse flags, call `appclient`, render output. No business logic.
- **API** (`internal/api/`): Huma v2 + chi HTTP handlers. All business logic accessed through service layer.
- **App Client** (`internal/appclient/`): Typed HTTP client for CLI→server communication.
- **App** (`internal/app/`): Application wiring. Holds config, builds services, shared lifecycle.
- **Services** (`internal/service/`): Domain logic (packages, bugs, bugsync, builds, commits, reviews, projects).
- **Adapters** (`internal/adapter/`): External system integrations (git, launchpad, distrocache, gitcache).
- **Ports** (`internal/port/`): Interface definitions for adapters.

Only `auth` remains CLI-only (interactive OAuth flow).

## Package Layout

```
internal/
├── api/                        HTTP API server (Huma v2 + chi)
│   ├── server.go               Server setup, custom schemaNamer, listener config
│   ├── packages.go             /api/v1/packages/* (diff, show, list, dsc, rdepends, cache)
│   ├── bugs.go                 /api/v1/bugs/* (list, get, sync)
│   ├── reviews.go              /api/v1/reviews/* (list, get)
│   ├── commits.go              /api/v1/commits/* (list, track)
│   ├── builds.go               /api/v1/builds/* (trigger, list, download, cleanup)
│   ├── projects.go             /api/v1/projects/sync
│   ├── cache.go                /api/v1/cache/* (sync git/upstream, delete, status)
│   └── config.go               /api/v1/config
│
├── appclient/                  Typed HTTP client (CLI→server)
│   ├── client.go               Base client (unix socket or TCP), get/post/delete helpers
│   ├── packages.go             PackagesDiff, PackagesShow, PackagesList, ...
│   ├── bugs.go                 BugsList, BugsGet, BugsSync
│   ├── reviews.go              ReviewsList, ReviewsGet
│   ├── commits.go              CommitsList, CommitsTrack
│   ├── builds.go               BuildsTrigger, BuildsList, BuildsDownload, BuildsCleanup
│   ├── projects.go             ProjectsSync
│   ├── cache.go                CacheSyncGit, CacheSyncUpstream, CacheDelete, CacheStatus
│   └── config.go               ConfigShow
│
├── app/                        Application wiring
│   └── app.go                  App struct, service construction, BuildPackageSources
│
├── cli/                        Thin Cobra CLI (flags → appclient → render)
│   ├── root.go                 Global flags, embedded server lifecycle, env vars
│   ├── serve.go                `watchtower serve` (standalone HTTP server)
│   ├── packages.go             packages diff/show/list/dsc/rdepends commands
│   ├── bug.go                  bug list/get/sync commands
│   ├── review.go               review list/get commands
│   ├── commit.go               commit log/track commands
│   ├── build.go                build trigger/list/download/cleanup commands
│   ├── project.go              project sync command
│   ├── cache.go                cache sync/clear/status commands
│   ├── config_cmd.go           config show command
│   ├── auth.go                 OAuth flow (CLI-only, no API)
│   ├── output.go               Table/JSON/YAML renderers
│   └── version.go              Version command
│
├── service/                    Domain logic
│   ├── package/                Package diff, show, list, rdepends, dsc
│   ├── bug/                    Bug tracking across LP/GitHub
│   ├── bugsync/                Bug sync orchestration
│   ├── build/                  Recipe builds (trigger, list, download, cleanup)
│   ├── commit/                 Commit log and tracking
│   ├── review/                 Merge request reviews
│   └── project/                LP project sync (series, focus of development)
│
├── adapter/                    External system integrations
│   ├── git/                    Git operations
│   ├── gitcache/               Local git cache management
│   ├── distrocache/            Package index cache
│   ├── launchpad/              Launchpad API client
│   └── openstack/              Gerrit/OpenStack forge
│
├── port/                       Interface definitions
│   └── build.go                Build, Recipe, BuildRequest interfaces
│
├── config/                     YAML config parsing
└── pkg/                        Shared domain types
    ├── forge/v1/               MergeRequest, Commit, BugRef types
    ├── distro/v1/              Package, Source types
    └── launchpad/v1/           LP API types
```

## API Endpoints

All served under Huma v2 with auto-generated OpenAPI spec:
- `GET /openapi.json` — OpenAPI 3.1 spec
- `GET /docs` — Stoplight Elements interactive docs UI
- `GET /api/v1/health` — health check

### Packages
- `GET /api/v1/packages/diff/{set}` — package diff across sources
- `GET /api/v1/packages/show/{name}` — show package details
- `GET /api/v1/packages/list` — list all packages
- `GET /api/v1/packages/dsc` — find .dsc files
- `GET /api/v1/packages/rdepends/{name}` — reverse dependencies
- `GET /api/v1/packages/cache/status` — package cache status
- `POST /api/v1/packages/cache/sync` — sync package index cache

### Bugs
- `GET /api/v1/bugs` — list bugs
- `GET /api/v1/bugs/{id}` — get bug details
- `POST /api/v1/bugs/sync` — sync bugs from trackers

### Reviews
- `GET /api/v1/reviews` — list merge requests
- `GET /api/v1/reviews/{project}/{id}` — get merge request details

### Commits
- `GET /api/v1/commits` — list commits
- `GET /api/v1/commits/track` — track commits across forges

### Builds
- `POST /api/v1/builds/trigger` — trigger recipe builds
- `GET /api/v1/builds` — list builds
- `POST /api/v1/builds/download` — download build artifacts
- `POST /api/v1/builds/cleanup` — cleanup old builds

### Projects
- `POST /api/v1/projects/sync` — sync LP projects (series, focus of development)

### Cache
- `POST /api/v1/cache/sync/git` — sync git cache
- `POST /api/v1/cache/sync/upstream` — sync upstream repos
- `DELETE /api/v1/cache/{type}` — clear cache by type
- `GET /api/v1/cache/status` — cache status

### Config
- `GET /api/v1/config` — show current config

## Design Principles
- Hexagonal architecture — no contamination of concerns
- CLI only imports `appclient`, `config`, presentation helpers
- RESTful where natural (GET for reads, POST for mutations)
- JSON responses always; CLI handles table rendering
- Consistent error format: `{"title": "...", "status": N, "detail": "..."}`
- All types have `json` + `doc` tags for OpenAPI spec quality
- Custom `schemaNamer` resolves cross-package Huma type name collisions

## Roadmap

### Planned Consumers
All built on top of the HTTP API:
- **MCP Server** — Model Context Protocol server for AI agent integration
- **TUI Dashboard** — Terminal UI for real-time monitoring
- **Prometheus Exporter** — Metrics endpoint for observability

## Completed Work

### HTTP Server Refactor
- **Phase 1** ✅ — Server skeleton + packages + bugs domains
- **Phase 2** ✅ — Reviews, commits, builds, projects, cache, config domains
- `factory.go` deleted — all wiring via `app.App` → API handlers

### Earlier Features
- Config restructure, backport suite expansion, parent release inference
- Per-source filtering, 3-state backport filter, `--only-in` auto-inference
- Environment variable support (`WATCHTOWER_` prefix)
- Structured JSON/YAML output for all commands
- LP project sync (series assignment, focus of development)
