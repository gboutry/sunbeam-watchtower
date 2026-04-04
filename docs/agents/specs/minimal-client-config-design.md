# Minimal Client Config Design

## Goal

Allow the CLI client to operate against a remote or daemon server without requiring the full configuration file locally. Read-only commands should work with zero local config when a server address is provided. Commands that need client-side resources (auth credentials, local build preparation) fail with specific, actionable error messages explaining exactly what is missing.

## Context

Today, `NewSession()` in `internal/adapter/primary/runtime/runtime.go` unconditionally calls `config.Load()` and creates an `App` instance — even when `--server` points to a remote daemon. This forces every user to maintain a complete `config.yaml` with all project, build, package, and forge definitions, duplicating what the server already has. For a team where one server instance holds the canonical config, this is impractical.

The server already exposes `GET /api/v1/config` and `POST /api/v1/config/reload`. The `ClientFacade` already supports `application=nil` (tested in `client_facade_test.go:15`). The foundation for remote config resolution exists — the session creation path just doesn't use it.

## Design

### Two-Phase Config Loading

**Phase 1 — Tolerant local load (always runs):**

`config.Load()` continues to read `~/.config/sunbeam-watchtower/config.yaml` (or the explicit `--config` path). If the file is missing or contains only client-side fields, that is acceptable. No eager `Validate()` call during session creation. The config struct is returned with zero values for absent sections.

**Phase 2 — On-demand resolution:**

When a command needs config data (projects, build settings, etc.), the `ConfigResolver` resolves it:

1. If connected to a remote/daemon server → fetch from `GET /api/v1/config` (cached per session).
2. If local config has the needed section populated → use local.
3. If neither source has it → fail with a specific error naming the missing section and suggesting remediation.

### ConfigResolver

New struct in `internal/adapter/primary/runtime/`:

```go
type ConfigResolver struct {
    local  *config.Config   // may be nil or partial
    client *client.Client   // may be nil (embedded mode)
    cached *config.Config   // server config, fetched once per session
    mu     sync.Mutex
}

func (r *ConfigResolver) Resolve(ctx context.Context) (*config.Config, error)
```

Resolution logic:

1. If `cached` is already populated → return it.
2. If `client` is set (remote/daemon mode) → call `GET /api/v1/config`, unmarshal into `*config.Config`, cache, return.
3. If only `local` is available → return it.
4. Neither → error.

No merging of local and remote config. When connected to a server, the server's config is authoritative for all server-side sections. Local-only fields (TUI preferences, `server_token`, `server_address`) are accessed directly from the local config, not through the resolver.

No cache invalidation within a session. CLI commands are short-lived. TUI refresh is out of scope.

### Session Creation Changes

**Current flow:**
```
Load config → Create App → Create LocalServerManager → Resolve target → Wire facade
```

**New flow:**
```
Attempt config load (tolerant) → Resolve target FIRST → Wire accordingly
```

Concretely in `NewSession()`:

- **Remote target (`opts.ServerAddr` set):** Skip `App` creation. Create `ClientFacade` with `application=nil`. Construct `ConfigResolver` with the remote client. `LocalServerManager` is not created.
- **Daemon target (discovered running):** Same as remote — skip `App`, use daemon's API for config.
- **Embedded target (no server):** Require local config. Create `App` as today. Validate populated sections. Fail with specific errors if the command needs sections that are absent.

`Session.Config` changes from `*config.Config` to `*ConfigResolver`. A new `Session.GetConfig(ctx)` method replaces direct field access.

When target policy is `RequirePersistent` and no daemon is running, auto-start requires the full config locally. If the local config is partial, fail with: `"cannot auto-start daemon without full configuration — provide a complete config file or connect to an existing server"`.

### Client Config Fields

The config file keeps the same format and path. No new file. Two new top-level fields:

| Field | Type | Purpose |
|-------|------|---------|
| `server_address` | `string` | Server URL, slots into the discovery chain between env var and socket discovery |
| `server_token` | `string` | Auth token for network connections to the server |

Existing fields that are client-side concerns and remain useful locally: `tui`, `otel`.

All other sections (`launchpad`, `github`, `gerrit`, `bug_groups`, `projects`, `build`, `releases`, `packages`, `collaborators`) become optional when targeting a remote server.

Backward compatibility: a full config file works exactly as today.

### Auth Tiering

Two transport-based tiers:

| Tier | Transport | Auth | Access |
|------|-----------|------|--------|
| Local | Unix socket | None (OS-level trust) | Full — all endpoints including config with secrets |
| Network | TCP/IP | `Authorization: Bearer <token>` | Full if authenticated, 401 otherwise |

**Server side:**

1. **Token source:** If `auth_token` is set in the server's config, use it (stable token for multi-machine setups). Otherwise, generate a random token at startup and write it to `~/.config/sunbeam-watchtower/server.token` with `0600` permissions. The server-side field is `auth_token` (distinct from the client-side `server_token` field — they hold the same value but are named for their respective contexts).
2. **Middleware:** New Huma middleware inspects the transport. Unix socket connections pass through. TCP connections require a valid bearer token. Invalid or missing token → HTTP 401 with body: `{"error": "authentication required"}`.
3. **Transport detection:** The server knows which listener (Unix vs TCP) accepted each request. Tag the request context at the listener level, not by inspecting peer addresses.

**Client side:**

1. Unix socket (local daemon) → no token, works as today.
2. Network (`--server http://host:port`) → reads token from `server_token` config field or `WATCHTOWER_TOKEN` env var. Passes as `Authorization: Bearer <token>` on every request.
3. Missing token on 401 → error: `"server at http://host:8472 requires authentication — set WATCHTOWER_TOKEN or add server_token to your config"`.

All-or-nothing per tier. No per-endpoint ACLs. This is a foundation for future OpenFGA integration, where the middleware hook point remains the same but the "allow" decision becomes a policy check instead of a token match.

### Error Messages

Specific, actionable errors for each failure mode:

1. **Remote server, command needs local-only feature:**
   ```
   "build trigger --local" requires local build configuration (launchpad, build sections).
   Add them to ~/.config/sunbeam-watchtower/config.yaml or omit --local to use the server's configuration.
   ```

2. **No server, minimal config, command needs projects:**
   ```
   "review list" requires project configuration.
   Either connect to a server (--server or server_address in config) or add a "projects" section to ~/.config/sunbeam-watchtower/config.yaml.
   ```

3. **Network server, no auth token:**
   ```
   Server at http://host:8472 requires authentication.
   Set WATCHTOWER_TOKEN or add "server_token" to ~/.config/sunbeam-watchtower/config.yaml.
   ```

4. **Remote server unreachable:**
   ```
   Could not fetch configuration from server at http://host:8472: connection refused.
   ```

5. **Cannot auto-start daemon with partial config:**
   ```
   Cannot auto-start daemon without full configuration.
   Provide a complete config file or connect to an existing server (--server).
   ```

No new error framework. Typed errors or sentinel errors in the runtime package. Errors bubble through existing Cobra error handling.

### Validation Changes

`config.Validate()` is called in two places only:

1. **Server startup (`serve` command):** Full validation required — all sections must be consistent.
2. **Embedded mode session creation:** Validate populated sections only.

Validation is **not** called during remote/daemon session creation. Commands that need specific config sections get validation errors from the server's API responses or from the resolver.

## Scope

### In scope

- Two-phase config loading with tolerant Phase 1
- `ConfigResolver` for on-demand server config fetching
- `NewSession()` refactor to skip App creation for remote/daemon targets
- Auth middleware with Unix socket / network token tiering
- Token generation at server startup
- `server_token` and `server_address` config fields
- `WATCHTOWER_TOKEN` env var
- Specific error messages for missing config
- Graceful degradation in embedded mode with partial config
- `Session.GetConfig(ctx)` accessor replacing direct `Session.Config` field access

### Out of scope

- OpenFGA integration (future — middleware hook point is ready)
- Per-endpoint authorization or role-based access
- Config merging (local + remote blending)
- TUI session config refresh / cache invalidation
- Client-side config file migration tooling
- mTLS or other auth mechanisms beyond bearer token
- Server-side config redaction

### Risk areas

1. **`ClientFacade` with `application=nil`:** Already tested, but workflows that pass `application` through (releases, packages, builds) must each be verified to degrade correctly rather than panic on nil dereference.

2. **Config DTO compatibility:** `GET /api/v1/config` returns `dto.Config`, not `config.Config`. The resolver needs a mapping layer, or `config.Config` needs a constructor from `dto.Config`. This is a non-trivial but mechanical mapping.

3. **Daemon auto-start with partial config:** The `RequirePersistent` + `EnsureRunning` path currently assumes a full config to spawn the daemon process. Must fail clearly with partial config rather than starting a broken daemon.
