# ADR 025 — On-behalf identity

> Status: **Accepted** · Date: 2026-11-22 · Amended 2026-11-22: on-behalf requests are coach-scoped

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
  details `on_behalf=true`. The credential and the header value are not
  written.
- Migration `0020` sets `ryan@ryanholt.net` only when a user named `ryan`
  exists, that email is empty, and the address is not already taken. It
  does not insert users. Any other address is set in the admin UI.

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
- PocketID does not speak RFC 8693 token exchange, so this handoff stays a
  shared credential plus the verified email rather than an exchanged token.
