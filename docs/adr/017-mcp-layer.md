# ADR 017 — MCP Layer: RepLog as an OAuth Resource Server

> Status: **Accepted** · Date: 2026-05-27

## Context

RepLog should be usable from the Claude apps (web, desktop, and crucially
**iOS/Android**) as a training-session companion: read live logbook state,
log work as it happens between sets, and trigger a program draft the coach
later approves. The Model Context Protocol (MCP) is the way Claude reaches
external tools.

Two hard constraints shape the design:

1. **The app never automates coaching (ADR 007 / 015).** A human makes every
   progression decision; LLM output is a proposal a human reviews. An MCP
   surface must not become a back door that lets an agent write the
   coaching-decision tables unattended.
2. **Single static binary on NixOS (ADR 001).** Whatever we add must not drag
   in CGO, a second runtime, or a heavyweight dependency tree.

The host already runs a spec-clean **OAuth 2.1 Authorization Server** —
`homelab-mcp` (Python/FastMCP) at `mcp.holthome.net`, federating login to
PocketID and minting RS256 JWTs that the Claude apps already trust (RFC 7591
dynamic client registration, RFC 8414 AS metadata, RFC 9728 protected-resource
metadata, PKCE, JWKS at `/oauth/jwks.json`). The original spec assumed RepLog
would build its own MCP server and its own OAuth AS; review (HOF-004) found
that the AS work — the largest and riskiest chunk — was already shipped and
battle-tested. That discovery is what this ADR records the consequences of.

The webui's authentication (session cookies via scs, ADR 003) is unchanged.
The MCP surface is an **additive, parallel** authentication path; non-MCP
users and the entire webui are unaffected.

## Decision

### Runtime — RepLog is a resource server, not an MCP server

The MCP transport, the OAuth dance, and the tool catalog all live in
`homelab-mcp` (the `src/homelab_mcp/tools/replog.py` module, auto-discovered by
its tool registry). RepLog exposes a parallel **`/api-mcp/*`** HTTP surface
gated by bearer-token verification and otherwise reuses its existing handlers.

Consequences of this split:

- **No Go MCP SDK dependency** and **no OAuth AS code** in the RepLog binary —
  ADR 001's single-static-binary identity is preserved. The only new direct
  dependency is `github.com/golang-jwt/jwt/v5`, which was already transitively
  present via `go-webauthn`.
- The tool surface is thin Python httpx wrappers over RepLog's REST endpoints —
  the same shape as the existing `cooklang` / `gatus` tools in `homelab-mcp`.

### Auth — JWT-as-identity, audience-scoped per resource

Identity flows end-to-end as a JWT; RepLog stores **no** per-user bearer token.

1. The Claude app holds a JWT from `homelab-mcp`'s AS (`aud=https://mcp.holthome.net`).
2. For each tool call addressed to RepLog, `homelab-mcp` re-mints a **short-TTL
   (60 s) RS256 JWT** carrying the original caller's `sub` and `email`, with
   **`aud=https://replog.holthome.net`**, signed with the AS's existing RSA key.
3. RepLog's `internal/middleware/bearer.go` verifies the token against the AS's
   JWKS (fetched once, cached ~1 h, lazy refetch on `kid` cache miss with a
   stale-key fallback on refresh failure). It pins **RS256** (rejecting any
   other `alg` to close the HS256 confusion attack), asserts `iss`, `aud`,
   `exp`, `iat`, and requires `sub`.
4. It resolves the `email` claim to a `*models.User` via `models.GetUserByEmail`
   (case-insensitive, mirroring the column's `COLLATE NOCASE`). An **empty or
   absent `email` claim is rejected with `401`** — `users.email` is nullable
   and `UNIQUE` permits multiple NULLs, so a null-claim lookup must never
   resolve a row.
5. The resolved user is attached to the request context under the **same
   `UserContextKey` / `PrefsContextKey`** the scs-cookie middleware uses, so
   every downstream handler (`UserFromContext`, `CanAccessAthlete`,
   `CanManageAthlete`, audit columns) works unchanged.

**RepLog never holds, transmits, or sees the AS's private signing key.** The
`aud` scoping means a token minted for `mcp.holthome.net`'s own tools cannot be
replayed against RepLog. JWKS rotation on the AS requires zero RepLog code or
config change. This shape is exactly what RFC 8414 / 9728 describe for one AS
fronting multiple resource servers.

Configuration (env, with defaults): `REPLOG_MCP_AS_ISSUER`
(`https://mcp.holthome.net`), `REPLOG_MCP_AS_JWKS_URL`
(`${ISSUER}/oauth/jwks.json`), `REPLOG_MCP_AUDIENCE`
(`https://${REPLOG_BASE_URL_HOST}`).

### Per-user gating — `users.mcp_enabled`, default-deny

A `users.mcp_enabled INTEGER NOT NULL DEFAULT 0 CHECK(mcp_enabled IN (0,1))`
column (migration `0005`, additive per ADR 002) gates access on top of a valid
JWT. The bearer middleware refuses with **`403 mcp-not-enabled`** after
resolving the user but before attaching context. An admin toggles it per user
(`PUT /api/users/{userID}/mcp`, admin-gated in-handler). Default-deny means the
rollout grants no one access by accident.

### Tool tiers — the doctrine boundary, encoded as a curated route list

`/api-mcp/*` mounts **only** an explicit, curated allow-list of session-free
handlers (asserted by a `chi.Walk` mount-list test). The list is the doctrine:

- **Group A — reads (direct):** dashboard, athlete profile, workout list +
  detail, prescription (next-up), training-max list + history, journal,
  athlete programs, athlete equipment.
- **Group B — clerical writes (direct):** create today's workout, log/edit/
  delete a set, update workout notes, log bodyweight, add a note. These are the
  human dictating fact, not a coaching decision. Each handler enforces
  `CanAccessAthlete` / `CanManageAthlete` in-handler, so bearer auth inherits
  the same ownership parity as the webui.
- **Group C — coaching changes (gated only):** enqueue a program generation and
  read its status. **There is deliberately no `execute` tool.** The succeeded
  draft is surfaced for the coach to approve on the webui, where the click is
  the approval (ADR 007 amendment / ADR 015).

**Omitted by design** (their absence is the safety property): the
`…/generations/{genID}/execute` route, the session-bound `Me` and impersonation
handlers, and every coaching-decision write — training-max writes, program
assignment/activation, tier promotion, cycle-review TM bumps, prescribed-set
edits. None of these is reachable via MCP.

A per-IP rate limiter guards `/api-mcp/*` as a distinct bucket from the webui
login limiter.

## Alternatives considered

- **RepLog runs its own Go MCP server + its own OAuth AS** (the original
  HOF-004 framing). Rejected: duplicates the AS that already runs on the same
  host and is already trusted by the Claude apps, and adds OAuth-AS code +
  dependencies to a binary whose value is its minimalism.
- **Per-user bearer-token map in `homelab-mcp` (sops).** Rejected: long-lived
  full-authority tokens that must be hand-synced with RepLog's `users` table; a
  misconfiguration silently grants one user another's access. JWT-as-identity
  obsoletes the map entirely.
- **`users.pocketid_sub` durable identity binding.** Deferred. Binding identity
  to `email` is fragile if a PocketID email is renamed, but at family scale the
  `email` UNIQUE + NOCASE + empty-claim-rejection rules cover it; doing `sub`
  correctly needs a PocketID backfill for existing accounts, which is its own
  micro-spec. Tracked as a follow-on.

## Security & privacy

- Asymmetric verification only — RepLog reads the public JWKS; the private key
  never leaves the AS. Full code-execution access to RepLog does not enable
  token minting.
- `aud`-scoping bounds blast radius: each resource server rejects tokens
  addressed elsewhere, independently.
- RS256 is pinned in two places (the keyfunc and the parser's valid-methods),
  closing alg-confusion.
- Default-deny gate plus per-IP rate limiting on the new internet-facing
  surface (ADR 014's per-account lockout still applies at the AS's password
  step, which is PocketID's concern now, not RepLog's).
- **The kids are naturally outside the MCP surface in v1**: they have no
  PocketID identity, no `email` populated, and `mcp_enabled` defaults to 0 — the
  empty-email rejection prevents them being addressable even if a token somehow
  carried a child's identity. This is intentional and should be preserved.

## Consequences

### Positive

- The Claude apps (including mobile) can act as a session companion — read
  state, log work, draft programs — with per-user identity and ownership
  enforced exactly as on the webui.
- ADR 001's single static binary is intact; no OAuth AS code, no Go MCP SDK.
- The no-automated-coaching line (ADR 007 / 015) is enforced structurally, by
  the absence of any commit/execute tool, not by a runtime check that could
  regress.

### Negative / trade-offs

- The surface spans three repos (`replog`, `homelab-mcp`, `nix-config`). The
  handoff/ACK review gate governs `replog` only; cross-repo changes are
  reviewed together by the host.
- A schema-mirroring tax: a RepLog REST change touching an MCP-exposed endpoint
  has a Python counterpart in `homelab-mcp` that can drift. Bounded at the v1
  tool count; see the graduation triggers.
- v1 does **not** expose an assembled-athlete-context read tool
  (`llm.BuildAthleteContext` is not a standalone handler); the shipped
  `replog_get_athlete` returns the basic profile. Exposing full context is a
  follow-on if it's wanted.

### Graduation to Option γ (RepLog-native Go MCP server, same AS)

When any of these fires, migrate to a RepLog-native Go MCP server that still
**trusts `homelab-mcp`'s AS** (RFC 9728 protected-resource metadata; the JWKS
verifier shipped here is reused, the auth boundary does not move):

1. RepLog opens to multi-family use (more than ~3–5 active coaches) — the
   cross-repo proxy config and contract drift start to bite.
2. Tool count crosses ~15 and the Python⇄Go schema-mirroring tax becomes
   visible.
3. A tool needs data or behavior the REST API doesn't expose, and adding a REST
   endpoint just for the tool is the wrong abstraction.

## References

- HOF-004 (basic-memory `replog` channel — spec, review, and ACK; archived to
  this ADR on ship)
- GitHub issue #22 — MCP agent surface for RepLog
- ADR 001 (single static binary — preserved), ADR 003 (scs session auth — the
  webui path, unchanged), ADR 005 (chi router — where `/api-mcp/*` mounts),
  ADR 007 + 015 (the human-in-the-loop principle encoded here as tool-catalog
  absence), ADR 002 (additive migration `0005`)
- RFC 6750 (bearer tokens), RFC 7591 (DCR), RFC 8414 (AS metadata), RFC 9728
  (protected-resource metadata / multi-resource pattern)
