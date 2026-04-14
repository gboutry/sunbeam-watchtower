# Build prepare worktree isolation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve `prepare_command` outputs across `.gitignore` by running prepare in a temporary linked `git worktree`, force-staging there, pushing from there, and running artifact discovery on the prepared tree.

**Architecture:** Extend `port.GitClient` with two methods (`CreateDetachedWorktree`, `ForceAddAll`). Implement them in the go-git adapter using fixed-argv `exec.CommandContext` shell-outs. Centralise go-git repo opening behind an `openRepo` helper that sets `EnableDotGitCommonDir: true` so existing operations (Commit, HeadSHA, Push, etc.) work inside linked worktrees. Rewrite `prepareAndPush` + `PrepareTrigger` to run prepare in the temp worktree and discovery against the prepared tree.

**Tech Stack:** Go 1.22+, go-git v5, `os/exec`, `os.MkdirTemp`, hexagonal ports/adapters.

---

## File Structure

**Modify:**
- `internal/core/port/git.go` — extend interface.
- `internal/adapter/secondary/git/client.go` — add `openRepo` helper, route all `PlainOpen` calls through it; implement new methods.
- `internal/adapter/secondary/git/client_test.go` — tests for new methods + linked-worktree compatibility.
- `internal/adapter/primary/frontend/build_prepare.go` — rewrite the prepare path to use the temp worktree; preserve non-prepare path byte-for-byte.
- `internal/adapter/primary/frontend/build_prepare_test.go` — extend `fakeGitClient` stubs; add tests for new flow.
- `PLAN.md` (repo root).

**Create:**
- `docs/agents/specs/build-prepare-worktree-isolation.md` — design spec (mirrors the design section of this plan plus v1/v2 rejection history).

---

## Task 1: Centralise go-git repo opening with EnableDotGitCommonDir

**Files:**
- Modify: `internal/adapter/secondary/git/client.go`
- Test: `internal/adapter/secondary/git/client_test.go`

**Why:** go-git's `PlainOpen` does not follow `.git` gitdir pointer files. A linked worktree has `.git` as a file containing `gitdir: <path>`, so `PlainOpen` on a linked worktree fails to reach the shared object store; Commit, Status, Add then fail with `object not found`. `PlainOpenWithOptions{EnableDotGitCommonDir: true}` fixes this. Pre-requisite for every subsequent task that touches the temp worktree.

- [ ] **Step 1: Write the failing test — linked worktree Commit**

Add to `internal/adapter/secondary/git/client_test.go`:

```go
func TestClient_Commit_LinkedWorktree(t *testing.T) {
	t.Parallel()
	// Arrange: create a source repo with one commit.
	srcDir := t.TempDir()
	runGit(t, srcDir, "init", "-q", "-b", "main")
	runGit(t, srcDir, "config", "user.email", "test@example.com")
	runGit(t, srcDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, srcDir, "add", "README.md")
	runGit(t, srcDir, "commit", "-q", "-m", "init")

	sha := strings.TrimSpace(runGit(t, srcDir, "rev-parse", "HEAD"))

	// Create a linked worktree via the real git CLI.
	wtDir := filepath.Join(t.TempDir(), "wt")
	runGit(t, srcDir, "worktree", "add", "-b", "tmp-branch", wtDir, sha)

	// Add a new file inside the linked worktree.
	if err := os.WriteFile(filepath.Join(wtDir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtDir, "add", "new.txt")

	// Act: commit via our adapter.
	c := NewClient(nil)
	if err := c.Commit(wtDir, "test commit in linked worktree"); err != nil {
		t.Fatalf("Commit in linked worktree: %v", err)
	}

	// Assert: HEAD advanced.
	newSHA := strings.TrimSpace(runGit(t, wtDir, "rev-parse", "HEAD"))
	if newSHA == sha {
		t.Fatalf("HEAD did not advance: still %s", sha)
	}
}

// runGit is a test helper that invokes the git CLI and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, string(out))
	}
	return string(out)
}
```

Required imports in the test file (check current imports first; add any missing):
`"os"`, `"os/exec"`, `"path/filepath"`, `"strings"`, `"testing"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/secondary/git/ -run TestClient_Commit_LinkedWorktree -v`
Expected: FAIL with an error surfacing from go-git inside the linked worktree (likely `object not found` or `repository does not exist`).

- [ ] **Step 3: Add the centralised helper + migrate call sites**

In `internal/adapter/secondary/git/client.go`, add (place below `NewClient`):

```go
// openRepo opens a repository with linked-worktree support enabled so
// operations run inside a worktree created by `git worktree add` reach
// the shared object store via the `.git` gitdir pointer file.
func openRepo(path string) (*gogit.Repository, error) {
	return gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
}
```

Then replace **every** `gogit.PlainOpen(path)` call in `client.go` with `openRepo(path)`. Current call sites (15 total, verified):
- `IsRepo` (line ~44)
- `HeadSHA` (line ~50)
- `HasUncommittedChanges` (line ~65)
- `Push` (line ~82)
- `AddRemote` (line ~210)
- `RemoveRemote` (line ~229)
- `CreateBranch` (line ~241)
- `CheckoutBranch` (line ~268)
- `CurrentBranch` (line ~286)
- `DeleteLocalBranch` (line ~302)
- `AddAll` (line ~314)
- `Commit` (line ~330)
- `ResetHard` (line ~354)

The `client_test.go` call sites at lines 111, 132 are test plumbing (verifying repo state) — leave those alone; they open the original repo directory, not a linked worktree.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/secondary/git/ -run TestClient_Commit_LinkedWorktree -v`
Expected: PASS.

- [ ] **Step 5: Run the full git adapter test suite to confirm no regression**

Run: `go test ./internal/adapter/secondary/git/ -v`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/secondary/git/client.go internal/adapter/secondary/git/client_test.go
git commit -m "$(cat <<'EOF'
fix(git): enable linked-worktree support via centralised repo opener

Open every repo via openRepo() which sets PlainOpenOptions.EnableDotGitCommonDir=true, so go-git resolves the .git gitdir pointer file produced by `git worktree add`. Required for the prepare-in-temp-worktree flow that follows.

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 2: Extend port.GitClient with CreateDetachedWorktree

**Files:**
- Modify: `internal/core/port/git.go`
- Modify: `internal/adapter/primary/frontend/build_prepare_test.go` (`fakeGitClient` stub)

**Why:** Defines the adapter-to-frontend contract for the new capability before implementing it. Stubbing the fake keeps the workspace compiling so downstream tasks can progress in parallel.

- [ ] **Step 1: Write the failing test — frontend fake satisfies port**

Verify the current compile state first:

Run: `go build ./...`
Expected: PASS (current state).

Then add the interface method (this will break compilation until fakes are stubbed — expected):

In `internal/core/port/git.go`, extend the interface (add below `Commit`, before `ResetHard`):

```go
	// Detached-worktree operations for isolated prepare/push flows.
	//
	// CreateDetachedWorktree materialises a temporary linked worktree of
	// repoPath at the given sha on a new local branch named `branch`. It
	// honours $TMPDIR for the worktree directory (required for
	// snap-confined invocations). The returned cleanup closure must be
	// called to remove the worktree, the local branch, prune stale
	// `.git/worktrees/<name>` metadata from repoPath, and remove the
	// temporary directory. Cleanup is safe to call multiple times.
	CreateDetachedWorktree(ctx context.Context, repoPath, branch, sha string) (worktreePath string, cleanup func(), err error)
```

Required import in `git.go`:
```go
import "context"
```

- [ ] **Step 2: Run `go build ./...` to verify the fake breaks**

Run: `go build ./...`
Expected: FAIL — `*fakeGitClient` does not implement `port.GitClient` (missing `CreateDetachedWorktree`).

- [ ] **Step 3: Stub the fake**

In `internal/adapter/primary/frontend/build_prepare_test.go`, add to `fakeGitClient`:

```go
func (f *fakeGitClient) CreateDetachedWorktree(context.Context, string, string, string) (string, func(), error) {
	return "", func() {}, nil
}
```

If `context` is not yet imported, add `"context"` to the test file's imports.

Also stub any other type in that file that embeds/implements `port.GitClient` (e.g. `pushTrackingGitClient` at line 339 — inspect and stub if needed).

- [ ] **Step 4: Run `go build ./...` to verify**

Run: `go build ./...`
Expected: PASS.

- [ ] **Step 5: Run the frontend test suite**

Run: `go test ./internal/adapter/primary/frontend/ -v`
Expected: all tests pass (fake stub returns empty path + no-op cleanup; no test exercises the method yet).

- [ ] **Step 6: Commit**

```bash
git add internal/core/port/git.go internal/adapter/primary/frontend/build_prepare_test.go
git commit -m "$(cat <<'EOF'
feat(port): add CreateDetachedWorktree to GitClient

Declares the contract for isolating prepare commands in a linked worktree. Implementation follows.

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 3: Extend port.GitClient with ForceAddAll

**Files:**
- Modify: `internal/core/port/git.go`
- Modify: `internal/adapter/primary/frontend/build_prepare_test.go`

**Why:** Companion to Task 2. Separating signature from implementation unblocks parallel work.

- [ ] **Step 1: Extend the interface**

In `internal/core/port/git.go`, add below `AddAll`:

```go
	// ForceAddAll stages every file in worktreePath, bypassing .gitignore
	// (equivalent to `git add -f -A`). Intended only for temporary
	// worktrees created by CreateDetachedWorktree, where no pre-existing
	// ignored files can leak secrets.
	ForceAddAll(ctx context.Context, worktreePath string) error
```

- [ ] **Step 2: Run `go build ./...` to verify the fake breaks**

Run: `go build ./...`
Expected: FAIL — missing `ForceAddAll`.

- [ ] **Step 3: Stub the fake**

In `build_prepare_test.go`, add to `fakeGitClient`:

```go
func (f *fakeGitClient) ForceAddAll(context.Context, string) error { return nil }
```

Stub any other `port.GitClient` implementer in the file too.

- [ ] **Step 4: Run `go build ./...` to verify**

Run: `go build ./...`
Expected: PASS.

- [ ] **Step 5: Run the frontend test suite**

Run: `go test ./internal/adapter/primary/frontend/ -v`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/core/port/git.go internal/adapter/primary/frontend/build_prepare_test.go
git commit -m "$(cat <<'EOF'
feat(port): add ForceAddAll to GitClient

Stages worktree contents ignoring .gitignore, intended for use only inside disposable worktrees created by CreateDetachedWorktree.

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 4: Implement CreateDetachedWorktree in the git adapter

**Files:**
- Modify: `internal/adapter/secondary/git/client.go`
- Test: `internal/adapter/secondary/git/client_test.go`

**Why:** The real implementation. Fixed-argv `exec.CommandContext` — no `sh -c`, no string interpolation, no `cmdRunner` routing.

- [ ] **Step 1: Write the failing test — round-trip + cleanup**

Add to `client_test.go`:

```go
func TestClient_CreateDetachedWorktree_RoundTrip(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	runGit(t, srcDir, "init", "-q", "-b", "main")
	runGit(t, srcDir, "config", "user.email", "test@example.com")
	runGit(t, srcDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, srcDir, "add", "README.md")
	runGit(t, srcDir, "commit", "-q", "-m", "init")
	sha := strings.TrimSpace(runGit(t, srcDir, "rev-parse", "HEAD"))

	c := NewClient(nil)
	wtPath, cleanup, err := c.CreateDetachedWorktree(context.Background(), srcDir, "tmp-test-branch", sha)
	if err != nil {
		t.Fatalf("CreateDetachedWorktree: %v", err)
	}

	// Assert: directory exists, is a worktree, on expected branch.
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}
	gotBranch := strings.TrimSpace(runGit(t, wtPath, "branch", "--show-current"))
	if gotBranch != "tmp-test-branch" {
		t.Fatalf("branch = %q, want tmp-test-branch", gotBranch)
	}

	// Assert: listed in parent repo.
	listOut := runGit(t, srcDir, "worktree", "list")
	if !strings.Contains(listOut, wtPath) {
		t.Fatalf("worktree not listed in parent: %s", listOut)
	}

	// Act: cleanup.
	cleanup()

	// Assert: directory gone, branch gone, not listed.
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree dir still present after cleanup: err=%v", err)
	}
	listOut = runGit(t, srcDir, "worktree", "list")
	if strings.Contains(listOut, wtPath) {
		t.Fatalf("worktree still listed after cleanup: %s", listOut)
	}
	branches := runGit(t, srcDir, "branch", "--list")
	if strings.Contains(branches, "tmp-test-branch") {
		t.Fatalf("branch still present after cleanup: %s", branches)
	}

	// Double-cleanup is safe.
	cleanup()
}

func TestClient_CreateDetachedWorktree_FixedArgv(t *testing.T) {
	t.Parallel()
	// Branch names containing shell metacharacters must reach git as
	// literal argv — proof that we don't route through `sh -c`. git
	// itself rejects them as invalid refs, which is the signal we want.
	srcDir := t.TempDir()
	runGit(t, srcDir, "init", "-q", "-b", "main")
	runGit(t, srcDir, "config", "user.email", "test@example.com")
	runGit(t, srcDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, srcDir, "add", "README.md")
	runGit(t, srcDir, "commit", "-q", "-m", "init")
	sha := strings.TrimSpace(runGit(t, srcDir, "rev-parse", "HEAD"))

	c := NewClient(nil)
	// `;rm -rf /` as branch name: git refuses it; our process does not
	// expand it.
	_, _, err := c.CreateDetachedWorktree(context.Background(), srcDir, ";rm -rf /", sha)
	if err == nil {
		t.Fatalf("expected git to reject malformed branch name")
	}
	// Confirm src still exists — sh-c would have executed the rm.
	if _, statErr := os.Stat(srcDir); statErr != nil {
		t.Fatalf("source dir destroyed: %v", statErr)
	}
}
```

Required additional imports in `client_test.go`: `"context"`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/secondary/git/ -run TestClient_CreateDetachedWorktree -v`
Expected: FAIL — method not implemented; compile error.

- [ ] **Step 3: Implement the method**

In `client.go`, add the method plus a small helper. Put them alongside the other methods:

```go
// runGit executes `git` with fixed argv in the given working directory.
// No shell is involved; arguments are never interpreted.
func (c *Client) runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Debug("git command failed", "dir", dir, "args", args, "output", string(out), "err", err)
		return fmt.Errorf("git %s: %w (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	c.logger.Debug("git command ok", "dir", dir, "args", args)
	return nil
}

func (c *Client) CreateDetachedWorktree(ctx context.Context, repoPath, branch, sha string) (string, func(), error) {
	c.logger.Debug("creating detached worktree", "repoPath", repoPath, "branch", branch, "sha", sha)

	// Honour $TMPDIR for snap confinement.
	wtPath, err := os.MkdirTemp("", "watchtower-prepare-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("mkdirtemp: %w", err)
	}
	// MkdirTemp created the directory, but `git worktree add` requires
	// the target path not to exist. Remove it; git will recreate.
	if err := os.Remove(wtPath); err != nil {
		_ = os.RemoveAll(wtPath)
		return "", func() {}, fmt.Errorf("remove tmp slot: %w", err)
	}

	if err := c.runGit(ctx, repoPath, "worktree", "add", "-b", branch, wtPath, sha); err != nil {
		_ = os.RemoveAll(wtPath)
		return "", func() {}, fmt.Errorf("git worktree add: %w", err)
	}

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			// Use a short fresh context; caller context may already be cancelled.
			cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := c.runGit(cctx, repoPath, "worktree", "remove", "--force", wtPath); err != nil {
				c.logger.Warn("worktree remove failed", "path", wtPath, "err", err)
			}
			if err := c.runGit(cctx, repoPath, "branch", "-D", branch); err != nil {
				c.logger.Debug("branch -D failed (may already be gone)", "branch", branch, "err", err)
			}
			if err := c.runGit(cctx, repoPath, "worktree", "prune", "--expire", "now"); err != nil {
				c.logger.Debug("worktree prune failed", "err", err)
			}
			if err := os.RemoveAll(wtPath); err != nil {
				c.logger.Warn("removeall tempdir failed", "path", wtPath, "err", err)
			}
		})
	}
	return wtPath, cleanup, nil
}
```

Required imports to add to `client.go`:
```go
"context"
"os/exec"
"sync"
```

(`os`, `time`, `fmt`, `strings` already imported.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/adapter/secondary/git/ -run TestClient_CreateDetachedWorktree -v`
Expected: PASS (both tests).

- [ ] **Step 5: Run the full git adapter test suite**

Run: `go test ./internal/adapter/secondary/git/ -v`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/secondary/git/client.go internal/adapter/secondary/git/client_test.go
git commit -m "$(cat <<'EOF'
feat(git): implement CreateDetachedWorktree via fixed-argv exec

Shells out to `git worktree add -b ...` with fixed argv (no sh -c, no string interpolation). Returns a cleanup closure that removes the worktree, deletes the local branch, prunes stale metadata, and removes the tmp directory. Honours $TMPDIR.

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 5: Implement ForceAddAll in the git adapter

**Files:**
- Modify: `internal/adapter/secondary/git/client.go`
- Test: `internal/adapter/secondary/git/client_test.go`

- [ ] **Step 1: Write the failing test**

Add to `client_test.go`:

```go
func TestClient_ForceAddAll_StagesIgnoredFiles(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	runGit(t, srcDir, "init", "-q", "-b", "main")
	runGit(t, srcDir, "config", "user.email", "test@example.com")
	runGit(t, srcDir, "config", "user.name", "Test")
	// .gitignore excludes build/
	if err := os.WriteFile(filepath.Join(srcDir, ".gitignore"), []byte("build/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, srcDir, "add", ".gitignore")
	runGit(t, srcDir, "commit", "-q", "-m", "init")

	// Create an "ignored" file.
	if err := os.MkdirAll(filepath.Join(srcDir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "build", "artifact.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Baseline: plain `git add -A` does NOT stage it.
	runGit(t, srcDir, "add", "-A")
	staged := strings.TrimSpace(runGit(t, srcDir, "diff", "--cached", "--name-only"))
	if strings.Contains(staged, "build/artifact.txt") {
		t.Fatalf("plain add staged ignored file — test setup wrong: %s", staged)
	}
	// Reset index.
	runGit(t, srcDir, "reset", "-q")

	// Act: ForceAddAll.
	c := NewClient(nil)
	if err := c.ForceAddAll(context.Background(), srcDir); err != nil {
		t.Fatalf("ForceAddAll: %v", err)
	}

	// Assert: ignored file is staged.
	staged = strings.TrimSpace(runGit(t, srcDir, "diff", "--cached", "--name-only"))
	if !strings.Contains(staged, "build/artifact.txt") {
		t.Fatalf("ignored file not staged: %s", staged)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapter/secondary/git/ -run TestClient_ForceAddAll -v`
Expected: FAIL — method not implemented.

- [ ] **Step 3: Implement**

In `client.go`, add:

```go
func (c *Client) ForceAddAll(ctx context.Context, worktreePath string) error {
	c.logger.Debug("force-staging all", "path", worktreePath)
	return c.runGit(ctx, worktreePath, "add", "-f", "-A")
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/adapter/secondary/git/ -run TestClient_ForceAddAll -v`
Expected: PASS.

- [ ] **Step 5: Run the full git adapter test suite**

Run: `go test ./internal/adapter/secondary/git/ -v`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/secondary/git/client.go internal/adapter/secondary/git/client_test.go
git commit -m "$(cat <<'EOF'
feat(git): implement ForceAddAll via `git add -f -A`

Shells out with fixed argv. Callers must restrict use to disposable worktrees (no pre-existing ignored files to leak).

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 6: Rewrite PrepareTrigger + prepareAndPush to use the temp worktree

**Files:**
- Modify: `internal/adapter/primary/frontend/build_prepare.go`
- Modify: `internal/adapter/primary/frontend/build_prepare_test.go`

**Why:** The integration step — wire the new port methods into the build flow and move discovery to the prepared tree. Non-prepare path preserved byte-for-byte.

Current flow (to rewrite):
- `PrepareTrigger` builds `branchName` (line 121), conditionally calls `prepareAndPush` when LP ref is missing (line 128), then always calls `DiscoverRecipes(localPath)` (line 147) and `snapProcessorsFromRepo(localPath, ...)` (line 201).
- `prepareAndPush` (line 254): `CurrentBranch` → `CreateBranch` → `CheckoutBranch` → defer checkout-back+delete → optional prepare+AddAll+Commit → `pushToLaunchpad`.

New flow (design section "Flow", steps 1-9 of the approved plan):
- When `prepare_command` is set: always materialise a temp worktree; run prepare + ForceAddAll + Commit unconditionally; push conditionally (skip if LP ref exists); discover from tempDir.
- When `prepare_command` is unset: preserve current flow (no worktree, AddAll on live repo is already dead code for this path since today's `prepareAndPush` only runs `AddAll` when `prepareCommand != ""`, so the unset path is just a no-op prepare followed by push-from-live-repo; discovery from `localPath` as today).

- [ ] **Step 1: Write the failing tests for the new flow**

Study the existing test harness around `build_prepare_test.go:102` and `:344` first:

Run: `sed -n '1,60p' internal/adapter/primary/frontend/build_prepare_test.go`

Then append to `build_prepare_test.go` (adjust helpers/fakes to match the existing file's style — e.g. extend `fakeGitClient` with call-tracking fields and a new `tempWorktree` return value):

```go
// Extend fakeGitClient with call-tracking + scripted temp worktree.
// Rename if the current file uses a different name for the same role.

// In fakeGitClient (or equivalent):
//   tempWorktreeDir  string
//   createWtCalls    int
//   forceAddAllCalls int
//   commitCalls      int
//   pushCalls        int
//   cleanupCalls     int

// Replace the existing CreateDetachedWorktree stub:
func (f *fakeGitClient) CreateDetachedWorktree(_ context.Context, _, _, _ string) (string, func(), error) {
	f.createWtCalls++
	if f.tempWorktreeDir == "" {
		// Default to a real temp dir so downstream code can stat it.
		d, err := os.MkdirTemp("", "watchtower-test-wt-*")
		if err != nil {
			return "", func() {}, err
		}
		f.tempWorktreeDir = d
	}
	return f.tempWorktreeDir, func() { f.cleanupCalls++ }, nil
}

// Replace ForceAddAll stub:
func (f *fakeGitClient) ForceAddAll(context.Context, string) error { f.forceAddAllCalls++; return nil }

func TestPrepareTrigger_WithPrepareCommand_UsesTempWorktree(t *testing.T) {
	// Arrange: build a preparer with a fakeGitClient that has a
	// prepare_command configured, a fake cmdRunner that records the
	// working-dir it was called with, a fake repoManager whose
	// GetGitRef reports "not found" (so push happens), and a fake
	// builder whose DiscoverRecipes records the path it was given.
	//
	// Act: call PrepareTrigger.
	//
	// Assert:
	//   - fakeGit.createWtCalls == 1
	//   - cmdRunner was invoked with cwd == tempWorktreeDir (NOT localPath)
	//   - fakeGit.forceAddAllCalls == 1
	//   - fakeGit.commitCalls == 1
	//   - fakeGit.pushCalls >= 1
	//   - builder.DiscoverRecipes was called with tempWorktreeDir
	//   - fakeGit.cleanupCalls == 1 (even if called twice, idempotent)
	//
	// Concrete assertions must match existing test helpers in the file.
}

func TestPrepareTrigger_WithPrepareCommand_SkipsPushWhenRefExists(t *testing.T) {
	// As above, but fake repoManager reports the LP ref EXISTS.
	// Assert:
	//   - createWtCalls == 1 (we still materialise)
	//   - cmdRunner invoked (prepare still runs)
	//   - forceAddAllCalls == 1, commitCalls == 1
	//   - pushCalls == 0
	//   - DiscoverRecipes called with tempWorktreeDir
	//   - cleanupCalls == 1
}

func TestPrepareTrigger_WithoutPrepareCommand_PreservesCurrentFlow(t *testing.T) {
	// Arrange: prepare_command == "".
	// Assert:
	//   - createWtCalls == 0
	//   - forceAddAllCalls == 0
	//   - DiscoverRecipes called with localPath (NOT a temp dir)
	//   - pushCalls matches the current behaviour
}

func TestPrepareTrigger_PrepareCommandFails_CleanupRuns(t *testing.T) {
	// Arrange: cmdRunner.Run returns an error.
	// Act: call PrepareTrigger — expect error.
	// Assert: cleanupCalls == 1 (defer still fires).
}
```

The four tests above are the specification. Flesh them out using the patterns already established in `build_prepare_test.go` — match the construction of `LocalBuildPreparer`, the `fakeCmdRunner`, `fakeRepoManager`, and project-builder fixtures as used at lines 102-344.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapter/primary/frontend/ -run TestPrepareTrigger_With -v`
Expected: FAIL — assertions don't match current flow (discovery reads localPath; no worktree created).

- [ ] **Step 3: Rewrite `prepareAndPush` and move discovery calls**

In `build_prepare.go`:

1. Change the `prepareAndPush` signature to return the path that discovery should use and the cleanup closure:

```go
// prepareAndPush runs the prepare command in an isolated worktree (when
// configured) and pushes the result to Launchpad. It returns the path
// that subsequent discovery should read from, plus a cleanup closure
// the caller must defer. When prepareCommand is empty, the returned
// path is localPath and cleanup is a no-op.
func (p *LocalBuildPreparer) prepareAndPush(
	ctx context.Context,
	localPath, gitSSHURL, repoSelfLink, lpOwner, branchName, sha, prepareCommand string,
	skipPush bool,
) (discoverPath string, cleanup func(), err error) {
	cleanup = func() {}

	if prepareCommand == "" {
		// No-prepare path: current behaviour preserved.
		if !skipPush {
			// Need a main branch on LP first if none exists.
			needsMain := true
			if _, err := p.repoManager.GetGitRef(ctx, repoSelfLink, "refs/heads/main"); err == nil {
				needsMain = false
			}
			// Push directly from localPath on the current branch.
			if err := pushFromLocalPath(ctx, p.gitClient, localPath, gitSSHURL, lpOwner, branchName, sha, needsMain); err != nil {
				return "", cleanup, fmt.Errorf("push to LP: %w", err)
			}
		}
		return localPath, cleanup, nil
	}

	if p.cmdRunner == nil {
		return "", cleanup, fmt.Errorf("prepare command configured but no command runner available")
	}

	// Prepare path: isolate in a temp worktree.
	wtPath, wtCleanup, err := p.gitClient.CreateDetachedWorktree(ctx, localPath, branchName, sha)
	if err != nil {
		return "", cleanup, fmt.Errorf("create detached worktree: %w", err)
	}
	cleanup = wtCleanup

	if err := p.cmdRunner.Run(ctx, wtPath, prepareCommand); err != nil {
		return "", cleanup, fmt.Errorf("run prepare command: %w", err)
	}
	if err := p.gitClient.ForceAddAll(ctx, wtPath); err != nil {
		return "", cleanup, fmt.Errorf("force-stage prepared changes: %w", err)
	}
	if err := p.gitClient.Commit(wtPath, "watchtower: prepare build"); err != nil {
		return "", cleanup, fmt.Errorf("commit prepared changes: %w", err)
	}

	if !skipPush {
		needsMain := true
		if _, err := p.repoManager.GetGitRef(ctx, repoSelfLink, "refs/heads/main"); err == nil {
			needsMain = false
		}
		if err := pushToLaunchpad(p.gitClient, wtPath, gitSSHURL, lpOwner, branchName, needsMain); err != nil {
			return "", cleanup, fmt.Errorf("push to LP: %w", err)
		}
	}

	return wtPath, cleanup, nil
}

// pushFromLocalPath replicates the old branch-dance for the
// no-prepare-command path: create the temp branch on localPath, push,
// restore. Preserves prior behaviour exactly.
func pushFromLocalPath(
	ctx context.Context,
	gitClient port.GitClient,
	localPath, gitSSHURL, lpOwner, branchName, sha string,
	pushMain bool,
) error {
	origBranch, err := gitClient.CurrentBranch(localPath)
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	if err := gitClient.CreateBranch(localPath, branchName, sha); err != nil {
		return fmt.Errorf("create branch %s: %w", branchName, err)
	}
	if err := gitClient.CheckoutBranch(localPath, branchName); err != nil {
		return fmt.Errorf("checkout branch %s: %w", branchName, err)
	}
	defer func() {
		_ = gitClient.CheckoutBranch(localPath, origBranch)
		_ = gitClient.DeleteLocalBranch(localPath, branchName)
	}()
	return pushToLaunchpad(gitClient, localPath, gitSSHURL, lpOwner, branchName, pushMain)
}
```

2. Update `PrepareTrigger` (currently around line 76) to call the new signature and use the returned `discoverPath` for discovery. Replace the block that currently reads:

```go
	_, refCheckErr := p.repoManager.GetGitRef(ctx, repoSelfLink, refPath)
	if refCheckErr != nil {
		if err := p.prepareAndPush(ctx, localPath, gitSSHURL, repoSelfLink, lpOwner, branchName, sha, pb.PrepareCommand); err != nil {
			return req, fmt.Errorf("prepare and push: %w", err)
		}
	}
	// ...
	discovered, err := pb.Strategy.DiscoverRecipes(localPath)
	// ... later:
	procs, err := snapProcessorsFromRepo(localPath, r, pb.Strategy)
```

with:

```go
	_, refCheckErr := p.repoManager.GetGitRef(ctx, repoSelfLink, refPath)
	skipPush := refCheckErr == nil
	discoverPath, cleanup, err := p.prepareAndPush(ctx, localPath, gitSSHURL, repoSelfLink, lpOwner, branchName, sha, pb.PrepareCommand, skipPush)
	if err != nil {
		return req, fmt.Errorf("prepare and push: %w", err)
	}
	defer cleanup()
	// ...
	discovered, err := pb.Strategy.DiscoverRecipes(discoverPath)
	// ... later:
	procs, err := snapProcessorsFromRepo(discoverPath, r, pb.Strategy)
```

Leave all other code in `PrepareTrigger` (artifacts filtering, tempNames, PreparedBuildSource construction) untouched.

3. Remove the old in-place branch-dance code from the former `prepareAndPush` body — it's now in `pushFromLocalPath` for the no-prepare path.

- [ ] **Step 4: Run the new tests to verify pass**

Run: `go test ./internal/adapter/primary/frontend/ -run TestPrepareTrigger_With -v`
Expected: PASS.

- [ ] **Step 5: Run the full frontend test suite**

Run: `go test ./internal/adapter/primary/frontend/ -v`
Expected: all tests pass, including pre-existing ones for `PrepareTrigger`.

- [ ] **Step 6: Run the whole test suite**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/primary/frontend/build_prepare.go internal/adapter/primary/frontend/build_prepare_test.go
git commit -m "$(cat <<'EOF'
feat(build): prepare in isolated worktree; discovery uses prepared tree

When prepare_command is set, materialise a linked worktree via CreateDetachedWorktree, run prepare there, ForceAddAll, commit, and push from there. DiscoverRecipes and snapProcessorsFromRepo read the prepared tree so LP and discovery see the same source. Cleanup runs on every exit path including push-skipped reruns and prepare failures. No-prepare path preserved byte-for-byte via pushFromLocalPath.

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 7: Create design spec in docs/agents/specs/

**Files:**
- Create: `docs/agents/specs/build-prepare-worktree-isolation.md`

- [ ] **Step 1: Write the spec**

Create the file with these sections (in order), sourcing content from `/home/guillaume.boutry@canonical.com/.claude/plans/dapper-toasting-popcorn.md`:

- Context (matches plan Context section)
- Problem (matches plan Problem section)
- Design (matches plan Design section, including rejected-alternatives subsection — v1 and v2 rejection history must be preserved so future work does not re-propose them)
- Shell-out safety (matches plan subsection)
- Security properties (narrow: prevents accidental inclusion of live-repo ignored files; does NOT sandbox prepare_command)
- Implementation summary (port methods added, adapter changes, build_prepare.go rewrite) — refer readers to this implementation plan for execution detail
- References to related work (link to `docs/agents/plans/build-prepare-worktree-isolation.md`)

- [ ] **Step 2: Commit**

```bash
git add docs/agents/specs/build-prepare-worktree-isolation.md
git commit -m "$(cat <<'EOF'
docs: add design spec for build-prepare worktree isolation

Captures the rejected v1 (live-worktree snapshot-diff) and v2 (detached HEAD in linked worktree) approaches alongside the accepted design, so future work does not re-propose them.

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 8: Update user-facing docs for prepare_command

**Files:**
- Modify: `watchtower.yaml.example` (or wherever `prepare_command` is documented — verify via `rg -n "prepare_command"` first)

- [ ] **Step 1: Locate the current `prepare_command` documentation**

Run: `rg -n "prepare_command" --type yaml --type md`

If a `watchtower.yaml.example` exists and documents `prepare_command`, edit it. If only `docs/` files mention it, edit those. If nothing documents it yet, add a section to the primary config example file near other `build:` options.

- [ ] **Step 2: Add the documentation block**

Insert (adjust indentation / style to match surrounding docs):

```yaml
    # prepare_command (optional) runs inside an isolated temporary
    # checkout of the target commit before the source is pushed to
    # Launchpad. Files produced by this command are pushed in full —
    # including files that your repo's .gitignore would normally
    # exclude (e.g. build/, dist/, generated tarballs). Your live
    # worktree is not touched.
    #
    # Security scope: this isolation prevents accidental inclusion
    # of pre-existing ignored files from your live tree (.env, IDE
    # caches, credentials, etc.). It does NOT sandbox the prepare
    # command itself — the command runs as your user with normal
    # filesystem and environment access.
    prepare_command: "make prepare"
```

- [ ] **Step 3: Commit**

```bash
git add watchtower.yaml.example   # adjust path
git commit -m "$(cat <<'EOF'
docs: document prepare_command gitignore semantics and security scope

Makes explicit that prepare_command outputs bypass .gitignore (by design) and clarifies that the isolation only prevents leaking pre-existing ignored files, not sandboxing the command itself.

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 9: Sync PLAN.md

**Files:**
- Modify: `PLAN.md`

**Why:** AGENTS.md mandates `PLAN.md` is synced for every feature.

- [ ] **Step 1: Read current PLAN.md to find the correct section**

Run: `head -80 PLAN.md`

Locate the section covering build-trigger / build-prepare work.

- [ ] **Step 2: Add an entry**

Add an entry (match existing style) stating:

> **Build prepare: isolated worktree.** `prepare_command` now runs in a temporary linked `git worktree` rather than in the live repo. Its outputs — including files matching `.gitignore` — are force-staged and pushed to Launchpad. Discovery runs on the prepared tree so LP and local discovery see the same source. Implemented via new `port.GitClient` methods `CreateDetachedWorktree` and `ForceAddAll`; existing go-git repo opens now use `EnableDotGitCommonDir: true` for linked-worktree compatibility.

- [ ] **Step 3: Commit**

```bash
git add PLAN.md
git commit -m "$(cat <<'EOF'
docs: sync PLAN.md with build-prepare worktree isolation

Assisted-by: Claude Code (claude-opus-4-6)
EOF
)"
```

---

## Task 10: End-to-end verification

Not a code change — execute after all code tasks are merged.

- [ ] **Step 1: Build the binary**

Run: `go build -o /tmp/watchtower ./cmd/watchtower`
Expected: PASS.

- [ ] **Step 2: Run against the user's project**

Configure `prepare_command: "<cmd> --clean prepare"` for the target project in `watchtower.yaml`. Then:

Run: `/tmp/watchtower build --local-path <repo> <project> <artifact>`

- [ ] **Step 3: Verify LP temp branch contents**

On Launchpad, inspect the temp branch `tmp-<prefix>-<sha8>` and confirm the tree contains files that the prepare command wrote into previously-ignored paths (`build/`, `dist/`, tarballs).

- [ ] **Step 4: Verify the live worktree is untouched**

Run: `git -C <repo> status`
Expected: whatever state existed before the invocation (the watchtower run must not have modified tracked files or created a stale tmp branch locally).

Run: `git -C <repo> worktree list`
Expected: only the main worktree. No stale `tmp-*` entries.

- [ ] **Step 5: Rerun and verify push skip**

Run the same command again. Confirm the invocation succeeds quickly (push is skipped because the LP ref already exists) and discovery still returns the correct recipe set.

---

## Parallelisation notes (for subagent-driven execution)

- **Serial prerequisite:** Task 1 (centralised `openRepo`) must merge before Tasks 4, 5, 6 — the linked-worktree path depends on it.
- **Parallel batch A (after Task 1):** Task 2 and Task 3 (port signatures + fakes) can run concurrently; both are tiny.
- **Parallel batch B (after Tasks 2+3):** Task 4 and Task 5 (adapter implementations) can run concurrently.
- **Serial:** Task 6 (build_prepare.go rewrite) depends on Tasks 2+3 (signatures) and logically follows 4+5 (implementations must exist to pass end-to-end tests).
- **Parallel batch C (after Task 6):** Tasks 7, 8, 9 (docs) can run concurrently.
- **Serial:** Task 10 (E2E) runs last, manually.

---

## Spec coverage check

| Spec requirement | Task |
|---|---|
| `openRepo` with `EnableDotGitCommonDir` across all call sites | 1 |
| `CreateDetachedWorktree` port method | 2 |
| `ForceAddAll` port method | 3 |
| Fixed-argv shell-out, no `sh -c` | 4, 5 |
| `os.MkdirTemp("", …)` honouring `$TMPDIR` | 4 |
| `git worktree add -b <branch>` (not `--detach`) | 4 |
| Cleanup closure: worktree remove, branch -D, prune, RemoveAll | 4 |
| `git add -f -A` implementation | 5 |
| Prepare runs in temp worktree | 6 |
| Always prepare when `prepare_command` set; push conditional | 6 |
| Discovery reads prepared tree | 6 |
| Non-prepare path byte-for-byte preserved | 6 |
| Cleanup on prepare/push failure | 6 |
| v1/v2 rejection captured in spec | 7 |
| User-facing docs updated | 8 |
| `PLAN.md` synced | 9 |
| End-to-end manual verification | 10 |
