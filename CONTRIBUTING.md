# Contributing to Sunbeam Watchtower

## Prerequisites

- Go 1.24+ (see `go.mod` for exact version)
- Git

## Getting started

```bash
git clone https://github.com/gboutry/sunbeam-watchtower.git
cd sunbeam-watchtower
go build ./...
go test ./...
```

## Project structure

```
cmd/watchtower/          Entry point
internal/
├── adapter/             Concrete implementations of port interfaces
│   ├── git/             go-git backed GitClient
│   ├── gitcache/        Local bare-clone cache for commit history
│   └── launchpad/       Launchpad recipe builders and repo manager
├── cli/                 Cobra commands, factory wiring, output rendering
├── config/              Config structs, loading, validation
├── pkg/
│   ├── forge/v1/        Forge implementations (GitHub, Gerrit, Launchpad)
│   └── launchpad/v1/    Raw Launchpad REST API client
├── port/                Interface definitions (hexagonal architecture)
└── service/             Business logic
    ├── bug/             Bug aggregation across trackers
    ├── build/           Build triggering, listing, downloading
    ├── commit/          Commit aggregation across sources
    └── review/          Merge request aggregation
```

The project follows **hexagonal architecture** (ports and adapters):
- `internal/port/` defines all interfaces
- `internal/adapter/` provides concrete implementations
- `internal/service/` contains business logic that depends only on port interfaces
- `internal/cli/` wires everything together via factory functions

## Building

```bash
# Build the binary
go build -o watchtower ./cmd/watchtower

# Build with version info
go build -ldflags "-X github.com/gboutry/sunbeam-watchtower/internal/cli.Version=v1.0.0" -o watchtower ./cmd/watchtower
```

## Running tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/config/...
go test ./internal/adapter/gitcache/...
go test ./internal/service/commit/...

# With verbose output
go test -v ./internal/service/commit/...

# With race detection
go test -race ./...
```

## Debug logging

Watchtower uses Go's `log/slog` for structured logging. All log output goes to stderr.

### Enabling debug logs

```bash
watchtower --verbose <command>
```

This sets the log level to `DEBUG`, which surfaces detailed tracing across every layer:

| Layer | What gets logged |
|-------|-----------------|
| **CLI commands** | Command start with filter parameters, result counts |
| **Factory** | Forge client configuration, commit source wiring, cache directory resolution |
| **Services** | Per-project query/skip decisions, fetch counts, aggregation totals, warnings on errors |
| **Git cache** | Clone/fetch operations, path resolution, commit read counts |
| **Git client** | Every operation: push, remote management, HEAD resolution |
| **LP repo manager** | Project/repo creation or reuse, git ref polling |
| **LP HTTP client** | Every HTTP request method and URL |

### Log levels

| Level | When used |
|-------|-----------|
| `DEBUG` | Detailed tracing: what's being queried, skipped, resolved, counted |
| `INFO` | Significant operations: cloning a repo, creating LP resources |
| `WARN` | Non-fatal issues: fetch failures, missing auth, per-project errors |
| `ERROR` | Not used directly — errors are returned up the call stack |

### Adding logging to new code

Every constructor that does I/O should accept a `*slog.Logger` and default to a no-op logger when nil:

```go
func NewMyAdapter(logger *slog.Logger) *MyAdapter {
    if logger == nil {
        logger = slog.New(slog.NewTextHandler(io.Discard, nil))
    }
    return &MyAdapter{logger: logger}
}
```

Use structured key-value pairs:

```go
s.logger.Debug("querying project",
    "project", name,
    "forge", forgeType,
)

s.logger.Warn("project query failed",
    "project", name,
    "error", err,
)
```

Guidelines:
- **Debug** at entry and exit of significant operations, with relevant parameters and result counts
- **Info** only for operations that change state (cloning, creating resources)
- **Warn** for non-fatal errors that the user should know about
- Never log sensitive data (credentials, tokens)
- Always use structured key-value pairs, never string formatting in log messages

## Architecture guidelines

### Adding a new forge

1. Implement the `forge.Forge` interface in `internal/pkg/forge/v1/`
2. Add a new `ForgeType` constant
3. Wire it in `internal/cli/factory.go` `buildForgeClients()`
4. Add config validation in `internal/config/config.go`
5. Update `CloneURL()` and `CommitURL()` in `internal/config/giturl.go`

### Adding a new service

1. Define any new port interfaces in `internal/port/`
2. Create the service in `internal/service/<name>/`
3. Accept port interfaces (not concrete types) in the constructor
4. Accept a `*slog.Logger` with nil-safe default
5. Wire it in `internal/cli/factory.go`
6. Add CLI commands in `internal/cli/<name>.go`
7. Register in `internal/cli/root.go`

### Adding a new cache type

The cache directory at `$XDG_CACHE_HOME/sunbeam-watchtower/` is designed for extensibility:

```
sunbeam-watchtower/
├── repos/       ← git repo cache (existing)
├── indices/     ← future: distro package indices
└── <other>/     ← future cache types
```

1. Define a port interface in `internal/port/`
2. Implement the adapter in `internal/adapter/<name>/`
3. Integrate via factory functions in `internal/cli/factory.go`

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep error messages lowercase, without trailing punctuation
- Prefer returning errors over logging + continuing
- Use table-driven tests
- No external test frameworks — use the standard `testing` package

## Submitting changes

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes with tests
4. Ensure `go build ./...`, `go test ./...`, and `go vet ./...` pass
5. Submit a pull request against `main`
