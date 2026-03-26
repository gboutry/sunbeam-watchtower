# Config Hot-Reload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow the persistent watchtower server to pick up watchtower.yaml changes without restarting, via fsnotify file watching, SIGHUP signal, and a manual API endpoint.

**Architecture:** Make `App.Config` private with `GetConfig()` accessor protected by `sync.RWMutex`. Add `ReloadConfig()` method for atomic config swap. Add `ConfigWatcher` (fsnotify) in the runtime layer, SIGHUP handler in serve command, and `POST /api/v1/config/reload` endpoint. Full migration of all 43 `a.Config` call sites to `a.GetConfig()`.

**Tech Stack:** Go, fsnotify, sync.RWMutex, signal.Notify

**Spec:** `docs/agents/specs/config-hot-reload-design.md`

---

### Task 1: Add fsnotify dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add fsnotify**

```bash
go get github.com/fsnotify/fsnotify
```

- [ ] **Step 2: Tidy**

```bash
go mod tidy
```

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add fsnotify for config file watching"
```

---

### Task 2: Make App.Config private, add GetConfig() and ReloadConfig()

This is the core change. Make the `Config` field private, add a `sync.RWMutex`, and provide `GetConfig()` / `ReloadConfig()` methods.

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`

- [ ] **Step 1: Update App struct and constructors**

In `internal/app/app.go`:

1. Rename the field from `Config *config.Config` to `config *config.Config` (lowercase)
2. Add `configMu sync.RWMutex` field
3. Add `configPath string` field (stored for reload)
4. Add `GetConfig()` method:

```go
// GetConfig returns the current configuration, safe for concurrent access.
func (a *App) GetConfig() *config.Config {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.config
}
```

5. Add `ReloadConfig()` method:

```go
// ReloadConfig loads configuration from the given path and atomically
// replaces the current config. On failure, the old config is kept and
// the error is returned.
func (a *App) ReloadConfig(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	a.configMu.Lock()
	old := a.config
	a.config = cfg
	a.configPath = path
	a.configMu.Unlock()

	a.Logger.Info("configuration reloaded",
		"path", path,
		"projects_before", projectNames(old),
		"projects_after", projectNames(cfg),
	)
	return nil
}

func projectNames(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, len(cfg.Projects))
	for i, p := range cfg.Projects {
		names[i] = p.Name
	}
	return names
}
```

6. Update constructors to use lowercase field:

```go
func NewAppWithOptions(cfg *config.Config, logger *slog.Logger, opts Options) *App {
	// ... existing logic ...
	return &App{config: cfg, Logger: logger, runtimeMode: mode}
}
```

7. Add `"fmt"` to imports.

- [ ] **Step 2: Update app_test.go**

The test at line 143 checks `a.Config != cfg` — update to `a.GetConfig() != cfg`.

- [ ] **Step 3: Verify the app package compiles (it won't — call sites break)**

Run: `go build ./internal/app/...`
Expected: PASS (internal references use `a.config` directly within the package)

Actually, the internal references like `a.Config` in bootstrap files are within the `app` package and will need updating too. Since Go allows lowercase access within the same package, they'll still compile — but for correctness they should use `GetConfig()` so the mutex is respected. This is done in Task 3.

- [ ] **Step 4: Commit**

```bash
git add internal/app/app.go internal/app/app_test.go
git commit -m "feat(app): make Config private, add GetConfig() and ReloadConfig() with RWMutex"
```

---

### Task 3: Migrate internal/app/ bootstrap files to GetConfig()

All bootstrap files in the `app` package access `a.config` (lowercase, same package). They should use `a.GetConfig()` for mutex correctness.

**Files:**
- Modify: `internal/app/build_bootstrap.go`
- Modify: `internal/app/build_factories.go`
- Modify: `internal/app/project_bootstrap.go`
- Modify: `internal/app/review_bootstrap.go`
- Modify: `internal/app/runtime_bootstrap.go`
- Modify: `internal/app/telemetry_bootstrap.go`
- Modify: `internal/app/release_bootstrap.go`
- Modify: `internal/app/package_bootstrap.go`
- Modify: `internal/app/cache_bootstrap.go`
- Modify: `internal/app/teamsync_bootstrap.go`

- [ ] **Step 1: Migrate each file**

For each file, replace all `a.Config` and `a.config` reads with `a.GetConfig()`. The pattern in most files is to capture once at the top of the method:

```go
// Before:
cfg := a.Config
// After:
cfg := a.GetConfig()
```

For nil checks like `a.Config == nil`, replace with `a.GetConfig() == nil`.

For chained access like `a.Config.Packages.Sets[name]`, replace with:
```go
cfg := a.GetConfig()
// then use cfg.Packages.Sets[name]
```

The `telemetry_bootstrap.go` has `s.app.Config.Projects` — replace with `s.app.GetConfig().Projects`.

- [ ] **Step 2: Verify app package compiles and tests pass**

Run: `go build ./internal/app/... && go test ./internal/app/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/app/
git commit -m "refactor(app): migrate all bootstrap files to GetConfig() accessor"
```

---

### Task 4: Migrate adapter packages to GetConfig()

All adapter packages access `application.Config` or `w.application.Config` on the `*app.App` pointer. These must use `GetConfig()`.

**Files:**
- Modify: `internal/adapter/primary/api/packages.go`
- Modify: `internal/adapter/primary/api/packages_excuses.go`
- Modify: `internal/adapter/primary/api/cache.go`
- Modify: `internal/adapter/primary/frontend/config_server_workflow.go`
- Modify: `internal/adapter/primary/frontend/build_server_workflow.go`
- Modify: `internal/adapter/primary/frontend/bug_server_workflow.go`
- Modify: `internal/adapter/primary/frontend/release_client_workflow.go`
- Modify: `internal/adapter/primary/frontend/team_server_workflow.go`
- Modify: `internal/adapter/primary/frontend/packages_client_workflow.go`

- [ ] **Step 1: Migrate each file**

Replace all `application.Config` / `w.application.Config` with `application.GetConfig()` / `w.application.GetConfig()`.

Specific patterns:

**api/packages.go:148** — `application.Config.Packages.Sets[setName]` → `application.GetConfig().Packages.Sets[setName]`

**api/packages_excuses.go:75,82** — same pattern for `application.Config.Packages.Sets[input.Set]`

**api/cache.go:176,240,399,451** — `cfg := application.Config` → `cfg := application.GetConfig()`
**api/cache.go:512,517** — `application.Config == nil` → `application.GetConfig() == nil`, `application.Config.Projects` → `application.GetConfig().Projects`

**frontend/config_server_workflow.go:27,30** — `w.application.Config` → `w.application.GetConfig()`

**frontend/build_server_workflow.go:77,80** — same pattern

**frontend/bug_server_workflow.go:90** — `w.application.Config.Projects` → `w.application.GetConfig().Projects`

**frontend/release_client_workflow.go:101** — `w.application.Config` → `w.application.GetConfig()`

**frontend/team_server_workflow.go:36** — `cfg := w.application.Config` → `cfg := w.application.GetConfig()`

**frontend/packages_client_workflow.go:290** — `w.application.Config` → `w.application.GetConfig()`

- [ ] **Step 2: Verify full build and tests**

Run: `go build ./... && go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/
git commit -m "refactor: migrate all adapter packages to App.GetConfig() accessor"
```

---

### Task 5: Migrate CLI root.go

The CLI root accesses `opts.config` (internal) and creates App instances.

**Files:**
- Modify: `internal/adapter/primary/cli/root.go`

- [ ] **Step 1: Update config assignment to Session.Config**

Line 125: `opts.config = session.Config` — `Session.Config` is a `*config.Config` field on the Session struct. This is separate from `App.Config`. Check if Session.Config also needs migration, or if it's a plain field.

Read `Session` struct to verify. If it's `Config *config.Config` on Session (not App), it stays as-is — it's not behind a mutex.

- [ ] **Step 2: Verify**

Run: `go build ./internal/adapter/primary/cli/...`
Expected: PASS

- [ ] **Step 3: Commit (if changes needed)**

```bash
git add internal/adapter/primary/cli/root.go
git commit -m "refactor(cli): update root.go for private App.Config"
```

---

### Task 6: Add ConfigWatcher

**Files:**
- Create: `internal/adapter/primary/runtime/config_watcher.go`
- Create: `internal/adapter/primary/runtime/config_watcher_test.go`

- [ ] **Step 1: Write test**

```go
package runtime

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigWatcher_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("v1"), 0644)

	var reloaded atomic.Int32
	w, err := NewConfigWatcher(path, func(p string) error {
		reloaded.Add(1)
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("NewConfigWatcher: %v", err)
	}
	defer w.Stop()

	// Modify the file.
	time.Sleep(200 * time.Millisecond)
	os.WriteFile(path, []byte("v2"), 0644)
	time.Sleep(500 * time.Millisecond)

	if reloaded.Load() == 0 {
		t.Error("expected reload callback to be called")
	}
}

func TestConfigWatcher_CallbackError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("v1"), 0644)

	w, err := NewConfigWatcher(path, func(p string) error {
		return fmt.Errorf("bad config")
	}, nil)
	if err != nil {
		t.Fatalf("NewConfigWatcher: %v", err)
	}
	defer w.Stop()

	// Modify — callback error should not crash, watcher continues.
	time.Sleep(200 * time.Millisecond)
	os.WriteFile(path, []byte("v2"), 0644)
	time.Sleep(500 * time.Millisecond)
	// No crash = pass.
}
```

Add `"fmt"` to imports for the second test.

- [ ] **Step 2: Implement ConfigWatcher**

```go
package runtime

import (
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches a config file and calls a reload function on changes.
type ConfigWatcher struct {
	watcher *fsnotify.Watcher
	stop    chan struct{}
	wg      sync.WaitGroup
	logger  *slog.Logger
}

// NewConfigWatcher creates and starts a watcher on the given config file path.
// The reload function is called with the file path when changes are detected.
// Changes are debounced (200ms) to coalesce editor save patterns.
func NewConfigWatcher(path string, reload func(string) error, logger *slog.Logger) (*ConfigWatcher, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch the directory (not the file) to handle editors that
	// delete+recreate the file (e.g. vim).
	dir := filepath.Dir(path)
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}

	cw := &ConfigWatcher{
		watcher: w,
		stop:    make(chan struct{}),
		logger:  logger,
	}

	base := filepath.Base(path)
	cw.wg.Add(1)
	go func() {
		defer cw.wg.Done()
		cw.run(base, path, reload)
	}()

	return cw, nil
}

func (cw *ConfigWatcher) run(base, fullPath string, reload func(string) error) {
	var debounce *time.Timer

	for {
		select {
		case <-cw.stop:
			if debounce != nil {
				debounce.Stop()
			}
			return

		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			if filepath.Base(event.Name) != base {
				continue
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
				continue
			}

			// Debounce: reset timer on each event.
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(200*time.Millisecond, func() {
				cw.logger.Info("config file changed, reloading", "path", fullPath)
				if err := reload(fullPath); err != nil {
					cw.logger.Warn("config reload failed", "error", err)
				}
			})

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logger.Warn("config watcher error", "error", err)
		}
	}
}

// Stop shuts down the watcher and waits for the goroutine to exit.
func (cw *ConfigWatcher) Stop() {
	close(cw.stop)
	cw.watcher.Close()
	cw.wg.Wait()
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/adapter/primary/runtime/... -run TestConfigWatcher -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/primary/runtime/config_watcher.go internal/adapter/primary/runtime/config_watcher_test.go
git commit -m "feat: add ConfigWatcher with fsnotify for automatic config reload"
```

---

### Task 7: Add config.reload action to catalog

**Files:**
- Modify: `internal/adapter/primary/frontend/action_catalog.go`

- [ ] **Step 1: Add ActionConfigReload**

Add to the action ID constants:

```go
ActionConfigReload ActionID = "config.reload"
```

Add to the action metadata map with:
- Mutability: Write
- LocalEffect: None
- RuntimeReq: EmbeddedOK
- ExportPolicy: Allowed
- Summary: "Reload configuration from file"

- [ ] **Step 2: Commit**

```bash
git add internal/adapter/primary/frontend/action_catalog.go
git commit -m "feat: add config.reload action to shared action catalog"
```

---

### Task 8: Add POST /api/v1/config/reload endpoint

**Files:**
- Modify: `internal/adapter/primary/api/config.go`

- [ ] **Step 1: Add reload endpoint**

Add a new Huma operation after the existing GET /api/v1/config:

```go
type ConfigReloadInput struct {
	// No body needed — reload uses the server's config path.
}

type ConfigReloadOutput struct {
	Body struct {
		Status  string `json:"status" doc:"Reload result"`
		Message string `json:"message,omitempty" doc:"Error message on failure"`
	}
}

huma.Register(api, huma.Operation{
	OperationID: "config-reload",
	Method:      http.MethodPost,
	Path:        "/api/v1/config/reload",
	Summary:     "Reload configuration from file",
	Tags:        []string{"config"},
}, func(ctx context.Context, _ *ConfigReloadInput) (*ConfigReloadOutput, error) {
	if err := application.ReloadConfig(application.ConfigPath()); err != nil {
		out := &ConfigReloadOutput{}
		out.Body.Status = "error"
		out.Body.Message = err.Error()
		return out, nil
	}
	out := &ConfigReloadOutput{}
	out.Body.Status = "ok"
	return out, nil
})
```

Note: `application.ConfigPath()` needs to be added to App — a simple getter for the stored config path.

- [ ] **Step 2: Add ConfigPath() to App**

In `internal/app/app.go`:

```go
// ConfigPath returns the path the config was loaded from.
func (a *App) ConfigPath() string {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.configPath
}
```

And update `NewAppWithOptions` to accept the path. The constructors need updating:

```go
func NewApp(cfg *config.Config, logger *slog.Logger) *App {
	return NewAppWithOptions(cfg, logger, Options{RuntimeMode: RuntimeModePersistent})
}

func NewAppWithOptions(cfg *config.Config, logger *slog.Logger, opts Options) *App {
	// ... existing ...
	return &App{config: cfg, configPath: opts.ConfigPath, Logger: logger, runtimeMode: mode}
}
```

Add `ConfigPath string` to `Options`:

```go
type Options struct {
	RuntimeMode RuntimeMode
	ConfigPath  string
}
```

Then update CLI `root.go` line 139 and runtime `runtime.go` line 475 to pass `ConfigPath` in `Options`.

- [ ] **Step 3: Commit**

```bash
git add internal/app/app.go internal/adapter/primary/api/config.go internal/adapter/primary/cli/root.go internal/adapter/primary/runtime/runtime.go
git commit -m "feat: add POST /api/v1/config/reload endpoint"
```

---

### Task 9: Add client ConfigReload method

**Files:**
- Modify: `pkg/client/config.go`

- [ ] **Step 1: Add ConfigReload**

```go
// ConfigReloadResult is the response from the config reload endpoint.
type ConfigReloadResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ConfigReload triggers a config reload on the server.
func (c *Client) ConfigReload(ctx context.Context) (*ConfigReloadResult, error) {
	var result ConfigReloadResult
	err := c.post(ctx, "/api/v1/config/reload", nil, &result)
	return &result, err
}
```

- [ ] **Step 2: Commit**

```bash
git add pkg/client/config.go
git commit -m "feat(client): add ConfigReload method"
```

---

### Task 10: Add CLI config reload command

**Files:**
- Modify: `internal/adapter/primary/cli/config_cmd.go`

- [ ] **Step 1: Add reload subcommand**

```go
func newConfigReloadCmd(opts *Options) *cobra.Command {
	return withActionID(&cobra.Command{
		Use:   "reload",
		Short: "Reload the server configuration from file",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := opts.Frontend().Config().Reload(cmd.Context())
			if err != nil {
				return err
			}
			if result.Status == "error" {
				return fmt.Errorf("reload failed: %s", result.Message)
			}
			fmt.Fprintln(opts.Out, "Configuration reloaded successfully.")
			return nil
		},
	}, frontend.ActionConfigReload)
}
```

Add to `newConfigCmd`:
```go
cmd.AddCommand(newConfigShowCmd(opts), newConfigReloadCmd(opts))
```

Add `"fmt"` to imports.

- [ ] **Step 2: Add Reload method to ConfigClientWorkflow**

In `internal/adapter/primary/frontend/config_client_workflow.go` (or wherever ConfigClientWorkflow is defined), add:

```go
func (w *ConfigClientWorkflow) Reload(ctx context.Context) (*client.ConfigReloadResult, error) {
	return w.client.ConfigReload(ctx)
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/primary/cli/config_cmd.go internal/adapter/primary/frontend/
git commit -m "feat(cli): add config reload command"
```

---

### Task 11: Wire SIGHUP handler and ConfigWatcher in serve command

**Files:**
- Modify: `internal/adapter/primary/cli/serve.go`

- [ ] **Step 1: Add SIGHUP handler and file watcher**

Update the serve command's `RunE`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	// ... existing server setup ...

	srv := runtimeadapter.NewConfiguredServer(opts.Logger, opts.Application(), serverOpts)

	if err := srv.Start(); err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	// ... existing logging ...

	// Start config file watcher.
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "watchtower.yaml" // default
	}
	application := opts.Application()
	watcher, err := runtimeadapter.NewConfigWatcher(configPath, func(path string) error {
		return application.ReloadConfig(path)
	}, opts.Logger)
	if err != nil {
		opts.Logger.Warn("failed to start config watcher", "error", err)
	} else {
		defer watcher.Stop()
	}

	// Handle SIGHUP for manual reload.
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			opts.Logger.Info("received SIGHUP, reloading config")
			if err := application.ReloadConfig(configPath); err != nil {
				opts.Logger.Warn("SIGHUP config reload failed", "error", err)
			}
		}
	}()
	defer signal.Stop(sighup)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	opts.Logger.Info("shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
},
```

Add `"os"` to imports.

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/watchtower/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/primary/cli/serve.go
git commit -m "feat(serve): add SIGHUP handler and fsnotify config watcher"
```

---

### Task 12: Update AGENTS.md with reload boundary documentation

**Files:**
- Modify: `AGENTS.md`

- [ ] **Step 1: Add reload boundary section**

Add a new section under the existing headings:

```markdown
### Config reload boundary

The server supports live config reload (fsnotify, SIGHUP, API endpoint). Not all services pick up changes immediately:

**Reloads immediately** (per-request factories that call `GetConfig()` on each invocation):
- Build recipe builders and build service
- Package sources, commit sources, upstream provider
- Review projects, forge clients, bug trackers
- Release tracking, excuses sources
- Config API endpoint

**Does NOT reload (requires server restart):**
- Telemetry/OTel collectors (`sync.Once` in `telemetry_bootstrap.go`)
- TeamSyncService (`sync.Once` in `teamsync_bootstrap.go`)

New `sync.Once` services that read config at init time must be listed here. If a service should support live reload, it must use `GetConfig()` on each request instead of caching at init time.
```

- [ ] **Step 2: Commit**

```bash
git add AGENTS.md
git commit -m "docs: document config reload boundary in AGENTS.md"
```

---

### Task 13: Full Validation and PLAN.md sync

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 2: Run linter and architecture check**

Run: `pre-commit run --all-files`
Expected: PASS (except pre-existing coverage failures)

- [ ] **Step 3: Update PLAN.md**

Add to Current State:
- server supports live config reload via fsnotify file watching, SIGHUP signal, and `POST /api/v1/config/reload` endpoint; `App.Config` is private with `GetConfig()` accessor protected by `sync.RWMutex`; per-request services pick up changes immediately; `sync.Once` services (Telemetry, TeamSyncService) require server restart

- [ ] **Step 4: Commit**

```bash
git add PLAN.md
git commit -m "docs: sync PLAN.md with config hot-reload implementation"
```

---

### Task 14: Manual Integration Test

- [ ] **Step 1: Start server**

```bash
go run ./cmd/watchtower serve &
```

- [ ] **Step 2: Edit watchtower.yaml** (e.g. add a comment or change a project name)

Watch server logs — should see:
```
level=INFO msg="config file changed, reloading" path=watchtower.yaml
level=INFO msg="configuration reloaded" path=watchtower.yaml ...
```

- [ ] **Step 3: Test SIGHUP**

```bash
kill -HUP $(pgrep -f "watchtower serve")
```

Should see reload log.

- [ ] **Step 4: Test API endpoint**

```bash
go run ./cmd/watchtower config reload
```

Should print: `Configuration reloaded successfully.`

- [ ] **Step 5: Test error handling**

Introduce a syntax error in watchtower.yaml, trigger reload, verify old config is kept and error is reported.
