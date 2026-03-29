# TUI Build Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add build retry/cancel/cleanup actions and auto-refresh for non-terminal builds to the TUI Builds tab.

**Architecture:** New retry/cancel API endpoints flow through client → frontend workflow → server workflow → existing `RecipeBuilder.RetryBuild`/`CancelBuild` ports. The TUI gains keybindings (`R`/`X`/`C`) for retry/cancel/cleanup and a `tea.Tick`-based auto-refresh that polls every 30s while non-terminal builds are visible.

**Tech Stack:** Go, Bubble Tea, Huma (API framework)

**Spec:** `docs/agents/specs/tui-build-integration-design.md`

---

### Task 1: Add retry/cancel action IDs to the action catalog

**Files:**
- Modify: `internal/adapter/primary/frontend/action_catalog.go`

- [ ] **Step 1: Add action ID constants**

After the existing build action IDs (`ActionBuildCleanupApply`), add:

```go
ActionBuildRetry  ActionID = "build.retry"
ActionBuildCancel ActionID = "build.cancel"
```

- [ ] **Step 2: Add action descriptors**

In the action descriptors map, add entries following the existing pattern:

```go
ActionBuildRetry:  descriptor(ActionBuildRetry, "build", "build", MutabilityWrite, LocalEffectNone, RuntimeEmbeddedOK, ExportPolicyAllowed, "Retry a failed build."),
ActionBuildCancel: descriptor(ActionBuildCancel, "build", "build", MutabilityWrite, LocalEffectNone, RuntimeEmbeddedOK, ExportPolicyAllowed, "Cancel an active build."),
```

- [ ] **Step 3: Verify and commit**

Run: `go build ./internal/adapter/primary/frontend/...`

```bash
git add internal/adapter/primary/frontend/action_catalog.go
git commit -m "feat: add build.retry and build.cancel action IDs"
```

---

### Task 2: Add retry/cancel client methods

**Files:**
- Modify: `pkg/client/builds.go`

- [ ] **Step 1: Add BuildsRetry and BuildsCancel**

```go
// BuildsRetryOptions holds the request body for retrying a build.
type BuildsRetryOptions struct {
	BuildSelfLink string `json:"build_self_link"`
	ArtifactType  string `json:"artifact_type"`
}

// BuildsRetry retries a failed build.
func (c *Client) BuildsRetry(ctx context.Context, opts BuildsRetryOptions) error {
	return c.post(ctx, "/api/v1/builds/retry", opts, nil)
}

// BuildsCancelOptions holds the request body for cancelling a build.
type BuildsCancelOptions struct {
	BuildSelfLink string `json:"build_self_link"`
	ArtifactType  string `json:"artifact_type"`
}

// BuildsCancel cancels an active build.
func (c *Client) BuildsCancel(ctx context.Context, opts BuildsCancelOptions) error {
	return c.post(ctx, "/api/v1/builds/cancel", opts, nil)
}
```

- [ ] **Step 2: Commit**

```bash
git add pkg/client/builds.go
git commit -m "feat(client): add BuildsRetry and BuildsCancel methods"
```

---

### Task 3: Add retry/cancel frontend workflow methods

**Files:**
- Modify: `internal/adapter/primary/frontend/build_workflow.go`

- [ ] **Step 1: Add Retry and Cancel to BuildWorkflow**

```go
// Retry retries a failed build.
func (w *BuildWorkflow) Retry(ctx context.Context, buildSelfLink, artifactType string) error {
	if w.client == nil {
		return errors.New("build workflow requires an API client")
	}
	return w.client.BuildsRetry(ctx, client.BuildsRetryOptions{
		BuildSelfLink: buildSelfLink,
		ArtifactType:  artifactType,
	})
}

// Cancel cancels an active build.
func (w *BuildWorkflow) Cancel(ctx context.Context, buildSelfLink, artifactType string) error {
	if w.client == nil {
		return errors.New("build workflow requires an API client")
	}
	return w.client.BuildsCancel(ctx, client.BuildsCancelOptions{
		BuildSelfLink: buildSelfLink,
		ArtifactType:  artifactType,
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/adapter/primary/frontend/build_workflow.go
git commit -m "feat(frontend): add Retry and Cancel to BuildWorkflow"
```

---

### Task 4: Add retry/cancel server workflow and API endpoints

**Files:**
- Modify: `internal/adapter/primary/frontend/build_server_workflow.go`
- Modify: `internal/adapter/primary/api/builds.go`

- [ ] **Step 1: Add Retry/Cancel to BuildServerWorkflow**

The server workflow needs to resolve the artifact type to the correct `RecipeBuilder`. The `BuildService` has a `projects` map keyed by project name, and each `ProjectBuilder` has a `Builder` (RecipeBuilder). Since the retry/cancel endpoints receive an artifact type string, the server workflow can iterate project builders to find the matching one.

Add to `build_server_workflow.go`:

```go
// Retry retries a failed build.
func (w *BuildServerWorkflow) Retry(ctx context.Context, buildSelfLink, artifactType string) error {
	builders, err := w.application.BuildRecipeBuilders()
	if err != nil {
		return err
	}
	for _, pb := range builders {
		if string(pb.Builder.ArtifactType()) == artifactType {
			return pb.Builder.RetryBuild(ctx, buildSelfLink)
		}
	}
	return fmt.Errorf("no builder for artifact type %q", artifactType)
}

// Cancel cancels an active build.
func (w *BuildServerWorkflow) Cancel(ctx context.Context, buildSelfLink, artifactType string) error {
	builders, err := w.application.BuildRecipeBuilders()
	if err != nil {
		return err
	}
	for _, pb := range builders {
		if string(pb.Builder.ArtifactType()) == artifactType {
			return pb.Builder.CancelBuild(ctx, buildSelfLink)
		}
	}
	return fmt.Errorf("no builder for artifact type %q", artifactType)
}
```

Add `"fmt"` to imports if not present.

- [ ] **Step 2: Add API endpoints**

In `builds.go`, add input/output types and register the endpoints. Follow the existing Huma pattern.

```go
// BuildsRetryInput is the request body for retrying a build.
type BuildsRetryInput struct {
	Body struct {
		BuildSelfLink string `json:"build_self_link" doc:"Self-link of the build to retry" required:"true"`
		ArtifactType  string `json:"artifact_type" doc:"Artifact type: rock, charm, or snap" required:"true"`
	}
}

// BuildsCancelInput is the request body for cancelling a build.
type BuildsCancelInput struct {
	Body struct {
		BuildSelfLink string `json:"build_self_link" doc:"Self-link of the build to cancel" required:"true"`
		ArtifactType  string `json:"artifact_type" doc:"Artifact type: rock, charm, or snap" required:"true"`
	}
}

// BuildsActionOutput is the response for retry/cancel operations.
type BuildsActionOutput struct {
	Body struct {
		Status string `json:"status" doc:"Operation result"`
	}
}
```

Register endpoints after the existing cleanup endpoint:

```go
huma.Register(api, huma.Operation{
	OperationID: "retry-build",
	Method:      http.MethodPost,
	Path:        "/api/v1/builds/retry",
	Summary:     "Retry a failed build",
	Tags:        []string{"builds"},
}, func(ctx context.Context, input *BuildsRetryInput) (*BuildsActionOutput, error) {
	if err := facade.Builds().Retry(ctx, input.Body.BuildSelfLink, input.Body.ArtifactType); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("retry failed: %v", err))
	}
	out := &BuildsActionOutput{}
	out.Body.Status = "ok"
	return out, nil
})

huma.Register(api, huma.Operation{
	OperationID: "cancel-build",
	Method:      http.MethodPost,
	Path:        "/api/v1/builds/cancel",
	Summary:     "Cancel an active build",
	Tags:        []string{"builds"},
}, func(ctx context.Context, input *BuildsCancelInput) (*BuildsActionOutput, error) {
	if err := facade.Builds().Cancel(ctx, input.Body.BuildSelfLink, input.Body.ArtifactType); err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("cancel failed: %v", err))
	}
	out := &BuildsActionOutput{}
	out.Body.Status = "ok"
	return out, nil
})
```

The `facade.Builds()` here returns the `BuildServerWorkflow`. Check how the existing endpoints reference the facade — the builds API handler uses `facade` which wraps the `ServerFacade`. Make sure the facade's `Builds()` returns a type that has `Retry` and `Cancel`. If the facade returns `*BuildServerWorkflow` directly, the methods are already available. If it returns an interface, the interface may need extending.

- [ ] **Step 3: Verify and commit**

Run: `go build ./...`

```bash
git add internal/adapter/primary/frontend/build_server_workflow.go internal/adapter/primary/api/builds.go
git commit -m "feat: add retry/cancel API endpoints and server workflow"
```

---

### Task 5: TUI auto-refresh for non-terminal builds

**Files:**
- Modify: `internal/adapter/primary/tui/model.go`

- [ ] **Step 1: Add tick type and helper functions**

Add near the other tick message types (around line 399):

```go
type tickBuildsMsg time.Time
```

Add near the other tick command functions (around line 2888):

```go
func tickBuildsCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickBuildsMsg(t) })
}

func hasNonTerminalBuilds(rows []dto.Build) bool {
	for _, b := range rows {
		if !b.State.IsTerminal() {
			return true
		}
	}
	return false
}
```

Add a `tickBuildsActive bool` field to the `rootModel` struct (near `tickOpsActive`):

```go
tickBuildsActive bool
```

- [ ] **Step 2: Handle tickBuildsMsg in Update()**

Add a case near the other tick handlers (after `tickOperationsMsg`):

```go
case tickBuildsMsg:
	if m.activeView == viewBuilds && hasNonTerminalBuilds(m.builds.rows) {
		return m, tea.Batch(loadBuildsCmd(m.session, m.builds.filters), tickBuildsCmd())
	}
	m.tickBuildsActive = false
	return m, nil
```

- [ ] **Step 3: Start tick when non-terminal builds are loaded**

In the `buildsLoadedMsg` handler (around line 539), after setting `m.builds.rows`, add:

```go
if hasNonTerminalBuilds(m.builds.rows) && !m.tickBuildsActive {
	m.tickBuildsActive = true
	cmds = append(cmds, tickBuildsCmd())
}
```

This requires changing the handler to collect commands in a `cmds` slice and return `tea.Batch(cmds...)`. Check how the existing handler returns — it may need restructuring to support returning multiple commands.

- [ ] **Step 4: Verify and commit**

Run: `go build ./cmd/watchtower-tui/...`

```bash
git add internal/adapter/primary/tui/model.go
git commit -m "feat(tui): auto-refresh Builds tab when non-terminal builds visible"
```

---

### Task 6: TUI retry/cancel keybindings

**Files:**
- Modify: `internal/adapter/primary/tui/model.go`

- [ ] **Step 1: Add message types**

```go
type buildRetryMsg struct{ err error }
type buildCancelMsg struct{ err error }
```

- [ ] **Step 2: Add command functions**

```go
func retryBuildCmd(session *runtimeadapter.Session, build dto.Build) tea.Cmd {
	return guardSessionAction(session, frontend.ActionBuildRetry, func() tea.Msg {
		err := session.Frontend.Builds().Retry(context.Background(), build.SelfLink, string(build.ArtifactType))
		return buildRetryMsg{err: err}
	})
}

func cancelBuildCmd(session *runtimeadapter.Session, build dto.Build) tea.Cmd {
	return guardSessionAction(session, frontend.ActionBuildCancel, func() tea.Msg {
		err := session.Frontend.Builds().Cancel(context.Background(), build.SelfLink, string(build.ArtifactType))
		return buildCancelMsg{err: err}
	})
}
```

- [ ] **Step 3: Add keybindings in updateGlobal()**

In the Builds tab key event handling, add cases for `R` and `X`:

```go
case "R":
	if m.activeView == viewBuilds {
		if b := selectedBuild(m.builds.rows, m.builds.index); b != nil && b.CanRetry {
			return m, retryBuildCmd(m.session, *b)
		}
	}
case "X":
	if m.activeView == viewBuilds {
		if b := selectedBuild(m.builds.rows, m.builds.index); b != nil && b.CanCancel {
			return m, cancelBuildCmd(m.session, *b)
		}
	}
```

Check that `selectedBuild` exists and returns `*dto.Build`. It's used in `renderBuildDetail`.

- [ ] **Step 4: Handle messages in Update()**

```go
case buildRetryMsg:
	if msg.err != nil {
		m.setToast(msg.err.Error(), "error")
		return m, clearToastLater()
	}
	m.setToast("Build retry requested", "success")
	return m, tea.Batch(clearToastLater(), loadBuildsCmd(m.session, m.builds.filters))
case buildCancelMsg:
	if msg.err != nil {
		m.setToast(msg.err.Error(), "error")
		return m, clearToastLater()
	}
	m.setToast("Build cancel requested", "success")
	return m, tea.Batch(clearToastLater(), loadBuildsCmd(m.session, m.builds.filters))
```

- [ ] **Step 5: Update build detail to show keybinding hints**

In `renderBuildDetail`, replace the hardcoded `"[t] trigger async build"` with context-sensitive hints:

```go
var hints []string
hints = append(hints, "[t] trigger")
if build.CanRetry {
	hints = append(hints, "[R] retry")
}
if build.CanCancel {
	hints = append(hints, "[X] cancel")
}
hints = append(hints, "[C] cleanup")
```

Use `strings.Join(hints, "  ")` as the last line of the detail panel.

- [ ] **Step 6: Verify and commit**

Run: `go build ./cmd/watchtower-tui/...`

```bash
git add internal/adapter/primary/tui/model.go
git commit -m "feat(tui): add R/X keybindings for build retry/cancel"
```

---

### Task 7: TUI cleanup form

**Files:**
- Modify: `internal/adapter/primary/tui/model.go`

- [ ] **Step 1: Add message type and overlay constant**

```go
type buildCleanupMsg struct {
	deletedRecipes  []string
	deletedBranches []string
	dryRun          bool
	err             error
}
```

Add an overlay constant if one doesn't exist for build cleanup:
```go
overlayBuildCleanup overlayKind = "buildCleanup"
```

- [ ] **Step 2: Add form constructor**

```go
func newBuildCleanupForm(session *runtimeadapter.Session) formModalModel {
	return newFormModal("Cleanup Builds", []fieldDef{
		{placeholder: "prefix", value: "tmp-build", resetValue: "tmp-build"},
		{placeholder: "owner", value: "", resetValue: ""},
		{placeholder: "dry run", value: "true", resetValue: "true", suggestions: []string{"true", "false"}, kind: fieldKindEnum},
	})
}
```

- [ ] **Step 3: Add command function**

```go
func cleanupBuildCmd(session *runtimeadapter.Session, prefix, owner string, dryRun bool) tea.Cmd {
	actionID := frontend.ActionBuildCleanupDryRun
	if !dryRun {
		actionID = frontend.ActionBuildCleanupApply
	}
	return guardSessionAction(session, actionID, func() tea.Msg {
		result, err := session.Frontend.Builds().Cleanup(context.Background(), frontend.BuildCleanupRequest{
			Prefix: prefix,
			Owner:  owner,
			DryRun: dryRun,
		})
		if err != nil {
			return buildCleanupMsg{err: err}
		}
		return buildCleanupMsg{
			deletedRecipes:  result.DeletedRecipes,
			deletedBranches: result.DeletedBranches,
			dryRun:          dryRun,
		}
	})
}
```

- [ ] **Step 4: Add C keybinding**

In `updateGlobal()`, add:

```go
case "C":
	if m.activeView == viewBuilds {
		m.buildCleanupForm = newBuildCleanupForm(m.session)
		m.overlay = overlayBuildCleanup
		m.overlayScroll = 0
		return m, nil
	}
```

Add `buildCleanupForm formModalModel` to the `rootModel` struct.

- [ ] **Step 5: Wire overlay rendering and form submission**

In the overlay rendering switch, add `overlayBuildCleanup` case that renders the form.

In the form submission handler, when `m.overlay == overlayBuildCleanup`, extract fields and call `cleanupBuildCmd`.

- [ ] **Step 6: Handle buildCleanupMsg**

```go
case buildCleanupMsg:
	m.overlay = overlayNone
	if msg.err != nil {
		m.setToast(msg.err.Error(), "error")
		return m, clearToastLater()
	}
	verb := "Deleted"
	if msg.dryRun {
		verb = "Would delete"
	}
	summary := fmt.Sprintf("%s %d recipes, %d branches", verb, len(msg.deletedRecipes), len(msg.deletedBranches))
	m.setToast(summary, "success")
	return m, tea.Batch(clearToastLater(), loadBuildsCmd(m.session, m.builds.filters))
```

- [ ] **Step 7: Verify and commit**

Run: `go build ./cmd/watchtower-tui/...`

```bash
git add internal/adapter/primary/tui/model.go
git commit -m "feat(tui): add C keybinding for build cleanup form"
```

---

### Task 8: Validation and PLAN.md sync

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -count=1
```

- [ ] **Step 2: Run pre-commit**

```bash
pre-commit run --all-files
```

- [ ] **Step 3: Update PLAN.md**

Add to Current State:
- the TUI Builds tab now supports retry (`R`) and cancel (`X`) keybindings on selected builds, a cleanup form (`C`), and auto-refresh polling (30s tick) while non-terminal builds are visible; retry/cancel are backed by `POST /api/v1/builds/retry` and `/cancel` endpoints

Remove from Current Gaps:
- the TUI retry/cancel gap entry

- [ ] **Step 4: Commit**

```bash
git add PLAN.md
git commit -m "docs: sync PLAN.md with TUI build integration"
```
