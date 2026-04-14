# Build-prepare worktree isolation — design spec

## Context

`watchtower build` pushes a user's local source tree to a Launchpad temporary
branch so LP can run the actual build. When a project defines a
`prepare_command`, watchtower runs it inside the local repo before pushing.

Before this change, the staging step in the git adapter
(`internal/adapter/secondary/git/client.go`, `AddWithOptions{All: true}`)
honored `.gitignore`. Many real prepare commands write into conventionally
ignored paths (`build/`, `dist/`, generated tarballs, vendored deps). Those
outputs were silently dropped from the push, and LP built against an
incomplete tree.

The motivating workflow: a client project runs `<cmd> --clean prepare`,
which wipes and recreates files in ignored directories. Every regenerated
output vanished from the push.

## Problem

`.gitignore` documents working-tree hygiene for the human developer. When
a `prepare_command` runs, its outputs are — by definition — the build
input and must reach LP regardless of `.gitignore`. Bypassing `.gitignore`
on the live worktree, however, would upload `.env`, cached credentials,
IDE state, and stale artifacts.

## Design

Run the prepare command in an **attached linked `git worktree`** on a named
throwaway branch. Shell out (fixed argv) to `git add -f -A` there. Commit +
push from that branch through the existing Push path. Run discovery in the
prepared tree. Clean up.

### Why this works

- The temp worktree is a fresh checkout of the target SHA via
  `os.MkdirTemp("", "watchtower-prepare-*")` — respects `$TMPDIR` (required
  for snap confinement; no hard-coded `/tmp`).
- `git worktree add -b <branch> <path> <sha>` produces a real local branch
  in the temp worktree, so the existing `Push` code path works unchanged —
  no detached-HEAD exception needed.
- The temp worktree starts with zero pre-existing ignored files.
  Force-staging everything is safe against accidental inclusion of
  live-repo secrets (`.env`, IDE caches). **This is the only security
  property claimed.** `prepare_command` itself still runs as the user and
  has normal access to SSH agent / environment / filesystem — the temp
  worktree is not a sandbox.
- The user's live worktree is never checked out onto a temp branch. No
  branch dance, no deferred checkout-back, no risk of go-git's `MergeReset`
  removing prepare outputs from the user's working tree.
- No TOCTOU window with background tools. Editors and file watchers do not
  touch `<tempDir>`.
- Discovery (`DiscoverRecipes`, `snapProcessorsFromRepo`) runs against the
  same tree LP receives — strictly more correct than the pre-change state,
  where discovery could see a different tree than what was pushed.

### Flow

Engages when `prepare_command` is set. Otherwise: prior behavior preserved
exactly (no worktree, no force-add, discovery on `localPath`).

1. `os.MkdirTemp("", "watchtower-prepare-*")` → `<tempDir>`.
2. Fixed-argv shell-out to
   `git -C <repoPath> worktree add -b tmp-<prefix>-<sha8> <tempDir> <sha>`.
3. Defer cleanup closure:
   - `git -C <repoPath> worktree remove --force <tempDir>`
   - `git -C <repoPath> branch -D tmp-<prefix>-<sha8>` (local-only branch;
     force-delete avoids "not merged" refusal)
   - `git -C <repoPath> worktree prune --expire now` — opportunistic;
     defends against stale `.git/worktrees/<name>/` from past SIGKILLs
   - `os.RemoveAll(<tempDir>)` as belt-and-braces
   Cleanup is idempotent via `sync.Once` — safe to call multiple times.
4. Run `prepare_command` inside `<tempDir>` via the existing frontend
   `cmdRunner`. `cmdRunner` retains its `sh -c` semantics because
   user-supplied commands expect shell semantics; the injection risk
   applies only to watchtower-generated shell-outs, not to user commands.
5. Fixed-argv shell-out to `git -C <tempDir> add -f -A`.
6. Commit via go-git against `<tempDir>`. The adapter's centralized
   `openRepo` helper uses
   `PlainOpenWithOptions(path, EnableDotGitCommonDir: true)` so go-git
   resolves the `.git` gitdir pointer file and reaches the shared object
   store. Live-repo behavior is identical.
7. Push via the existing `GitClient.Push(<tempDir>, ..., refPath)`.
   Named branch → existing push code path, no modification. Push is
   **skipped** if the LP ref already exists (rerun optimization preserved).
8. Discovery runs against `<tempDir>`:
   `DiscoverRecipes(<tempDir>)`, `snapProcessorsFromRepo(<tempDir>, ...)`.
   On reruns we still materialise + prepare to get a correct discovery
   tree; only the push is conditional. Deliberate trade-off: prepare cost
   in exchange for discovery/push parity.
9. Deferred cleanup fires.

### Shell-out safety

All watchtower-internal shell-outs (`git worktree add`, `worktree remove`,
`worktree prune`, `branch -D`, `add -f -A`) use
`exec.CommandContext(ctx, "git", argv...)` with fixed argv. No `sh -c`. No
string interpolation into a single command string. User-supplied inputs
(`<tempDir>`, `<sha>`, branch name) go into argv positions only.

The branch name `tmp-<prefix>-<sha8>` is built from `<prefix>` (configured,
validated) and a hex-only SHA prefix — always a safe git ref name.
`<sha>` is a full 40-char hex SHA. `<tempDir>` is `os.MkdirTemp` output.

## Security properties

What this change prevents:

- Accidental inclusion of pre-existing ignored files from the live
  worktree in the push (`.env`, IDE caches, credentials, stale artifacts).

What it does **not** prevent:

- Malicious or misbehaving `prepare_command` — the command runs as the
  user, with normal filesystem and environment access. This is not a
  sandbox.
- The command reading the user's SSH keys, environment, or other local
  secrets. It could, just as before.
- The command writing outside the temp worktree. If it does, those writes
  won't reach LP (only the tempDir is staged), but local damage is
  possible, same as today.

## Implementation summary

Changes live in:

- `internal/core/port/git.go` — two new methods on `GitClient`:
  - `CreateDetachedWorktree(ctx, repoPath, branch, sha) (worktreePath, cleanup, err)`
  - `ForceAddAll(ctx, worktreePath) error`
- `internal/adapter/secondary/git/client.go`:
  - Centralized `openRepo` helper with `EnableDotGitCommonDir: true`.
  - `runGit` helper wrapping `exec.CommandContext` with fixed argv.
  - Real implementations of the two new port methods.
- `internal/adapter/primary/frontend/build_prepare.go`:
  - `prepareAndPush` rewritten: new signature
    `(discoverPath, cleanup, err)` + `skipPush bool` parameter; runs prepare
    in the temp worktree when `prepare_command` is set.
  - New `pushFromLocalPath` helper encapsulates the old branch-dance for
    the no-prepare path.
  - `PrepareTrigger` now threads `discoverPath` into `DiscoverRecipes` and
    `snapProcessorsFromRepo`.

See `docs/agents/plans/build-prepare-worktree-isolation.md` for the
task-by-task implementation plan.

## Rejected alternatives (history)

Preserved here so future work does not re-propose them.

### v1: snapshot-diff in the live worktree

Before prepare, walk the tree (including ignored paths) and record
`path → (size, mtime)`. Run prepare. For any currently-ignored path whose
stat differs from the snapshot, force-add via `wt.Add(path)`.

**Why rejected** (adversarial review, two CRITICALs):

1. The snapshot-diff identifies "files that changed during the prepare
   window", not "files that prepare produced". Any editor/LSP/file watcher
   writing inside an ignored path during that window would be force-staged
   and pushed. The claimed security promise over "bypass .gitignore
   entirely" cannot survive contact with a real dev machine.
2. In the live worktree, converting prepare outputs to tracked files would
   cause the deferred checkout-back in `prepareAndPush` to **delete**
   those files (go-git's `MergeReset` behavior) before `DiscoverRecipes`
   and `snapProcessorsFromRepo` read the tree. Silent corruption of the
   very metadata we push.

### v2: detached linked worktree + raw three-method port + `AddAll` via `cmdRunner`

Same temp-worktree strategy, but with `git worktree add --detach`, three
separate port methods (`CreateWorktree`, `RemoveWorktree`, `ForceAddAll`),
and routing `git add -f -A` through the existing frontend `cmdRunner`.

**Why rejected** (adversarial review, two CRITICALs + others):

1. go-git's default `PlainOpen` is not linked-worktree-aware; Commit/Status
   inside the worktree fail with `object not found` until
   `EnableDotGitCommonDir` is enabled everywhere.
2. `--detach` produces a detached HEAD, and the existing `GitClient.Push`
   explicitly rejects detached HEAD. The fix is `-b <branch>` to create a
   named branch in the temp worktree, so the existing Push path works
   unchanged.
3. Three port methods leak worktree mechanics into the shared port.
   Collapsed to `CreateDetachedWorktree` returning a cleanup closure.
4. `cmdRunner` is `sh -c <raw string>` — injection-prone for internal
   watchtower-generated arguments. Internal shell-outs use fixed-argv
   `exec.CommandContext` instead.

### Other considered and rejected

- **Bypass `.gitignore` entirely on the live worktree.** Simplest, but
  leaks `.env` / credentials / IDE state. Rejected.
- **`cp -a` to a temp dir instead of `git worktree add`.** Copies ignored
  junk; does not share the object database; slow on large repos. Rejected.
- **Per-project `force_include: [...]` globs.** Burdens the user with
  listing what their prepare produces. Not needed once the temp-worktree
  strategy exists. Deferred; add only if a real case demands it.
- **Fetch the already-pushed LP ref on rerun instead of re-preparing.**
  Avoids re-running prepare on reruns, but adds a network fetch and new
  failure modes. Deferred — trade-off is acceptable because prepare is
  idempotent for typical `--clean` workflows.
