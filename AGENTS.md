# Agent Notes

Every feature implementation must be followed by a sync to the PLAN.md file at root.

Non-obvious behaviours encountered during development.

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
