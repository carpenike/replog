# ADR 003: Auth — Session Cookies with `alexedwards/scs`

**Status:** Accepted
**Date:** 2026-02-12

## Context

RepLog needs authentication for a single-family deployment (1–2 users). The requirements specify: simple login, session persistence across browser restarts, and auto-creation of the admin account on first run.

We need the simplest auth approach that is still secure, with no external dependencies (no SSO, no OAuth, no Redis).

## Decision

- **Password hashing:** `bcrypt` via Go's `golang.org/x/crypto/bcrypt`
- **Session management:** `alexedwards/scs` with SQLite session store (or signed cookie store)
- **Session transport:** HTTP cookies with `HttpOnly`, `SameSite=Lax`, configurable `Secure` flag
- **Session lifetime:** 30 days (long-lived — family device, low-risk)
- **Bootstrap:** On startup, if `users` table has zero rows, create an admin account from `REPLOG_ADMIN_USER`, `REPLOG_ADMIN_PASS`, and `REPLOG_ADMIN_EMAIL` environment variables with `is_coach = 1`. Fail startup if env vars are missing and no users exist.

## Rationale

- `alexedwards/scs` is a well-maintained, minimal session library with SQLite support. No framework lock-in.
- Cookie-based sessions avoid a separate session table if using signed cookies, or provide server-side expiry if using the SQLite store. Either works at this scale.
- `bcrypt` is the standard for password hashing in Go — no configuration needed, safe defaults.
- 30-day sessions are appropriate for a family device. No need for refresh tokens or short-lived JWTs.
- No roles/permissions beyond coach vs non-coach (`is_coach` flag). All coaches can do everything; non-coaches can only manage their linked athlete.
- A non-coach user without a linked athlete profile sees an informative message and cannot take other actions.

## Consequences

- **Positive:** Zero external dependencies for auth. Single binary, no Redis/Memcached.
- **Positive:** `HttpOnly` + `SameSite=Lax` prevents XSS and CSRF for cookie-based sessions.
- **Negative:** No password reset flow in v1 — if the admin forgets their password, they must re-set it via env vars or direct DB update. Acceptable for a family app.
- **Accepted risk:** `Secure` flag must be off for HTTP-only local dev, on for production behind Caddy. Configurable via env var or flag.
