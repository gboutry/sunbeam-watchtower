# Charmhub publisher auth flow

The Charmhub login flow is **four steps**, not two. Missing the third step
produces a stored credential that looks valid (`auth charmhub status` reports
authenticated) but every subsequent publisher call fails with `HTTP 400:
api-error: Invalid macaroon`.

## Steps

1. **Root macaroon request** — `POST https://api.charmhub.io/v1/tokens` with
   the description + requested permissions. Returns a JSON-serialized root
   macaroon with a third-party caveat against Candid.
   (`internal/adapter/secondary/charmhub/authenticator.go::requestRootMacaroon`.)
2. **Client-side discharge** — the client runs `httpbakery.DischargeAll`
   against Candid, driving a browser interaction. Result: a macaroon slice
   `[root, d1, d2, ...]` serialized as space-separated base64 binary via
   `storeauth/v1.SerializeMacaroonSlice`. (`pkg/storeauth/v1/discharge.go`.)
3. **Exchange** — `POST https://api.charmhub.io/v1/tokens/exchange` with:
   - Empty request body.
   - `Macaroons: <base64(json(slice))>` request header. The value is
     `base64.StdEncoding` over a JSON array whose elements are each macaroon's
     `macaroon.v2` JSON form (matches craft-store's convention).
   - Returns `{"macaroon": "<exchanged>"}`. This exchanged token is the short
     lived credential that `/v1/charm/...` endpoints accept.
     (`internal/adapter/secondary/charmhub/authenticator.go::ExchangeToken`.)
4. **Use** — every subsequent publisher call sends
   `Authorization: Macaroon <exchanged>`.
   (`internal/adapter/secondary/charmhub/collaborator.go`.)

## Where the exchange is wired

`auth.Service.SaveCharmhubCredential` (in `internal/core/service/auth/service.go`)
owns orchestration: it receives the discharged bundle from the client, calls
`CharmhubAuthenticator.ExchangeToken`, and persists the exchanged token via
`CharmhubCredentialStore.Save`. The stored `rec.Macaroon` is therefore the
exchanged publisher token, which is what every downstream caller wants.

`CharmhubAuthenticator` is a distinct port interface from
`SnapStoreAuthenticator` (no longer a type alias) because the Snap Store's
discharged bundle is the final credential — there is no exchange step there.

## Not covered here

- Automatic re-exchange / refresh on 401 from a `/v1/charm/...` call. The
  exchanged macaroon is short-lived, so long-running servers eventually need
  a refresh. Today the only fix is `watchtower auth charmhub login` again.
  Tracked separately.
- Snap Store collaborator APIs return `ErrCollaboratorsUnsupported`; see
  `snapstore-collaborator-api.md`.
