# Team Collaborator Sync

Sync Launchpad team members as collaborators on snap and charm store artifacts.

## Problem

When a team manages many snaps and charms, keeping store collaborator lists in sync with the Launchpad team roster is manual and error-prone. Members join or leave the LP team, but store artifacts drift out of sync.

## Solution

A new sync operation that compares Launchpad team membership against store collaborator lists for each discovered artifact, then reports discrepancies (dry-run) or sends invites (apply).

Rocks are excluded — they have no backing store with collaborator management.

## Configuration

A new top-level config block:

```yaml
collaborators:
  launchpad_team: "ubuntu-openstack"
```

The LP team name is global — applied to all snap and charm artifacts. Validation: must be non-empty if present. Team existence is verified at sync time, not config load.

No per-project collaborator config is needed. Artifact discovery is implicit from project `artifact_type`.

## Artifact Discovery

Discovery is a **client-side** operation. In the split-mode runtime, the CLI resolves local worktree paths and discovers artifact names before sending prepared sync targets to the server. The server never accesses local filesystem paths directly.

For each project with `artifact_type: snap` or `charm`, scan the local worktree for manifest files:

- **Snaps**: glob `snaps/**/snap/snapcraft.yaml`, fall back to `./snap/snapcraft.yaml`
- **Charms**: glob `charms/**/charmcraft.yaml`, fall back to `./charmcraft.yaml`

Each manifest's `name` YAML field is the store artifact name. The tuple `(project, artifact_type, store_name)` becomes a sync target transmitted to the sync service.

If a project's worktree is unavailable or no manifests are found, a warning is emitted and the project is skipped.

## Store Collaborator Port

A new port interface abstracts collaborator management across both stores:

```go
type StoreCollaboratorManager interface {
    ListCollaborators(ctx context.Context, storeName string) ([]StoreCollaborator, error)
    InviteCollaborator(ctx context.Context, storeName string, email string) error
}
```

DTO:

```go
type StoreCollaborator struct {
    Username    string
    Email       string
    DisplayName string
    Status      string // "accepted", "pending", "expired"
}
```

Two secondary adapter implementations:

- `internal/adapter/secondary/snapstore/collaborator.go` — Snap Store publisher API
- `internal/adapter/secondary/charmhub/collaborator.go` — Charmhub publisher API

The composition root wires the correct adapter based on `ArtifactType`. The sync service receives `map[dto.ArtifactType]StoreCollaboratorManager`.

### Store API Details

**Snap Store publisher API** (`dashboard.snapcraft.io/api/v2/`):
- `GET /snaps/{name}/collaborators` — list current collaborators (returns account email, username, permissions, status)
- `POST /snaps/{name}/collaborators` — invite by email
- Auth: macaroon-based, `Authorization: Macaroon ...` header
- Pagination: standard `page` / `page_size` query params

**Charmhub publisher API** (`api.charmhub.io/v1/`):
- `GET /charm/{name}/collaborators` — list current collaborators
- `POST /charm/{name}/collaborators` — invite by email
- Auth: macaroon-based, `Authorization: Macaroon ...` header
- Pagination: similar to Snap Store

Both APIs invite by email and return collaborator records with email, username, and invite status. The exact response shapes differ and are handled by each adapter; the port interface normalizes them into `StoreCollaborator`.

### Join Key

LP team members are matched to store collaborators by **email**. The LP `Person` struct does not directly expose email. The `preferred_email_address_link` field (a URL to an email resource) must be resolved via a follow-up `GET` to retrieve the actual address.

Users with `hide_email_addresses: true` cannot be matched. These are reported as warnings with their LP username so the operator can resolve manually.

The `TeamMember` DTO carries both username and email (when available):

```go
type TeamMember struct {
    Username string
    Email    string // empty if hidden
}
```

The `LaunchpadTeamProvider` adapter resolves emails during `GetTeamMembers` by fetching each member's `preferred_email_address_link`.

## Authentication

Both stores use Ubuntu SSO macaroon-based auth. While they share the same Canonical account system, the macaroon permissions differ per store (snap publish vs charm publish), so each store requires its own credential flow and storage.

### Snap Store

- Obtain a root macaroon from `dashboard.snapcraft.io`, discharge via Ubuntu SSO
- Interactive flow: user authorizes in browser
- Stored in keyring (file fallback), env var override: `SNAPCRAFT_STORE_CREDENTIALS`

### Charmhub

- Obtain a root macaroon from `api.charmhub.io`, discharge via Ubuntu SSO
- Interactive flow: user authorizes in browser
- Stored in keyring (file fallback), env var override: `CHARMCRAFT_AUTH`

### Integration

- New credential store ports: `SnapStoreCredentialStore`, `CharmhubCredentialStore`
- New auth workflow methods: `BeginSnapStore`, `FinalizeSnapStore`, `LogoutSnapStore` (same pattern for Charmhub)
- `dto.AuthStatus` gains `SnapStore` and `Charmhub` fields
- TUI auth overlay, CLI `auth login/logout`, and API auth endpoints all extended
- `StoreCollaboratorManager` adapters receive credentials at construction time, same as the LP client

## Sync Service

New core service at `internal/core/service/teamsync/service.go`.

### Inputs

- LP team name (from config)
- Sync targets from discovery: `[]SyncTarget{Project, ArtifactType, StoreName}`
- Mode: dry-run or apply

### Flow

1. Fetch LP team members via `LaunchpadTeamProvider.GetTeamMembers` — produces a set of emails (members with hidden emails are collected as warnings)
2. For each sync target, call `StoreCollaboratorManager.ListCollaborators` — produces a set of emails
3. Diff:
   - **Missing**: in LP team, not a store collaborator — invite (apply) or report (dry-run)
   - **Extra**: store collaborator, not in LP team — report only, never removed
   - **Pending**: already invited, not yet accepted — report, do not re-invite
4. Collect per-artifact results; store API errors are collected as warnings per-artifact (non-fatal), following the project sync error collection pattern

### Launchpad Team Provider

A thin port interface so the core service does not depend on the LP client directly:

```go
type LaunchpadTeamProvider interface {
    GetTeamMembers(ctx context.Context, teamName string) ([]TeamMember, error)
}
```

### Result DTO

```go
type TeamSyncResult struct {
    Artifacts []ArtifactSyncResult
    Warnings  []string
}

type ArtifactSyncResult struct {
    Project      string
    ArtifactType dto.ArtifactType
    StoreName    string
    Invited      []string  // emails invited (apply) or would-be-invited (dry-run)
    Extra        []string  // collaborators not in LP team
    Pending      []string  // existing pending invites
    AlreadySync  bool      // no action needed
    Error        string    // non-fatal store API error for this artifact
}
```

### Operation Kind

`OperationKindTeamSync` is defined in `pkg/dto/v1/operation.go` alongside the existing `OperationKindBuildTrigger` and `OperationKindProjectSync`.

## Frontend Integration

### Action Catalog

Two new actions in `internal/adapter/primary/frontend/action_catalog.go`, following the project sync pattern where the async/persistent-server requirement is handled dynamically by `commandNeedsPersistentServer` via the `--async` flag:

| Action | Mutability | Local Effect | Runtime | MCP |
|--------|-----------|-------------|---------|-----|
| `ActionTeamSyncDryRun` | read | none | embedded OK | allowed |
| `ActionTeamSyncApply` | write | none | embedded OK | allowed |

### CLI

Following the existing project sync convention where `--async` is a flag:

```
watchtower team sync [--dry-run] [--async] [--project <name>...]
```

### API

```
POST /api/v1/team/sync          — synchronous, returns TeamSyncResult
POST /api/v1/team/sync/async    — returns operation job
```

Both accept `{ "projects": [...], "dry_run": bool }`.

### TUI

Added to the sync overlay (`u` key) as a new option alongside project sync and bug sync. Results displayed in the same summary format.

### Async

The async variant uses the existing operation workflow with `OperationKindTeamSync`, following the project sync async pattern.

## Telemetry

Intentionally no telemetry yet. The team sync domain does not expose cache-backed or live metrics in this iteration. If sync frequency or failure rates prove operationally relevant, a cache/internal collector can be added later.

## PLAN.md

PLAN.md must be updated alongside implementation to reflect:
- New authorization surface (Snap Store and Charmhub auth)
- New sync operation and its action catalog entries

## Testing

### Unit Tests

- **Sync service**: fake `StoreCollaboratorManager` + fake `LaunchpadTeamProvider`. Scenarios: missing members, extra collaborators, pending invites, already-synced, empty team, no artifacts found, hidden-email members, per-artifact store API errors.
- **Manifest discovery**: temp directories with monorepo layouts, single-artifact projects, missing manifests, malformed YAML.
- **Config validation**: collaborators block present/absent, empty team name.

### Integration Tests

- **API handlers**: request/response shapes, dry-run vs apply, error codes (401 for missing store auth, 500 for store API errors).
- **Operation workflow**: async team sync starts, reports progress, completes.

### Architecture Tests

- Store collaborator adapters stay in `internal/adapter/secondary/`
- Core service does not import adapters directly
- Coverage guard thresholds maintained on all new/changed packages

All store interactions are behind the port interface and faked in tests. No live store API calls.
