# TUI Build Integration Design

## Goal

Expose build retry, cancel, cleanup, and live status monitoring in the TUI Builds tab. Currently builds are CLI/API-first — the TUI shows a read-only list with manual refresh and a trigger form, but has no post-trigger actions or automatic status updates.

## Context

The TUI Builds tab already supports:
- Build list with project/state/active/source filters
- Manual refresh (`r`)
- Trigger form (`t`) that queues an async build operation
- Two-column list+detail layout

Missing:
- Auto-refresh when non-terminal builds are visible
- Retry/cancel actions on individual builds
- Cleanup of temporary recipes and branches

The CLI already has all these operations (`build trigger --wait`, `build cleanup`). The server-side `RecipeBuilder` port has `RetryBuild` and `CancelBuild`. The gap is the API endpoint + client + frontend workflow + TUI wiring.

## Design

### Auto-refresh for Builds tab

When the Builds tab loads or refreshes and the result contains any non-terminal builds (Pending, Building, Cancelling), schedule a `tea.Tick` at 30 seconds. When the tick fires:

1. If the Builds tab is still the active view, re-fetch builds
2. If the new result still has non-terminal builds, schedule another tick
3. If all builds are terminal, stop polling

The tick only fires when the Builds tab is the active view. Switching away pauses polling. Switching back triggers a refresh which restarts the cycle if needed.

The trigger form uses async mode (`Async: true`) so it returns immediately with a toast "Build triggered for <project>". Builds appear in the list on the next refresh cycle or on manual `r`.

### Retry and Cancel keybindings

On the Builds list, keybindings act on the currently selected build row:

- **`R`** — retry the selected build. Only enabled when `build.CanRetry == true`. Calls retry endpoint, shows toast "Retrying build <arch> for <recipe>", refreshes list.
- **`X`** — cancel the selected build. Only enabled when `build.CanCancel == true`. Calls cancel endpoint, shows toast, refreshes list.

Both are guarded with `guardSessionAction` using their respective action IDs.

The status bar at the bottom of the Builds tab shows context-sensitive hints based on the selected build's state: `"R: retry"` when CanRetry, `"X: cancel"` when CanCancel.

### Cleanup form

Keybinding **`C`** on the Builds tab opens a form modal with:

- **prefix** — text, default `"tmp-build"`
- **owner** — text, default from session LP user
- **dry-run** — boolean toggle, default `true`

On submit, calls the cleanup endpoint. Shows result as toast:
- Dry-run: "Would delete N recipes, M branches"
- Apply: "Deleted N recipes, M branches"

Dry-run defaults to true to prevent accidental deletion.

### New API endpoints

- `POST /api/v1/builds/retry` — body: `{"build_self_link": "<link>"}` — retries a failed build
- `POST /api/v1/builds/cancel` — body: `{"build_self_link": "<link>"}` — cancels an active build

Both return `{"status": "ok"}` on success or an error. The server resolves the artifact type from the build self_link path and delegates to the appropriate `RecipeBuilder.RetryBuild` / `CancelBuild`.

### New action catalog entries

| Action ID | Mutability | Local Effect | Runtime | Export | Summary |
|-----------|-----------|--------------|---------|--------|---------|
| `build.retry` | Write | None | EmbeddedOK | Allowed | Retry a failed build |
| `build.cancel` | Write | None | EmbeddedOK | Allowed | Cancel an active build |

### New client and frontend methods

**Client** (`pkg/client/builds.go`):
- `BuildsRetry(ctx, selfLink string) error`
- `BuildsCancel(ctx, selfLink string) error`

**Frontend workflow** (`build_workflow.go`):
- `Retry(ctx, selfLink string) error`
- `Cancel(ctx, selfLink string) error`

**Server workflow** (`build_server_workflow.go`):
- `Retry(ctx, selfLink string) error`
- `Cancel(ctx, selfLink string) error`

The server workflow determines which `RecipeBuilder` to use by accepting an `artifact_type` field in the request body alongside `build_self_link`. The TUI has this information from the `dto.Build.ArtifactType` field on the selected build row.

## Files Affected

**Modified (API/frontend):**
- `internal/adapter/primary/api/builds.go` — add retry/cancel endpoints
- `pkg/client/builds.go` — add `BuildsRetry`, `BuildsCancel`
- `internal/adapter/primary/frontend/build_workflow.go` — add `Retry`, `Cancel`
- `internal/adapter/primary/frontend/build_server_workflow.go` — add `Retry`, `Cancel`
- `internal/adapter/primary/frontend/action_catalog.go` — add `build.retry`, `build.cancel`

**Modified (TUI):**
- `internal/adapter/primary/tui/model.go` — auto-refresh tick, retry/cancel/cleanup keybindings, cleanup form, status bar hints, new message types

## Architecture Compliance

- Retry/cancel flow goes through frontend workflow → client → API → server workflow → RecipeBuilder port — same layering as all other mutations
- TUI actions guarded by `guardSessionAction` with proper action IDs
- No direct adapter imports in TUI — everything through the frontend facade
- Auto-refresh uses bubbletea's `tea.Tick` command pattern, no goroutines or background polling outside the tea runtime
