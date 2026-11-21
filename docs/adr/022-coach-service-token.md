# ADR 022 — Coach Service Token

> Status: **Accepted** · Date: 2026-11-21

## Context

The homelab connector calls RepLog's REST API as Ryan's coach: it reads the
coach dashboard and logs a set the person dictated. That caller is not a
browser. A long-lived bearer has to identify the coach account, and it must
not become a back door that assigns a program, executes a generation, writes
a training max, or promotes an athlete. Those are coaching decisions. The
human approves them in the browser (ADR 007).

Login tokens and MCP tokens are stored as SHA-256 hashes (ADR 019,
ADR 021). The service credential follows that pattern. It is not an MCP
token. `rlpat_` was the prefix for the removed `/api/mcp` bearer; that
path is not served. `mcp_enabled` does not gate this path. `RequireAuth`
accepts only an `rlcst_` bearer.

## Decision

- Table `service_tokens` (migration `0017`) stores `token_hash`, the
  lowercase-hex SHA-256 of the full presented secret. The plaintext is never
  inserted and never logged.
- The presented secret starts with `rlcst_`. `RequireAuth` rejects any other
  bearer before a query, including an `rlpat_` MCP token. A valid service
  token loads the owning user under the same context keys as a session and
  sets a service-credential flag. It does not write `userID` into the session,
  so the token cannot mint a browser session.
- When `REPLOG_SERVICE_TOKEN` is set, startup hashes it onto the coach named
  by `REPLOG_SERVICE_USER`, or `REPLOG_ADMIN_USER` when that is unset. The
  account must be a coach or an admin. Replacing the env value replaces the
  previous `coach-service` row.
- These handlers return 403 when the flag is set, before they mutate:
  program assign (`POST /athletes/{id}/programs`), generation execute,
  training-max writes (`POST /training-maxes`, `POST /batch-tms`,
  `POST /cycle-review`), and promote. The same routes accept a browser
  session of that user. Object-level authorization (ADR 021) is unchanged
  on every route the token is allowed to call.

## Consequences

- A leaked database does not yield a usable service token. The usable value
  lives only in the process environment and in the connector's secret store.
- The connector can log work. It cannot approve a draft, change a training
  max, assign a program, or promote.
- No new HTTP route. OpenAPI is unchanged.

## Amendment 2026-11-21 — Program drafts (ADR 023)

`POST /programs/drafts` is the write a service token is allowed to make.
The row is a proposal with provenance. `POST /programs/{id}/approve` and
`POST /programs` (which inserts an approved template) return 403 for the
token. A browser session of the same coach can approve. Approval does
not assign the program.
