# Agent Notes

Every feature implementation must be followed by a sync to the PLAN.md file at root.

Non-obvious behaviours encountered during development.

## Runtime model

### Stateful features are persistent-server-first

Auth flows, async operations, and any workflow that claims durable state must be designed around persistent server semantics first. Embedded mode is convenience-only and must not pretend to preserve durable state across invocations.

### Local paths stay local

For split workflows, client-side code may inspect or prepare local worktrees, but the server must receive prepared references rather than raw local filesystem access.

## Architecture guardrails

### Frontends should reuse shared frontend/runtime seams

CLI, TUI, and API work should go through `internal/adapter/primary/frontend` and `internal/adapter/primary/runtime` where possible instead of adding new raw `pkg/client` call sites or wiring business flows directly out of `internal/app`.

### Operation access classification is mandatory

Every new or modified user-invoked operation must be assigned one canonical `ActionID` in the shared frontend action catalog under `internal/adapter/primary/frontend`.

Classification rules:

- classification lives in the shared frontend action catalog, not only in CLI, TUI, or API wrappers
- every action must declare:
  - mutability
  - local effect
  - runtime requirement
  - MCP export policy
- dry-run variants must be separate action IDs whenever the effective mutability changes
- auth/session operations are always mutating
- local-only output generation may still be classified as read-only
- command/action tests must be updated whenever classification changes
- if a new or changed operation expands the authorization surface, `PLAN.md` must be updated in the same change

Reviewer checklist:

- action catalog updated intentionally
- CLI and TUI mappings updated where relevant
- tests cover the new or changed classification
- MCP export policy was chosen deliberately

### Adapter boundaries are enforced mechanically

Primary adapters must not import secondary adapters directly, `internal/adapter/*` packages must not define interfaces, and boundary changes should come with matching architecture-test updates instead of one-off exceptions.

## Quality gates

### Changed-package coverage is part of the merge contract

`tools/coverageguard`, `pre-commit`, and CI enforce changed-package coverage floors. Feature work in a guarded package should normally include the tests needed to keep that package above its threshold.

## Launchpad API

### Creation endpoints return empty bodies

All LP creation endpoints (`ws.op=new`, `ws.op=new_project`) return **HTTP 201 with an empty body** and a `Location` header. Do **not** use `json.Unmarshal` on the response. Instead, `POST` to create, then `GET` the resource by its known path.

Affected endpoints: `/+git`, `/projects`, `/+rock-recipes`, `/+charm-recipes`, `/+snaps`.

### The `/~` path is not a valid API endpoint

`GET /~` returns HTML, not JSON. To fetch the authenticated user, use `/people/+me` (the `me_link` from the service root).

### Owner/project parameters expect full self_links

LP form parameters like `owner` and `target`/`project` expect full API self_links (e.g. `https://api.launchpad.net/devel/~username`), not plain names.

### Git SSH URLs use `git+ssh://` scheme

LP's `git_ssh_url` field returns `git+ssh://` URLs. go-git (and many other libraries) only support `ssh://`. Replace `git+ssh://` → `ssh://` before use.

### Git SSH URLs omit the username

LP's `git_ssh_url` has no user component, but push requires `<lp_username>@` in the URL. Inject the LP owner username before using the URL.

### Project creation requires `licenses`

`POST /projects` with `ws.op=new_project` fails if the `licenses` field is omitted. Pass at least one value (e.g. `"Apache Licence"`).

### Date/time parameters must be in UTC

LP rejects date/time query parameters (e.g. `created_since`, `created_before`) that include a non-UTC timezone offset. Always convert to UTC before formatting: `t.UTC().Format(time.RFC3339)`.

### `getRefByPath` returns refs without `self_link`

LP's `getRefByPath` custom operation may return a `GitRef` object with an empty `self_link`. Construct it manually: `<repoSelfLink>/+ref/<refPath>`.

### Recipe `git_ref` requires a real branch

LP recipe creation rejects bare SHA refs. Always push to a named branch (e.g. `refs/heads/tmp-<sha8>`) and use that as the `git_ref`.

## go-git

### `HEAD` in push refspecs silently skips objects

go-git does not reliably resolve `HEAD` in push refspecs. A push with `HEAD:refs/heads/<branch>` may report success (or `NoErrAlreadyUpToDate`) without transferring any objects. Always resolve `HEAD` to the concrete branch ref (e.g. `refs/heads/main`) before building the refspec.

### `NoErrAlreadyUpToDate` is not an error

go-git's `Push` returns `NoErrAlreadyUpToDate` when the remote already has the ref at the same commit. This is a no-op, not a failure — treat it as success.

## Huma (API framework)

### Slices and maps are required by default

Huma treats non-pointer `[]string`, `map[...]`, and `bool` fields as **required** — in both `Body` structs and `query`/`header` params. Omitting them from the request triggers a **422 Unprocessable Entity** validation error. Add `required:"false"` to every optional slice/map/bool field. The `omitempty` JSON tag does **not** affect Huma's required inference.

### Use correct HTTP status codes for errors

`huma.Error422UnprocessableEntity` should only be used for actual validation/format issues. For missing resources use `huma.Error404NotFound`, for server failures use `huma.Error500InternalServerError`. Misusing 422 for "not found" scenarios misleads both clients and developers into debugging request format issues.

## Excuses feeds

### Ubuntu team ownership comes from `update_excuses_by_team.yaml`

The main Ubuntu excuses feed (`update_excuses.yaml.xz`) does **not** carry team ownership directly. Fetch the companion `update_excuses_by_team.yaml` feed and merge its per-package team data if `--team` needs to work.

### `update_excuses_by_team.yaml` is not normal YAML

Ubuntu's team feed contains Python-specific YAML tags/aliases (for example `!!python/object/apply:collections.defaultdict`, anchors, and aliases) that `gopkg.in/yaml.v3` does not cleanly unmarshal at scale. Parse it defensively for the fields you need instead of assuming a normal schema-safe YAML decode.

## OpenTelemetry

### Cache-backed domain telemetry is the default

Domain metrics should expose cached or internal state by default because it is cheaper and more stable to scrape. If a domain collector needs live upstream fan-out, it must be explicitly opt-in through `otel.metrics.domain.live_systems`.

### Domain changes must evaluate telemetry impact

If a domain adds or changes operationally relevant state, counts, or snapshots, update the telemetry collector and tests in the same change, or document clearly why telemetry is intentionally deferred. New domains must be classified as one of:

- cache/internal collector
- live collector
- intentionally no telemetry yet

### Live collectors need explicit review

If a change adds a live collector or broadens its upstream fan-out, update all of:

- `otel.metrics.domain.live_systems` documentation
- refresh interval defaults/expectations
- telemetry tests
- `PLAN.md`

### Keep metric labels bounded

Never add unbounded telemetry labels. In particular, do not use raw IDs, usernames, SHAs, URLs, or other free-form values as metric labels. Prefer stable gauges/counters and put changing string metadata in traces/logs instead of metrics.

### Preserve outbound tracing coverage

If an HTTP-backed upstream client changes or a new one is introduced, ensure outbound tracing still wraps the client construction path. Do not scatter OTel calls across business logic.

### Keep observability dependencies confined

OpenTelemetry and Prometheus imports stay confined to `internal/adapter/secondary/otel`. A new direct import outside that package is a bug unless the architecture rules are intentionally updated with matching tests.
