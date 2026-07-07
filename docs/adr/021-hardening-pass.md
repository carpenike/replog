# ADR 021 — Comprehensive Hardening Pass

> Status: **Accepted** · Date: 2026-07-07
>
> Batches several cross-cutting decisions that came out of a full-application
> review (security, AI pipeline, data integrity, observability). Each is small
> on its own; grouping them keeps the rationale discoverable without one ADR per
> one-line fix. Where a decision refines an earlier ADR, it is noted inline.

## Context

A comprehensive review of RepLog surfaced one systemic security bug plus a set
of correctness, AI-safety, and operability gaps. This ADR records the decisions
that have lasting architectural weight; purely mechanical fixes (typos, error
strings, pagination clamps) are not enumerated here.

## Decisions

### 1. Object-level authorization is scoped in the model layer

Several older CRUD handlers authorized the athlete `{id}` in the URL path via
`CanAccessAthlete`, then operated on a child resource id (`workoutID`, `setID`,
`noteID`, `bwID`) looked up **globally** — a classic IDOR. Because an
athlete-linked user may access their own athlete id, any such user could read,
edit, or delete another athlete's workouts, sets, notes, and body weights by
pairing their own athlete id with someone else's child id.

**Decision:** ownership is enforced in the model layer, not just the handler.
Child mutations scope by athlete in SQL (`... WHERE id = ? AND athlete_id = ?`;
for sets, join through `workout_id → workouts.athlete_id`) and return
`ErrNotFound` on mismatch, which the handler maps to 404. This mirrors the
pattern `loadOwnedGeneration` (ADR 015) and the ADR-018 handlers already used,
and a route-level regression test (`TestCrossAthleteIDOR`) guards it. Refines
the authorization model of ADR 003.

### 2. Magic-link login tokens are hashed at rest

`login_tokens.token` was stored in plaintext and the usable link was also copied
into a persisted notification row — anyone with read access to the SQLite file
obtained working magic links. **Decision:** store `sha256(token)` and look up by
hash (same treatment MCP tokens already got in ADR 019); never persist the full
usable link in a notification. Refines ADR 014.

### 3. Privileged actions are audited to a durable table

Impersonation (ADR 003) was logged only to stdout. **Decision:** an `audit_log`
table records privileged actions (impersonation start/stop) with real-user,
target-user, action, and timestamp, so the trail survives a log-rotation.

### 4. Check-then-act invariants are enforced structurally

Two races were closed with database-level guarantees rather than app-level
pre-checks: a partial unique index (`idx_generations_inflight`) makes
"one in-flight generation per (athlete, kind)" structural, and
`MarkGenerationExecuted` is a claiming `UPDATE ... WHERE executed_at IS NULL`
run **before** the import, so a double-click cannot import twice. Refines
ADR 015.

### 5. AI output is linted, and prompt overrides cannot strip safety

The generation pipeline (ADR 007/015) trusted the model to only use catalog
exercises and to respect youth loading rules; nothing enforced it, so an
invented or incompatible exercise name was silently dropped at import.

**Decision:** a deterministic post-generation lint (`LintCatalog`) checks every
prescribed exercise against the catalog the model was given (case-insensitive),
flags equipment-incompatible exercises, and flags percentage loading for
foundational/intermediate youth. Results persist on the generation row and
surface in the coach's preview as advisory warnings (never blocking — the coach
is the backstop). Separately, the admin system-prompt override is now
**compositional, not substitutional**: it is appended after the built prompt so
a global override can never remove the youth NSCA safety block or the
CatalogJSON schema from a minor's generation. Untrusted athlete-authored text is
wrapped in `<athlete_context>` delimiters with an explicit trust-boundary rule.

### 6. Provider calls are robust; JSON extraction is string-aware

LLM JSON extraction now uses a string-aware decoder (a brace inside a note
string no longer truncates valid output), with one bounded parse-repair retry.
Providers retry on 429/5xx with backoff; the OpenAI client uses
`max_completion_tokens` and omits temperature for reasoning models; the Ollama
client sets `num_ctx`/`num_predict` and reports real stop reasons and token
counts so truncation hints can fire.

### 7. Observability is opt-in and dependency-light

Structured logging (`log/slog`, JSON when `REPLOG_LOG_FORMAT=json`) covers
startup/shutdown and request logging; a hand-rolled `/metrics` endpoint
(`REPLOG_METRICS_ENABLED=true`, Prometheus text format, no new dependency)
exposes request counters and Go runtime gauges; a `replog healthcheck`
subcommand backs a container `HEALTHCHECK`. A startup `PRAGMA quick_check`
warns on a corrupt database.

### 8. Retired WebAuthn configuration is removed

Passkeys were retired in ADR 019 but the Nix module/lib/tests and the operations
runbook still derived and documented `REPLOG_WEBAUTHN_*`. That dead configuration
is removed; the runbook documents the PocketID OIDC path that is actually used.

## Consequences

- The IDOR fix is the load-bearing change; it closes cross-athlete
  read/write/delete on the app's core tables.
- New migrations `0012`–`0014` (in-flight unique index, `audit_log`,
  `generations.warnings`/`prompt_version`).
- New env vars: `REPLOG_LOG_FORMAT`, `REPLOG_METRICS_ENABLED` (both optional,
  default off/text).
- The MCP surface gains a `list_exercises` read tool and richer tool schemas;
  the "no automated coaching" doctrine (ADR 007/015) is unchanged.
- No user-facing behavior changes for the coach beyond the new preview warnings
  and a correctly-working per-athlete data export (ADR 006), which had a dead UI
  with no backend before this pass.
