# ADR 014 — Per-Account Login Lockout

**Status:** Accepted
**Date:** 2026-05-12

## Context

[ADR 003](003-auth-sessions.md) settled on bcrypt + scs sessions. The
2026-05-12 security audit (issue #6) made `models.Authenticate`
constant-time on the unknown-username path so that an attacker cannot
enumerate accounts by response time, and added a per-IP rate limiter on
`/api/login` (10 attempts per minute).

That leaves one structural gap: **the rate limit is per-IP, not
per-account**. A patient attacker who knows N usernames and rotates
through them once every six seconds stays under the per-IP cap forever.
Even without that, an attacker with multiple source IPs can spray a
small list of common passwords against every account at full speed.
There is no per-account failure counter today, so a successful guess
is silently indistinguishable from a legitimate first-try login.

For RepLog's threat model — small self-hosted family deployment with
one admin and a handful of accounts — the realistic risk is **password
spray** (a few common guesses against every account) rather than a
full brute-force of any one account. A per-account lockout addresses
spray directly: even if the attacker knows the username, they get a
small fixed budget of guesses before that account becomes unavailable
to them.

## Decision

Add a per-account failure counter to the `users` table and integrate it
into `models.Authenticate` so that:

- After **5 consecutive wrong-password attempts** against a known
  account, that account is locked for **15 minutes**.
- A successful login resets the counter immediately.
- The lockout window slides — every additional wrong guess while
  locked extends `locked_until` by 15 minutes from the moment of the
  guess. This prevents an attacker from waiting out the lockout in
  the background while continuing to test other accounts.
- An admin can manually clear a lockout via the existing user
  management UI (a future ticket; for now, manual `UPDATE` is
  acceptable).
- Unknown-username attempts do **not** consume any per-account
  budget — that path is still constant-time and stateless. Locking
  accounts based on attacker-supplied usernames would be a trivial DoS.

### Schema

Two new columns on `users` (pre-prod migration policy from
[ADR 002](002-migrations.md): edit `0001_initial_schema.sql` in place):

```sql
failed_login_count   INTEGER NOT NULL DEFAULT 0,
locked_until         DATETIME,
```

No partial index — the table is small (single-family scale) and lookups
are by username, which already has a unique index.

### Authenticate behavior

The model returns a new sentinel error `ErrLocked` with an embedded
unlock time. The login handler maps it to:

```
HTTP/1.1 429 Too Many Requests
Retry-After: <seconds remaining>
Content-Type: application/json

{"error":"account temporarily locked — try again later","code":429}
```

The check order inside `Authenticate` is:

1. Look up the user by username. If not found, run a dummy bcrypt
   compare (existing constant-time defense) and return `ErrNotFound`.
   No row update.
2. If the user exists but `locked_until > now()`, return `ErrLocked`
   **without** comparing the password. This makes the lockout window
   feel instantaneous and prevents "still locked, but right password"
   from being a useful timing oracle. Also extends `locked_until`.
3. If the password is wrong, increment `failed_login_count`. If the
   incremented value reaches 5, set `locked_until = now() + 15m`.
4. If the password is right, reset `failed_login_count = 0` and
   `locked_until = NULL`.

### Why these numbers

- **5 attempts** is permissive enough that a typo'd password on an
  intermittently-used account does not trigger a lockout, but tight
  enough that a 100-password spray locks within the first try per
  account. For a family deployment, more than 5 wrong passwords in a
  row almost always means a real attack or a forgotten password.
- **15 minutes** matches what most consumer products do (Apple ID,
  GitHub) and keeps the recovery loop short for a real user. Long
  enough to thwart automated retries, short enough that a locked-out
  user can resolve it themselves with patience and not require admin
  intervention.
- Both values are constants in code today. If they need tuning later,
  promote them to `app_settings` rows (the registry already supports
  numeric settings).

## Rationale

### Why per-account, not just per-IP

Per-IP works against single-IP brute-force but not against the
realistic threat (a botnet, or a single attacker behind CGNAT spraying
common passwords across all known accounts). The two checks compose:
per-IP caps the global request rate, per-account caps the damage any
one account can take.

### Why reject locked accounts at the application layer, not the proxy

Lockout state is per-account, which the reverse proxy does not have
access to. Putting the check in `models.Authenticate` is the only place
where the username is known and the password is about to be checked —
exactly the right point to gate.

### Why not lock on unknown usernames

If the lockout key were the *attacker-supplied* username, an attacker
could:

1. Discover (or guess) target usernames.
2. Send N+1 wrong-password attempts to lock every target account.

That turns the security feature into a denial-of-service primitive
against legitimate users. The existing constant-time response on
unknown usernames means the attacker cannot tell which of their guesses
are real accounts anyway, so this exclusion costs nothing in
information leakage.

### Why slide the window on attempts-while-locked

A static lockout window lets the attacker run a slow spray: lock an
account, move on to the next account for 15 minutes, return after the
window expires, and resume guessing. Sliding the window means every
attempt during a lockout extends the lockout, so the attacker cannot
spread guesses across accounts to keep all of them barely-active.

### Why 429 instead of 401

`401 invalid username or password` is a lie when the password might
have been right (we did not check). `429 Too Many Requests` plus
`Retry-After` is honest, and any well-behaved client (browser SPA,
curl, automation) understands the convention. An attacker who is
spraying does not care which status code they get; they will just move
on to the next target.

### Why not delegate to fail2ban or a WAF

Both work fine for HTTP-layer abuse but neither sees inside the request
to know which *account* is being attacked. Per-account lockout
specifically requires application knowledge of which row in `users` the
attempt was aimed at. Plus, RepLog is intentionally a single-binary
deployment with no required external infrastructure, so building this
into the binary keeps the operational story unchanged.

## Consequences

### Positive

- Password spray (the realistic attack) is structurally limited to
  ~5 guesses per account per 15 minutes.
- Combines with the per-IP rate limiter and the constant-time
  unknown-username path to make `/api/login` boring for an attacker.
- One small migration; no new tables.
- No new env vars, no new config rows, no new background jobs.

### Negative

- A real user who fat-fingers their password 5 times in a row gets a
  15-minute timeout. For a family app this is rare; if it ever
  becomes annoying we promote the constants to `app_settings`.
- Admin recovery from a lockout requires either waiting 15 minutes or
  a manual `UPDATE users SET failed_login_count = 0, locked_until =
  NULL WHERE username = ?`. A future admin-UI button is tracked but
  not in this ADR's scope.
- The lockout state lives in `users`. A user-row reset (e.g. password
  change by admin) must clear both columns; the existing
  `UpdatePassword` path is updated to do this.

### Neutral

- The lockout is per-account, not per-account-per-IP. If the same
  legitimate user is also subject to attack from a hostile IP, both
  the user and the attacker share the lockout. This is the right
  trade-off — alternative would let attackers DoS the user via
  reverse spoofing once we trust X-Forwarded-For.
- Magic-link / token login (`POST /api/auth/token/{token}`) is **not**
  gated by this lockout. Those tokens are single-use cryptographic
  values; account state should not block a legitimate token-bearer.
  Same reasoning for passkey login.
