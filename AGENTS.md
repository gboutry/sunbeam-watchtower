# Agent Notes

Non-obvious behaviours encountered during development.

## Launchpad API

### Creation endpoints return empty bodies

All LP creation endpoints (`ws.op=new`, `ws.op=new_project`) return **HTTP 201 with an empty body** and a `Location` header. Do **not** use `json.Unmarshal` on the response. Instead, `POST` to create, then `GET` the resource by its known path.

Affected endpoints: `/+git`, `/projects`, `/+rock-recipes`, `/+charm-recipes`, `/+snaps`.

### The `/~` path is not a valid API endpoint

`GET /~` returns HTML, not JSON. To fetch the authenticated user, use `/people/+me` (the `me_link` from the service root).

### Owner/project parameters expect full self_links

LP form parameters like `owner` and `target`/`project` expect full API self_links (e.g. `https://api.launchpad.net/devel/~username`), not plain names.

### Project creation requires `licenses`

`POST /projects` with `ws.op=new_project` fails if the `licenses` field is omitted. Pass at least one value (e.g. `"Apache Licence"`).
