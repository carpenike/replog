# Copilot instructions

You are working on **RepLog** — a self-hosted web app for tracking
resistance-training workouts for a single family (kids on tier-based
progression, adults on percentage programs). Go single-binary backend
(JSON REST API) + embedded React SPA, SQLite storage, deployed on NixOS.

**Key principle:** the app is a logbook. A human coach makes all
progression decisions — the app never automates coaching. The LLM features
draft proposals a coach reviews and approves (ADR 007).

## Read first

The primary instructions live in **[`AGENTS.md`](../AGENTS.md)** at the repo
root. **Read it before doing anything** — it carries the full orientation,
stack, repo layout, architecture rules, coding standards, build/verify
steps, and session-completion discipline.

Deep-dive docs you'll likely need:

- **[`docs/data-model.md`](../docs/data-model.md)** — the schema source of
  truth (27 tables, DDL, ERD, triggers). Read before any migration or model
  change.
- **[`docs/adr/`](../docs/adr/)** — 14 architecture decision records. Read
  the relevant ADR before changing the area it governs.
- **[`docs/COACH_VOICE.md`](../docs/COACH_VOICE.md)** — voice + behavior for
  user-visible copy and LLM prompts, and the no-automated-coaching line.

Path-scoped rules under **[`.github/instructions/`](instructions/)**
(`go.instructions.md`, `sql.instructions.md`) fire automatically for
matching files — they still apply on top of this file.

## Cross-agent comms (basic-memory)

This repo has a shared **basic-memory** MCP project for async hand-offs
between you (Copilot) and Coach (Claude Desktop / Cowork). Both clients hit
the same local basic-memory server and read/write the same notes.

- **Project name:** `replog` — address basic-memory by this name.
- **Project ID is client-local.** Run `list_memory_projects` to get *your*
  client's `external_id` for the `replog` folder and pass it as `project_id`
  — do not reuse another client's UUID (each client mints its own). The
  shared source of truth is the folder `/Users/ryan/basic-memory-replog`. The
  default `main` project is unrelated — never write RepLog state there.
- **At session start**, call `recent_activity` against this project (7-day
  window) to see what Coach left for you, then read `handoff/README` in the
  same project — it codifies the three-surface model (GitHub Issues =
  backlog, basic-memory = handoffs, docs/ADRs = durable), the directory
  layout, the observation/relation vocabulary, the review lifecycle, and the
  write protocol. Don't post your first note without reading it.
- **You are expected to challenge Coach's spec.** Every spec handoff carries
  a `[review-mandate]`. Read the linked draft doc/ADR AND the actual source
  files referenced. If you find drift between spec and code, post
  `[challenge]` / `[finding]` / `[proposed-revision]` observations to a
  paired `handoff/HOF-NNN DISCUSSION` note. You may edit the draft doc/ADR
  directly to propose revisions — your edits show up in the VS Code
  working-tree diff for host review.
- **Pause after review.** Even if review is clean, post `[review-clean]`,
  flip the spec `[status]` to `review-complete`, and stop. Do NOT proceed to
  implementation until Coach flips `[status]` to `approved`. Human gate
  every time.

See also the **Cross-agent comms** section of [`AGENTS.md`](../AGENTS.md).

## Quick reminders specific to this project

- **No automated coaching.** Anything the LLM produces is a *proposal* a
  coach approves — never an instruction or an automatic progression. See
  `docs/COACH_VOICE.md`.
- **No ORM.** SQL lives in `internal/models/`; use `?` placeholders.
- **Pure-Go SQLite only** (`modernc.org/sqlite`) — never `mattn/go-sqlite3`
  (CGO breaks the static binary). Always `SetMaxOpenConns(1)` and set the
  WAL/busy_timeout/foreign_keys PRAGMAs on open.
- **No full web framework** — `chi` is the only router (ADR 005).
- **Handlers stay thin and JSON-only** — validate → model → `api.WriteJSON` /
  `api.WriteError`. Never write raw HTML in Go.
- **Regenerate the OpenAPI spec** with `just openapi` after any route/DTO
  change and commit it — CI fails on a stale spec (ADR 013).
- **Frontend:** TanStack Query for server data, the typed `api` client (no
  raw `fetch` in components), shadcn/ui + Tailwind.
- **Verify before declaring done:** `just qa` (openapi-check + lint + test +
  build).

## VS Code workflow

`.vscode/tasks.json` provides the dev tasks:

- **Dev (server + frontend)** — runs both processes together (preferred;
  equivalent to `just dev`). Open <http://localhost:5173>, login admin/admin.
- **Run Server** / **Vite Dev Server** — run them individually.
- **Quality Gates (build + vet + test)** — full backend QA.
- **Build**, **Test All**, **Test Current Package**, **Vet**.

Prefer the tasks (or `just` recipes) over ad-hoc terminal commands — they
preserve the dev environment.

## Where to start with any change

1. Read `AGENTS.md`.
2. Read the relevant ADR + the schema in `docs/data-model.md`.
3. Open the nearest existing neighbor of what you're adding — copy the pattern.
4. Make the change.
5. Run `just qa` (and `just openapi` if you touched a route/DTO).
6. Complete the session: file/close issues, push (see AGENTS.md → Session
   Completion). Be brief in your summary — the diff says it.
