# ADR 019 — Self-Hosted Identity + Native MCP Server (supersedes ADR 017)

> Status: **Proposed** (draft) · Date: 2026-06-01
>
> Supersedes **ADR 017** (resource-server-behind-homelab-mcp) outright, and
> supersedes the authentication half of **ADR 003** (RepLog-managed
> passwords/passkeys). Graduates to **Accepted** with the Phase-1
> implementation. Draft authored by Coach for host diff review.

## Context

ADR 017 made RepLog an OAuth *resource server* sitting behind `homelab-mcp`'s
OAuth AS: the tool catalog lived in Python (`replog.py` httpx wrappers), and
RepLog verified RS256 JWTs against the AS's JWKS. That was the right call at
the time — it reused a battle-tested AS and avoided a Go MCP SDK dependency.
Three things have since changed the calculus:

1. **The agent surface is growing.** The multi-modal logbook (ADR 018, shipped)
   added throwing/conditioning/skill/recovery logging + a load view + the Pitch
   Smart advisory. Wrapping these as MCP tools means more Python `replog.py`
   wrappers — each a second source of truth that can drift from its Go endpoint.
2. **Code volume is no longer the constraint.** With generated implementation,
   the "schema-mirroring tax" ADR 017 named is mostly *drift risk*, not typing
   effort — and a native server eliminates drift by removing the second source.
3. **There is now a proven, owned blueprint.** Operation W.W.W.
   (`/Users/ryan/src/whiskey-whiskey-whiskey`) ships a self-contained MCP OAuth
   AS (`server/routes/oauth.ts`, `docs/OAUTH_AS_PLAN.md`, HOF-023→027) that does
   DCR + RFC 9728/8414 metadata + a PKCE federation hop to PocketID + opaque
   tokens, working end-to-end against claude.ai mobile and VS Code. And there is
   now an **official, pure-Go MCP SDK** (`modelcontextprotocol/go-sdk`, no CGO),
   which answers ADR 017's "no Go MCP SDK dependency" objection.

Two design questions were resolved during review (2026-06-01):

- **Roll our own AS, no external gateway?** Yes. Research confirmed **PocketID
  has no Dynamic Client Registration** (clients are admin-created only), so it
  cannot be the MCP-facing AS directly. But W.W.W. proves an app can be its own
  DCR-capable AS *that federates login to PocketID* — so the homelab-mcp gateway
  is not required; RepLog can own the AS itself.
- **Whose identity?** PocketID. Identity and passkeys are the worst things to
  hand-roll per app, and the household already runs PocketID. RepLog stops
  managing its own passkeys/passwords and becomes a PocketID OIDC relying party.
  (Single user today → clean cutover, no migration.)

The load-bearing principle is unchanged: **RepLog is a logbook; a human makes
every coaching decision; the agent surface exposes no coaching-decision write
and no `execute`** (ADR 007 / 015). That boundary moves from "absent from a
curated route list" to "absent from the native tool registry" — same property,
asserted by the same kind of test.

## Decision

RepLog becomes **self-contained except for PocketID as the identity authority**:
its own MCP OAuth Authorization Server, its own native Go MCP server, and a
PocketID OIDC relying party for the webui. `homelab-mcp` leaves RepLog's path
entirely.

1. **Identity → PocketID (OIDC relying party).** The webui logs in via PocketID
   (Authorization Code + PKCE); RepLog retires `webauthn_credentials` and
   `password_hash` (ADR 003's auth half). PocketID's device-code flow (RFC 8628)
   and email one-time-access cover shared-device/kid login, so RepLog's
   `login_tokens` magic-link path also retires. Verification of PocketID's ID
   token uses `coreos/go-oidc` (pure Go) against PocketID's JWKS — the only
   JWKS RepLog still consumes.

2. **RepLog is its own MCP OAuth AS** (port of W.W.W.'s `oauth.ts`): DCR
   (`/oauth/register`), RFC 8414 AS metadata + RFC 9728 protected-resource
   metadata (both origin-root and path-suffixed variants), `/oauth/authorize`
   that generates an AS↔PocketID PKCE pair and 302s to PocketID,
   `/oauth/callback` that exchanges the code and JIT-upserts the user, and
   `/oauth/token` that mints the access token. Login is federated to PocketID;
   RepLog runs no consent UI of its own.

3. **Opaque, PAT-shaped MCP tokens** (W.W.W. Q1), e.g. `rlpat_…`, validated
   against RepLog's DB — **no JWT, no JWKS, no signing key** for the MCP path.
   This *retires* the ADR 017 RS256/JWKS bearer verifier (`bearer.go`) and
   reuses the existing `login_tokens`-style opaque-token mechanics. Fixed TTL
   (90 days, W.W.W. Q2), no refresh, soft-delete revocation, `mcp_enabled` gate
   preserved.

4. **Native Go MCP server** using `modelcontextprotocol/go-sdk` over
   StreamableHTTP at `/api/mcp`. The tool catalog is a Go registry
   (`buildMCPServer(user)`), the single source of truth — the `replog.py`
   wrappers retire and the Python⇄Go mirror disappears. Tools map to existing
   handlers/models; identity/role flows in for scoping.

5. **The doctrine boundary is preserved in the registry.** Exposed: Group A
   reads, Group B clerical writes — **now including the four multi-modal
   discipline logs** + the load/Pitch-Smart reads — and Group C draft-enqueue.
   **Not registered** (the safety property): any `execute`, training-max writes,
   program assignment/activation, tier promotion, cycle-review bumps,
   prescribed-set edits, and **season-phase create/delete** (the coaching-gated
   write from HOF-011). A registry-assertion test replaces ADR 017's
   `chi.Walk` mount-list test.

6. **Kid-safety is re-derived as explicit gating.** ADR 017 leaned on kids being
   structurally unaddressable (no PocketID identity). Onboarding kids to PocketID
   makes them addressable, so the wall becomes `mcp_enabled` default-0 + the
   coaching-write boundary above. PocketID has no minor/parental features; any
   child-account policy lives in RepLog (groups/claims + `mcp_enabled`).

`homelab-mcp` is removed from RepLog's path: no proxy wrappers, no trusted
external AS. The only service RepLog talks to is PocketID, which is the
identity authority by design.

### Adopted W.W.W. decisions (the de-risked Q-list)

These were settled empirically in W.W.W. (HOF-023→027) and are adopted verbatim:

- **Opaque tokens, prefix-gated** (Q1): hard-reject any bearer not starting with
  the prefix before any DB hit — structurally prevents bearer-format collisions.
- **90-day TTL, no refresh** (Q2): re-auth on expiry is a passkey tap.
- **DCR redirect-URI allowlist, filter-don't-reject** (Q3): store only
  allowlisted URIs, succeed if ≥1 matches; per-client validation at
  `/oauth/authorize` is the authoritative gate. Loopback prefixes require a
  trailing `/` or `:` (RFC 8252 §7.3). Allowlist must include claude.ai,
  claude.com, `127.0.0.1`/`localhost` (port and port-less), and the VS Code
  redirect URIs.
- **In-memory state store, 5-min TTL, single-use** (Q5) for the authorize hop.
- **Per-IP rate cap on `/oauth/register`** (Q7) — the only public write endpoint.
- **`jwks_uri` omitted** from AS metadata (Q9) — opaque tokens, and claude.ai
  tolerates its absence (empirically verified in W.W.W.).
- **Two PRM variants** — origin-root and path-suffixed
  (`/.well-known/oauth-protected-resource/api/mcp`) with `resource` exactly
  equal to the MCP URL (RFC 9728 §3.3; VS Code rejects mismatches), advertised
  via `WWW-Authenticate` on 401.
- **Gotchas:** urlencoded body parser on `/oauth/token` (not JSON); constant-time
  client-secret compare; never forward a PocketID error to a redirect_uri
  without a valid state lookup; never log tokens.

### Schema changes (additive migration, ADR 002)

- `users.pocketid_sub TEXT UNIQUE` — the stable PocketID subject, set on first
  OIDC login (resolves ADR 017's deferred `pocketid_sub` follow-on; replaces
  email as the identity key for login, email retained for display/notify).
- An opaque MCP-token store (extend the `login_tokens` pattern or a dedicated
  `mcp_tokens` table: token hash, user_id, oauth_client_id, label, expires_at,
  revoked_at, last_used_at). Settled in the Phase-2 HOF.
- A DCR client store (client_id, client_secret_hash, client_name,
  redirect_uris, created_at).
- **Retire** `webauthn_credentials` and `users.password_hash` (and the
  `login_tokens` magic-link path) once the OIDC RP is live — single user, so a
  clean cutover rather than a dual-auth transition.

## Implementation plan (phased — each phase is a HOF through the review gate)

Rolled together as one ADR / one coordinated arc (host request), but sequenced
into reviewable phases like ADR 016:

- **Phase 1 — `HOF-012`: PocketID OIDC relying party (webui).** `coreos/go-oidc`
  + `golang.org/x/oauth2`; login flow → scs session; `users.pocketid_sub`
  JIT-upsert; retire native password/passkey/magic-link login. Clean cutover.
- **Phase 2 — `HOF-013`: the MCP OAuth AS.** Port W.W.W.'s `oauth.ts` to Go —
  DCR, the two metadata docs, `/oauth/authorize`→PocketID PKCE hop,
  `/oauth/callback`, `/oauth/token`, the opaque-token store, the in-memory state
  store, and the adopted gotchas. Retire the `bearer.go` JWKS verifier.
- **Phase 3 — `HOF-014`: native Go MCP server + tools.** `go-sdk` StreamableHTTP
  server at `/api/mcp`; the `buildMCPServer(user)` registry (Group A/B/C incl.
  the multi-modal logs, coaching writes excluded); the registry-assertion test;
  retire `homelab-mcp`'s `replog.py` wrappers (cross-repo, host-reviewed).

## Consequences

### Positive
- Self-contained but for PocketID — no homelab-mcp dependency, no Python⇄Go mirror.
- One source of truth for tools (Go registry) and for identity (PocketID).
- Token path *simplifies*: opaque DB-validated tokens replace the RS256/JWKS verifier.
- ADR 001 preserved — `go-sdk`, `go-oidc`, `oauth2` are all pure Go, no CGO.
- The no-automated-coaching line stays structural (absent from the registry).

### Negative / cost
- RepLog now owns AS security (DCR, PKCE, token lifecycle, the redirect-URI
  allowlist) — mitigated by porting W.W.W.'s de-risked, production-proven design
  rather than green-fielding it.
- Hard dependency on PocketID for first-auth (sessions persist; device-code is
  the fallback). It's self-hosted infra, not SaaS.
- Cross-repo cleanup (retire `replog.py`); reviewed by the host.

### Neutral
- Phased; each phase ships independently. If only Phase 1 lands, the webui is on
  PocketID and the MCP path still works via the old verifier until Phase 2/3.

## Alternatives considered
1. **Keep homelab-mcp as a thin DCR-AS** (the prior recommendation). Rejected:
   W.W.W. proves RepLog can own the DCR+federation itself; keeping the gateway
   retains an external dependency for no benefit once the AS is owned.
2. **Roll our own identity too (W.W.W.-style, no PocketID).** Rejected: identity
   is the thing worth centralizing on the purpose-built product; PocketID is
   already the household IdP.
3. **JWT/JWKS MCP tokens.** Rejected per W.W.W. Q1 — a single-process AS+RS moots
   JWT statelessness and opaque tokens give honest per-token revocation.
4. **Status quo (ADR 017).** Rejected per the Context above.

## References
- ADR 017 (superseded — resource server / homelab-mcp), ADR 003 (auth half
  superseded), ADR 001 (single static binary — preserved), ADR 007 / 015
  (no-automated-coaching — preserved structurally), ADR 018 (the surface being
  exposed), ADR 002 (additive migration).
- Operation W.W.W.: `server/routes/oauth.ts`, `server/lib/auth.ts`,
  `docs/OAUTH_AS_PLAN.md`, basic-memory `whiskey-whiskey-whiskey` HOF-023→027.
- `modelcontextprotocol/go-sdk`; `coreos/go-oidc`; `golang.org/x/oauth2`.
- PocketID (pocket-id/pocket-id): OIDC + device-code (RFC 8628) + email
  one-time-access; **no DCR**, no minor controls.
- RFC 6749/6750/7591/8252/8414/8628/9728; MCP authorization spec.
