# ADR 025 — On-behalf identity

> Status: **Accepted** · Date: 2026-11-22 · Amended 2026-11-22: on-behalf requests are coach-scoped · Amended 2026-11-22: signed short-lived tokens

## Context

Homelab-mcp calls RepLog for a person PocketID has already verified. A
per-user `rlcst_` token (ADR 022) binds one bearer to one account. That
forces a token per caller. The connector already knows the caller's
verified email. RepLog should accept that email and act as the matching
user, with one shared credential that is not `REPLOG_SERVICE_TOKEN`.

`users.email` is already `UNIQUE COLLATE NOCASE` and the admin user form
already edits it. The service-token issue and revoke UI stays.

## Decision

- `REPLOG_ON_BEHALF_CREDENTIAL` is the dedicated secret. Empty, or a value
  that starts with `rlcst_`, disables the path. The compare is SHA-256 of
  both sides, then `subtle.ConstantTimeCompare`. The secret is never logged,
  stored, or returned.
- Homelab-mcp sends `Authorization: Bearer <REPLOG_ON_BEHALF_CREDENTIAL>`
  and `X-RepLog-On-Behalf-Of: <verified email>`. The bearer must not be an
  `rlcst_` token. An `rlcst_` bearer is still the service-token path.
- RequireAuth reads the email header and deletes it before the handler.
  The header is honored only when the bearer matches the on-behalf secret.
  A browser session, an `rlcst_` token, and the coach-service token ignore
  it. The credential check is what stops an outside client. Ingress is not
  the control.
- A match maps the email with `GetUserByEmail` (case-insensitive). The
  request then carries a copy of that user with `IsAdmin` cleared and an
  on-behalf flag, without the service-credential flag. Coach and athlete
  checks are the ones a browser session of a non-admin with the same
  roles passes: a coach reaches athletes whose `coach_id` is that user,
  and anyone reaches their own linked athlete. It does not write a
  session. A missing or unknown email is 403. There is no fallback to the
  coach-service user or any other default.
- On-behalf requests are scoped in `RequireAuth`, before any handler, by
  an allowlist (`onBehalfRouteAllowed`): `GET /api/me`,
  `GET /api/dashboard`, `GET /api/athletes`, `GET /api/athletes/{id}`,
  any method under `/api/athletes/{id}/`, `GET` under `/api/exercises`,
  `/api/programs`, and `/api/equipment`, and `GET /api/reviews/pending`.
  Everything else is 403, so a new route stays closed until it is added
  on purpose. That shuts token issue and revoke, user and login-token
  management, admin settings, catalog import, impersonation, preferences,
  avatars, and logout, even when the mapped user is an admin. Paths must
  be clean and unescaped. `adminBrowser` also refuses an on-behalf request
  directly.
- Each success appends `audit_log` action `on_behalf` for that user, with
  details `on_behalf=true proof=shared` or `on_behalf=true proof=signed`.
  The credential, the token, and the header value are not written.
- Migration `0020` once backfilled one user's address. It is now a no-op
  and inserts nothing. Addresses are set in the admin UI.

`rlcst_` tokens and `REPLOG_SERVICE_TOKEN` are unchanged, including the
coaching-write refusals on that credential.

## Consequences

- A leaked database still has no usable on-behalf secret. The secret lives
  only in the process environment and in the connector.
- A browser that sends the email header does not change who the session is.
- A leaked on-behalf secret reaches coach data for any mapped user, but not
  admin actions. Admin work, including token issue and revoke, stays a
  browser session.
- A coach action under `/api/athletes/{id}/` that is refused to an `rlcst_` token
  (for example training-max writes or generation execute) is
  still a coach action here. The scope removes admin, not coaching.
- An unknown PocketID email does not become the coach-service account.
- PocketID does not speak RFC 8693 token exchange. The amendment below
  replaces the shared secret with a token homelab-mcp signs itself.

## Amendment: signed short-lived tokens

A shared secret is a bearer that works for any email, forever, from
anywhere it leaks. Homelab-mcp now signs one token per call instead.

- Homelab-mcp holds an Ed25519 private key. Per RepLog call it signs a
  JWT (`alg` `EdDSA`) with `iss` `homelab-mcp`, `aud` `replog`, `sub` and
  `email` set to the verified PocketID email, `act` `{"sub":
  "homelab-mcp"}`, `iat`, `exp` (about 60 s later), and a random `jti`.
  It sends `Authorization: Bearer <jwt>`. It may also send
  `X-RepLog-On-Behalf-Of`; when present it must name the same address.
- RepLog holds only the public key (`REPLOG_ON_BEHALF_JWT_PUBLIC_KEY`, PEM
  SPKI, or `REPLOG_ON_BEHALF_JWT_PUBLIC_KEY_FILE`). It accepts only
  `EdDSA` with exactly one signature, and checks `iss` (default
  `homelab-mcp`), `aud` `replog`, `exp`, and `iat` with 30 s skew. It
  requires `jti`, refuses a lifetime over 5 minutes, refuses an `iat` in
  the future, requires `act.sub` to be the actor (default `homelab-mcp`),
  and requires `email` to equal `sub` (case-insensitive). A `jti` is
  accepted once per process; entries are dropped after they expire.
- A token that passes maps `email` exactly as above. The route allowlist,
  admin clearing, 403 for an unknown email, and the audit row are the
  same. Any failed check is 401 and logs no claim or token.
- `REPLOG_ON_BEHALF_MODE` picks the accepted proof: `shared` (default,
  shared secret only), `both` (signed token or shared secret), or
  `signed` (signed token only). Any other value accepts neither. `shared`
  stays the default for one release so the code can ship before the keys
  exist. The rollout is `both`, then `signed`, then the shared secret and
  this switch are removed.

Consequences of the amendment:

- RepLog stores no secret for this path. A leaked public key signs nothing.
- A captured token works for one call, for one email, for at most about a
  minute, and only against `aud` `replog`.
- A leaked private key still reaches coach data for any mapped user until
  the key is rotated, but not admin actions. Rotation is a new key pair in
  1Password and a pod restart on both sides.
- Replay protection is per replica and in memory. RepLog runs one replica.
  The short `exp` bounds what a restart forgets.
