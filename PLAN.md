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
- `POST /api/v1/cache/*` / `DELETE /api/v1/cache/{type}` / `GET /api/v1/cache/status`
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

## Remaining follow-ups

These are still the main gaps before TUI and MCP work:

- auth is still CLI-driven rather than application-surface driven
- long-running operations do not yet expose reusable async/progress/event primitives
- `internal/app` is still the shared composition root; TUI/MCP will likely want a dedicated application facade on top of the core services
