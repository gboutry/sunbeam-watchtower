# Build Prepare Pipeline Design

## Goal

Rework the build subcommand's local preparation flow to support branch-based temporary builds from local clones of official LP mirror repos. The primary use case is CI integration (Zuul pipelines for charm and rock monorepos), but the design is project-agnostic.

## Context

The build system already supports triggering, listing, downloading, and cleaning up LP recipe builds across rocks, charms, and snaps. The local preparation flow (`LocalBuildPreparer`) pushes HEAD to a personal LP repo and creates temporary recipes. However, it lacks:

- Named temporary branches (required by LP recipes)
- Optional prepare-command execution (e.g., charm monorepo needs `tox -e build` before commit)
- Remote branch reuse when the same commit has already been prepared
- Branch cleanup alongside recipe cleanup
- Proper local state restoration after push

## CI Workflow (Zuul)

The target CI pipeline has these stages:

1. **Check**: detect touched artifacts, `build trigger --prefix tmp-<changeset>-<ref> --local-path /path artifact1 artifact2 ...` with `--wait`, then `build download` to get artifacts for testing
2. **Gate**: same flow but different git ref (merge candidate on top of master)
3. **Promotion**: artifacts transferred via Zuul artifact storage, watchtower not involved
4. **Cleanup**: `build cleanup --prefix tmp-<changeset>` deletes all recipes and branches across all stages

The CI manages prefix naming externally. Touched-path detection (which artifacts to build) is also external — watchtower just builds what it's told.

Key property: re-running `build trigger` with the same prefix picks up where it left off. The existing `assessRecipe` logic handles create/retry/monitor/download decisions.

## Design

### GitClient Interface Extension

`port.GitClient` gains these methods (all implemented via go-git):

```go
type GitClient interface {
    // existing
    IsRepo(path string) bool
    HeadSHA(path string) (string, error)
    HasUncommittedChanges(path string) (bool, error)
    Push(path, remote, localRef, remoteRef string, force bool) error
    AddRemote(path, name, url string) error
    RemoveRemote(path, name string) error

    // new
    CreateBranch(path, branchName, startPoint string) error
    CheckoutBranch(path, branchName string) error
    CurrentBranch(path string) (string, error)
    AddAll(path string) error
    Commit(path, message string) error
    ResetHard(path, ref string) error
    RemoteRefExists(path, remote, ref string) (bool, error)
}
```

### CommandRunner

A frontend-side interface for executing shell commands in a directory:

```go
type CommandRunner interface {
    Run(ctx context.Context, dir string, command string) error
}
```

Implementation uses `os/exec` with shell invocation. This is a dependency of `LocalBuildPreparer`, not a core port.

### RepoManager Interface Extension

`port.RepoManager` gains two methods for cleanup:

```go
type RepoManager interface {
    // existing methods unchanged

    ListBranches(ctx context.Context, repoSelfLink string) ([]GitRef, error)
    DeleteGitRef(ctx context.Context, refSelfLink string) error
}
```

`ListBranches` wraps `GetGitBranches`. `DeleteGitRef` calls `DELETE` on the ref's self_link via the existing LP client `Delete` method.

The remote-branch-exists check for the skip optimization uses the existing `GetGitRef` method (404 means not found).

### Reworked LocalBuildPreparer.PrepareTrigger

The full flow:

1. Resolve HEAD SHA from local clone
2. Resolve LP owner (from flag or `GetCurrentUser`)
3. Get or create personal LP project and repo
4. Build branch name: `tmp-<prefix>-<sha[:8]>`
5. Check if branch already exists on LP via `RepoManager.GetGitRef`
   - If exists: use the returned ref link, skip to step 11
   - If not: continue with preparation
6. Save current branch via `GitClient.CurrentBranch`
7. Create and checkout temp branch via `GitClient.CreateBranch` + `CheckoutBranch`
8. If `prepare_command` configured: run via `CommandRunner`, then `GitClient.AddAll` + `Commit`
9. Push temp branch to LP via `pushToLaunchpad` (reworked to push a single named branch)
10. Restore local state: checkout original branch, delete local temp branch
11. If ref link not already obtained in step 5: wait for git ref on LP via `RepoManager.WaitForGitRef`
12. Discover artifacts from local clone via `Strategy.DiscoverRecipes`
13. Filter by explicit artifact names if provided via CLI positional args
14. Build `PreparedBuildSource` with one entry per artifact (temp recipe name, ref link, build path)
15. Return prepared request for server-side execution

### pushToLaunchpad Rework

Current: pushes HEAD to both `refs/heads/main` and `refs/heads/tmp-<sha>`.

New: pushes the named temp branch only. No main branch push.

```
pushToLaunchpad(gitClient, localPath, gitSSHURL, lpOwner, branchName):
    sshURL = fix git+ssh:// and inject username
    add temp remote
    push branchName to refs/heads/branchName (force)
    remove temp remote
```

### Cleanup Rework

**Bug fix**: `Service.Cleanup` currently iterates `pb.Artifacts` (configured artifact names) and filters by prefix. Temp recipes have names like `tmp-<prefix>-<sha>-<artifact>` which are never in the config. Fix: use `ListRecipesByOwner` and filter by prefix, same as `List` already does for prefix discovery.

**Branch cleanup**: after deleting recipes, delete matching remote branches via LP API.

**CleanupOpts extension**:

```go
type CleanupOpts struct {
    Projects  []string
    Owner     string
    Prefix    string
    DryRun    bool
    TargetRef string // LP project for branch cleanup resolution
}
```

**Return type change**:

```go
type CleanupResult struct {
    DeletedRecipes  []string
    DeletedBranches []string
}
```

Cleanup flow:

1. For each matching project:
   - `ListRecipesByOwner(ctx, owner)` and filter by prefix
   - Delete each matching recipe (or log in dry-run)
2. Resolve user's LP repo via `RepoManager.GetOrCreateRepo`
3. `ListBranches(ctx, repoSelfLink)` and filter by `refs/heads/tmp-<prefix>`
4. Delete each matching branch via `DeleteGitRef` (or log in dry-run)

### CLI Changes

- `build trigger`: remove `--source` flag. `--local-path` being set implies local preparation; absence implies remote/official resolution from LP.
- `build cleanup`: report deleted branches alongside deleted recipes in output. Resolve user's LP repo for branch cleanup.
- No new flags needed.

### Config

No new config fields. Existing fields suffice:

- `prepare_command`: already in `ProjectBuildConfig`, just needs wiring
- `official_codehosting`: already exists
- `owner`, `lp_project`, `artifacts`: already exist

`watchtower.yaml` updates: add `build:` blocks to snap and charm projects that need them.

### Action Catalog

No changes. Existing action IDs cover all user-facing operations. The prepare step is internal to the trigger flow.

## Files Affected

**New files:**
- `CommandRunner` interface and `os/exec` implementation (frontend-side)

**Extended:**
- `internal/core/port/git.go` — new `GitClient` methods
- `internal/core/port/build.go` — new `RepoManager` methods
- Git go-git adapter — implement new `GitClient` methods
- LP `RepoManager` adapter — implement `ListBranches`, `DeleteGitRef`
- `pkg/launchpad/v1/git.go` — add `DeleteGitRef` LP client method
- `internal/core/service/build/service.go` — fix `Cleanup`, add branch deletion, new `CleanupResult`

**Reworked:**
- `internal/adapter/primary/frontend/build_prepare.go` — branch-based prepare flow with prepare command support
- `internal/adapter/primary/cli/build.go` — remove `--source`, update cleanup output

**Config:**
- `watchtower.yaml` — add `build:` blocks to projects

## Architecture Compliance

- Local git operations and command execution stay in the frontend layer (`LocalBuildPreparer`)
- Server receives only LP references via `PreparedBuildSource` — no filesystem access
- `CommandRunner` is a frontend-side dependency, not a core port
- `RepoManager` extensions follow the existing secondary adapter pattern
- No new primary-to-secondary adapter imports
- Action catalog unchanged — no new authorization surface
