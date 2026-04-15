# Snap Store Collaborator API — Research & Design Note

Status: research complete; no adapter changes yet.
Scope: find the real Snap Store publisher API for per-snap collaborator
management, or document its absence and propose a fallback.

## TL;DR

A per-snap collaborator endpoint **does exist** — the
`dashboard.snapcraft.io/snaps/<name>/collaboration/` web UI drives it
every time an owner invites someone, and the macaroon caveat
`package_manage` is documented as "allows managing snap developers
(collaborators), restricted to `packages` values, if specified."

However, it is **not publicly documented**. The dashboard that serves
the collaboration UI is a closed-source Canonical-internal Django app,
and none of the open-source clients we grepped (`canonical/snapcraft`,
`canonical/snapcraft.io`, `canonicalwebteam.store-api`,
`canonical/craft-store`, `canonical/surl`,
`canonical-web-and-design/snapstore-api`) expose the call. The paths
the current adapter uses (`GET/POST /api/v2/snaps/{name}/collaborators`)
are **not found** in any of these references and are almost certainly
guessed.

**Decision: mark Snap Store collaborator management as unsupported
for now.** The port stays, the adapter returns a typed
`ErrUnsupported`, and the CLI/TUI/API surface a clear message pointing
operators at `https://dashboard.snapcraft.io/snaps/<snap_name>/collaboration/`.
We will not ship against guessed paths, and we will not invest the
capture-and-maintain work on an undocumented endpoint until there is a
concrete need.

Rejected alternatives:

- **`/api/v2/snaps/{name}/collaborators` (current code).** Undocumented,
  unverified, probably wrong. Must not ship.
- **Brand Stores API.** Requires `store_admin` (we do not have it) and
  grants store-wide access we do not want.
- **`snap-developer` assertion flow.** Offline-signed by the snap
  owner; unsuitable for a macaroon-only automated adapter.
- **Reverse-engineer via browser devtools.** Technically viable, but
  the endpoint is Canonical-internal and can change without notice.
  Defer until the feature is actually demanded.

Details and sources below.

## Investigation

### 1. Official Snap Store Dashboard API (v1 and v2)

Sources consulted:

- `https://dashboard.snapcraft.io/docs/reference/v1/snap.html`
- `https://dashboard.snapcraft.io/docs/reference/v1/account.html`
- `https://dashboard.snapcraft.io/docs/reference/v2/en/index.html`
- `https://dashboard.snapcraft.io/docs/reference/v2/en/stores.html`
- `https://dashboard.snapcraft.io/docs/reference/v1/macaroon.html`

Findings:

- v1 `/dev/api/` surfaces covered: `register-name`, `account`,
  `account/account-key`, `snap-push`, `snaps/{snap_id}/close`,
  `snaps/{snap_id}/validations`. **No collaborator endpoint.**
- v2 `/api/v2/` surfaces covered: `snaps/{name}/channel-map`,
  `snaps/{name}/releases`, `stores/...`, `validation-sets`,
  `confdb-schemas`. **No `/api/v2/snaps/{name}/collaborators` exists in
  the reference.**
- The macaroon permission `package_manage` is documented as "manage snap
  developers, restricted to packages values, if specified". This strongly
  implies an internal endpoint exists, but its path, method, and payload
  are not documented in the public reference.

### 2. Brand Stores API (documented, store-scoped)

`GET/POST /api/v2/stores/{store_id}/users` — manage user roles across an
entire brand store. Requires `store_admin` macaroon caveat and admin role
in the store.

POST payload shape:

```json
[
  {"email": "user@example.com", "roles": ["access"]},
  {"id": "account-id-string", "roles": ["view"]}
]
```

- Roles: `admin`, `review`, `access`, `view`.
- Roles are non-additive — POST replaces the full role set for the user.
- Users identified by email (case-insensitive) or account ID.

`POST/PUT /api/v2/stores/{store_id}/invites` — create or update pending
invitations. The response body from `GET /api/v2/stores/{store_id}`
includes both the resolved `users` array and an `invites` array with
pending/revoked/expired entries.

**Scope caveat:** this grants access to the entire brand store, not to
one snap. For a team whose work is spread across multiple unrelated snaps
in the global Ubuntu store (no brand store), this API is not applicable.

### 3. Snapcraft CLI (`github.com/canonical/snapcraft`)

File inspected: `snapcraft/store/client.py` (main branch).

Every `/dev/api/` and `/api/v2/` path used by the CLI was enumerated:

- `/dev/api/register-name/`, `/dev/api/account`, `/dev/api/account/account-key`
- `/dev/api/snap-push/`, `/dev/api/snaps/{snap_id}/close`
- `/dev/api/snaps/{snap_id}/validations`
- `/api/v2/snaps/{snap_name}/channel-map`, `.../releases`
- `/api/v2/confdb-schemas`, `/api/v2/validation-sets`

**No collaborator, invite, or developer-management endpoint.** The
snapcraft CLI has no `add-collaborator` command, confirming that the
snapcraft toolchain does not carry a client for this operation.

### 4. Snapcraft.io publisher UI (`github.com/canonical/snapcraft.io`)

File inspected:
`webapp/publisher/snaps/collaboration_views.py` (main branch).

The `/<snap_name>/collaboration` view's handler is effectively a stub:

```python
context = {
    "snap_id": snap_details["snap_id"],
    ...,
    "collaborators": [],
    "invites": [],
}
return flask.render_template("publisher/collaboration.html", ...)
```

It returns **hardcoded empty arrays** — the publisher UI renders a
collaboration page that lists nothing because no backing API call is
wired. This strongly suggests collaboration has been a long-running
unimplemented placeholder in the public publisher surface.

### 5. `canonicalwebteam.store-api` Python library

File inspected:
`canonicalwebteam/store_api/dashboard.py` (main branch).

Only store-wide invite methods exist:

- `invite_store_members()` — `POST /api/v2/stores/{store_id}/invites`
- `update_store_invites()` — `PUT /api/v2/stores/{store_id}/invites`
- `get_store_invites()` — `GET /api/v2/stores/{store_id}` (users/invites
  are embedded in the store payload)

No per-snap collaborator method.

### 6. Historical mechanism: `snap-developer` assertion

Older references describe a `snap-developer` assertion that delegates
publishing rights from one developer to another, signed by the snap's
owner. This is an assertion-based, offline-signed flow, not an HTTP
invite flow — and its store-side registration endpoint is not in the
public API reference. We did not find evidence that modern `snapcraft`
still exercises this path, and it cannot be driven from a macaroon-only
client without signing keys. Treat it as out of scope for our adapter.

## Chosen approach

1. Add a typed `ErrUnsupported` (or reuse an existing one) to the
   collaborator port.
2. Rewrite `internal/adapter/secondary/snapstore/collaborator.go` so
   both `ListCollaborators` and `InviteCollaborator` return
   `ErrUnsupported` wrapped with an operator-friendly message that
   includes the dashboard URL
   (`https://dashboard.snapcraft.io/snaps/<snap_name>/collaboration/`).
3. Drop the placeholder HTTP client plumbing and the guessed
   `/api/v2/snaps/{name}/collaborators` paths.
4. Update CLI/TUI/API error surfaces so "snap store: unsupported" is
   rendered cleanly without breaking sync for other adapters.
5. Update `PLAN.md` "Current Gaps" to state that Snap Store
   collaborator sync is intentionally unimplemented; link to this spec
   for the research context.
6. Keep the port contract unchanged so a future real implementation is
   drop-in. If/when the feature is reopened, the first step is the
   endpoint-capture plan in the appendix.

## Auth model (confirmed — kept for future reference)

- The `package_manage` macaroon permission is documented explicitly as
  managing snap developers (collaborators), constrained by `packages`.
- A macaroon with this caveat is obtained by `POST /dev/api/acl/`:

  ```json
  {
    "permissions": ["package_manage"],
    "packages": [{"snap_id": "<snap_id>"}],
    "channels": null,
    "expires": "<optional RFC3339>"
  }
  ```

- The response is a serialised root macaroon whose third-party caveat
  is discharged at `login.ubuntu.com`. This is the same exchange our
  `internal/adapter/secondary/snapstore` already performs for other
  operations — so no new auth plumbing is required beyond asking for
  the `package_manage` permission and the specific `snap_id`.
- An owner (publisher) can request `package_manage` on their own snaps.
  A plain collaborator typically cannot — the dashboard UI hides the
  "invite" action when the viewer isn't the publisher. For the target
  use case the operator is owner on the snaps in question, so this
  caveat is reachable.

What remains unknown: the **URL path and request/response shape** of
the endpoint that *consumes* that macaroon to list/invite collaborators.

## Appendix: endpoint capture plan (future work)

If/when Snap Store collaborator management is reopened, this is the
plan for discovering the real endpoint. Not required for the current
`ErrUnsupported` change.


Since no open-source Canonical client exposes this call, the only
reliable way to pin the contract is to capture traffic from the
dashboard UI while performing a real invite. Steps:

1. Sign in to `https://dashboard.snapcraft.io` as a snap owner.
2. Open DevTools → Network → XHR/Fetch, then navigate to
   `https://dashboard.snapcraft.io/snaps/<snap_name>/collaboration/`.
3. Capture the request issued when the page loads (this is the **list**
   call). Record the URL, method, status, request headers (especially
   `Authorization`/`X-Device-Authorization`), and response body.
4. Invite a real or throwaway test account (or a staging account) and
   capture the **invite** request. Record URL, method, request body,
   status, and response body. Also capture the subsequent list call
   that reflects the new pending invite.
5. If a "remove collaborator" control exists, capture that too —
   revocation is in scope for the sync flow.
6. Repeat all of the above on staging
   (`https://dashboard.staging.snapcraft.io`) to verify path parity.
7. Export as HAR and attach to the follow-up implementation PR, or
   paste the decoded requests into a private issue. Redact any
   macaroon values.

**Acceptance criteria for proceeding to implementation:**

- List endpoint URL confirmed; 200 response schema captured.
- Invite endpoint URL confirmed; successful response schema captured.
- At least one failure case captured (e.g. invalid email, duplicate
  invite) to confirm error body shape.
- If possible, one capture taken with a macaroon that has
  `package_manage` but not `package_upload` — to confirm that
  `package_manage` alone is the right caveat.

Until these criteria are met, the adapter must not be rewired.

## Likely shape (to be confirmed by capture)

These are informed guesses based on dashboard conventions, **not**
prescriptions. Do not code against them until capture confirms.

- Base URL: `https://dashboard.snapcraft.io`. The `/dev/api/` prefix
  (used by historical v1 per-snap endpoints) is a stronger candidate
  than `/api/v2/` (which is mostly reserved for channel-map/releases
  and brand stores). A path like `/dev/api/snaps/<snap_id>/developers`
  fits the existing v1 style; so does
  `/dev/api/snaps/<snap_id>/acl`. The identifier is most likely
  `snap_id`, not `snap_name` — v1 endpoints uniformly take `snap_id`.
- List: `GET` returning a JSON object with (at least) accepted
  collaborators (account id, username, display name, email) and
  pending invites (email, status, timestamps).
- Invite: `POST` with a small JSON body — `{"email": "..."}` or
  `[{"email": "...", "role": "..."}]`. The dashboard has no role picker
  for per-snap collaborators, so a roleless body is plausible.
- Remove: `DELETE` keyed on account id or invite id. Possibly a `PUT`
  with a replacement list — the Brand Stores API uses the
  "replace-the-set" idiom, so the per-snap endpoint might too.
- Auth: `Authorization: Macaroon <root>`
  + `X-Device-Authorization: Discharge <discharge>`, both derived from
  the `package_manage`-caveated macaroon described above.

The capture will either confirm this shape or correct it. Either way,
the design note must be updated with the captured contract before code
changes land.

## Rejected alternatives

- **`/api/v2/snaps/{name}/collaborators` (current code).** Not
  documented anywhere, not used by any open-source Canonical client,
  and — given the convention that v2 per-snap endpoints use `name` but
  v1 uses `snap_id` — probably not the shape the real endpoint takes.
  Keeping this code is worse than deleting it.
- **Brand Stores API.** Requires `store_admin`, which we do not have,
  and grants store-wide access we do not want. Not usable.
- **`snap-developer` assertion flow.** Offline-signed, requires the
  owner's signing key on the local machine. Not suitable for an
  automated sync adapter that holds only a macaroon.
- **Defer entirely.** Rejected — the capability is reachable from the
  user's actual permissions (owner), we just don't have the URL yet.

## Caveats

- **`snap_id` vs `snap_name`.** If the endpoint takes `snap_id` (as
  expected for `/dev/api/` paths), the adapter needs a resolver from
  `snap_name` → `snap_id`. The v1 account info endpoint already
  returns this mapping.
- **Owner-only operation.** If the operator happens to be a
  collaborator (not owner) on a given snap, `package_manage` requests
  for that snap will fail at `/dev/api/acl/`. The adapter should
  surface a typed "not owner" error cleanly so the CLI/TUI/API can
  explain instead of dumping HTTP status codes.
- **Undocumented contract risk.** Any captured endpoint is Canonical-
  internal and can change without notice. The adapter should log the
  raw response body on any unexpected status, and an integration test
  against staging should run before each release.
- **Email disambiguation.** If the Snap Store invite flow matches
  Brand Stores' behaviour, an email that maps to multiple accounts may
  require an account-id fallback. Worth probing during capture.
- **Staging availability.** Capture on staging first — it is the only
  safe environment to trigger real invites with synthetic accounts.

## Follow-up implementation tasks

Now (this round):

1. Add `ErrUnsupported` to the collaborator port.
2. Rewrite `internal/adapter/secondary/snapstore/collaborator.go` to
   return `ErrUnsupported` from both methods, with a message
   referencing the dashboard URL.
3. Update CLI/TUI/API error surfaces so this is rendered cleanly.
4. Drop unused HTTP client plumbing and the guessed endpoint strings.
5. Update `PLAN.md` "Current Gaps" accordingly.

Future (only when the feature is reopened):

1. Execute the capture plan in the appendix against staging, then
   production.
2. Update this spec with the captured contract.
3. Implement against the captured endpoint; add a `snap_id` resolver
   and a `package_manage`-scoped macaroon exchange.
4. Add integration tests against staging and recorded-fixture unit
   tests.

## Sources

- Snap Store Dashboard v2 reference:
  `https://dashboard.snapcraft.io/docs/reference/v2/en/index.html`
- Brand Stores v2 reference:
  `https://dashboard.snapcraft.io/docs/reference/v2/en/stores.html`
- Snap Store Dashboard v1 snap reference:
  `https://dashboard.snapcraft.io/docs/reference/v1/snap.html`
- Snap Store Dashboard v1 account reference:
  `https://dashboard.snapcraft.io/docs/reference/v1/account.html`
- Macaroon reference (permissions list):
  `https://dashboard.snapcraft.io/docs/reference/v1/macaroon.html`
- snapcraft CLI store client:
  `https://github.com/canonical/snapcraft/blob/main/snapcraft/store/client.py`
- snapcraft.io publisher UI collaboration view:
  `https://github.com/canonical/snapcraft.io/blob/main/webapp/publisher/snaps/collaboration_views.py`
- canonicalwebteam.store-api dashboard client:
  `https://github.com/canonical/canonicalwebteam.store-api/blob/main/canonicalwebteam/store_api/dashboard.py`
