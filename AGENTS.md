# RepLog — Agent Instructions

> **You are working on RepLog — a self-hosted web app for tracking
> resistance-training workouts.** It serves a single family: kids on a
> tier-based progression system and adults running percentage-based
> programs (5/3/1, GZCL). Go single-binary backend (JSON REST API) +
> React SPA, deployed as one static binary on NixOS.
>
> **Key principle: the app is a logbook. A human coach makes all
> progression decisions — the app never automates coaching.** The
> LLM features are a research assistant that *drafts* proposals a coach
> reviews and approves (ADR 007). Hold this line in code and in copy.

This file is the **main entry point** for any AI coding agent (Claude /
Coach, GitHub Copilot, or otherwise) joining this project. Read it first,
then go to the linked deep-dive docs as needed. Copilot also has
path-scoped rules under [`.github/instructions/`](.github/instructions/)
(`go.instructions.md`, `sql.instructions.md`) that fire automatically for
matching files — honor them.

## Quick orientation

- **What it is:** Go (1.25+) single static binary serving a JSON REST API
  and an embedded React SPA from one process. SQLite (WAL) for storage.
- **Backend stack:** `chi` router (group-based middleware) · `modernc.org/sqlite`
  (pure-Go driver, **no CGO**) · `pressly/goose` migrations embedded via
  `embed.FS`, auto-run on startup · `alexedwards/scs` session management ·
  `coreos/go-oidc` + `golang.org/x/oauth2` PocketID OIDC relying party (ADR
  019) · `containrrr/shoutrrr` external notifications · `bcrypt` password
  hashing (break-glass). No ORM — SQL lives in the models layer.
- **MCP-AS contract:** the embedded MCP OAuth Authorization Server (ADR 019)
  conforms to [`pocketid-mcp-as`](https://github.com/carpenike/mcp-as-contract)
  **v1.1**, profile `opaque-no-refresh`, scope `mcp-only`, MCP path `/api/mcp`.
  CI (`.github/workflows/conformance.yml`) boots the binary and runs the
  contract's own harness; the pin (`CONTRACT_REF`) is tracked by Renovate.
- **Frontend stack:** React 19 + TypeScript + Vite · shadcn/ui (components
  copied into `web/src/components/ui/`) · Tailwind CSS v4 · TanStack Query
  for server state · React Router v7 · lucide-react icons.
- **Deployment:** Nix flake builds the Vite frontend, then `go build`s the
  binary with `web/dist` embedded. Single binary runs as a systemd service
  on NixOS behind a reverse proxy. Module path `github.com/carpenike/replog`.
- **State of the project:** Working and in active use. **Don't break what
  works.** Read existing code patterns before adding features.

## Cross-agent comms (basic-memory)

A shared **basic-memory** MCP project is the async hand-off channel
between AI agents working on this repo — currently **Coach** (Claude
Desktop / Cowork) and **GitHub Copilot** (VS Code). Both clients connect
to the same local basic-memory server and read/write the same markdown
notes on disk.

- **Project name:** `replog` — address basic-memory by this name; it is the
  stable identifier shared across clients.
- **Project ID is client-local.** Each basic-memory client mints its own
  `external_id` (UUID) for the `replog` folder — there is no universal one.
  Run `list_memory_projects` to get *your* client's id and pass it as
  `project_id` for unambiguous routing; do NOT copy another client's UUID.
  (For reference: Cowork's is `d93e6b10-…`, VS Code Copilot's is `d2b38e13-…`;
  yours may differ — verify it.)
- **Local path / shared source of truth:** `/Users/ryan/basic-memory-replog`
  (not in this repo) — every client reads/writes these same files.
- **Default project (`main`) is unrelated** — never write RepLog state there.

At session start, call `recent_activity` against this project with a 7-day
window to see what the other agent left behind. Then read
`handoff/README` in the same project — it codifies the full convention
(three-surface model, directory layout, observation/relation vocabulary,
the review lifecycle, the graduation rule, and the write protocol). Don't
write your first note without reading it.

**Three surfaces, three jobs.** RepLog tracks work in three places —
keep them in their lanes:

| GitHub Issues (`gh`)             | basic-memory (`replog`)            | `docs/` in the repo (git)        |
| -------------------------------- | ---------------------------------- | -------------------------------- |
| The backlog — *what* work exists | Review-first specs in flight       | Durable architecture (ADRs)      |
| Bug reports, feature requests    | Discussion logs, review findings   | `data-model.md` (schema truth)   |
| Closed when work ships           | Open questions, decisions pending  | `requirements.md`, `ui-design.md`|

GitHub Issues stays the source of truth for *what* is tracked. basic-memory
is where a spec gets written, challenged, and approved before code lands. A
handoff `implements` an issue and `graduates` to an ADR when it ships.

**Review is mandatory by default.** Every spec handoff goes through a
review phase before any code lands. Coach drafts the spec and plants a
`handoff/HOF-NNN` note at `[status] needs-review` with a `[review-mandate]`.
Copilot is expected to *challenge* the spec against the actual code, edit
the draft doc/ADR directly when proposing revisions (visible in the VS Code
working-tree diff), and post findings to a paired `handoff/HOF-NNN
DISCUSSION` note. Coach then either accepts the revisions or pushes back via
`[responds-to]` observations. Implementation only begins after `[status]`
flips to `approved`. Clean reviews still pause for the host verdict — human
gate every time. Opt out via `[skip-review] true` for trivial work only
(rename, typo, doc-only).

## Repository layout

```
replog/
├── AGENTS.md                  ← you are here
├── README.md                  ← human-facing project + dev setup
├── Justfile                   ← dev commands (just <recipe>)
├── flake.nix / flake.lock     ← Nix build (frontend + backend)
├── .github/
│   ├── copilot-instructions.md  ← Copilot pointer to this file
│   ├── instructions/            ← path-scoped Copilot rules (go, sql)
│   └── workflows/               ← CI (ci.yaml) + release (release.yaml)
├── cmd/replog/
│   └── main.go                ← entrypoint: DB init, migrations, router, serve
├── internal/                  ← all app code (not importable outside module)
│   ├── api/                   ← JSON REST handlers (the only HTTP layer)
│   │   └── openapi/swagger.yaml  ← GENERATED by swag; don't hand-edit
│   ├── database/
│   │   ├── migrations/*.sql   ← goose migrations, embedded via embed.FS
│   │   ├── migrate.go         ← RunMigrations() on startup
│   │   ├── db.go              ← Open DB, set PRAGMAs, return *sql.DB
│   │   └── seed.go            ← exercise-catalog seeding from JSON
│   ├── importers/             ← Hevy / Strong / RepLog JSON / catalog parsers
│   ├── llm/                   ← LLM providers (Anthropic, OpenAI, Ollama)
│   ├── middleware/            ← auth, CORS, logging, rate limit, security headers
│   ├── models/                ← data access layer (hand-written SQL, not ORM)
│   ├── notify/                ← in-app + external (shoutrrr) notifications
│   ├── oidc/                  ← PocketID OIDC relying-party endpoints (ADR 019)
│   └── scheduler/             ← background maintenance (token cleanup, etc.)
├── web/                       ← React SPA (Vite)
│   ├── embed.go               ← //go:embed all:dist
│   └── src/{api,components,pages}/
├── avatars/                   ← runtime avatar storage (not embedded)
└── docs/
    ├── data-model.md          ← schema source of truth (27 tables, DDL, ERD)
    ├── requirements.md        ← v1.0 user stories + acceptance criteria
    ├── ui-design.md           ← design system + component patterns
    ├── operations.md          ← deploy, backups, secret-key handling, upgrades
    ├── seed-catalog.md        ← exercise seed-data format
    ├── COACH_VOICE.md         ← how the Cowork/Claude agent speaks + behaves
    └── adr/                   ← 14 architecture decision records
```

## Architecture decisions (read before changing these areas)

The ADR index is [`docs/adr/README.md`](docs/adr/README.md). The
load-bearing ones:

- **ADR 002** — Goose migrations, embedded via `embed.FS`, auto-run on
  startup. Pre-prod, `0001_initial_schema.sql` is mutated in place (see
  `just db-reset`); once shipped, additive migrations only.
- **ADR 003** — `bcrypt` + `scs` sessions; env-var bootstrap of the first
  admin; coach vs non-coach access.
- **ADR 005** — `chi` router for group-based middleware. No full web
  framework (no gin/echo/fiber) — chi is the only router dependency.
- **ADR 007** — LLM as a *research assistant* that drafts program
  proposals a coach reviews. The app never automates coaching.
- **ADR 011** — REST API + React SPA is the current frontend architecture
  (supersedes the original htmx plan in ADR 001 and the Pico CSS of ADR 004).
- **ADR 012** — shared API test harness (`internal/api/handlers_test.go`).
- **ADR 013** — OpenAPI spec generated from `swaggo/swag` annotations; CI
  fails on a stale spec.
- **ADR 014** — per-account login lockout (5 fails / 15 min, sliding window).

## Database schema

27 tables; full DDL, constraints, indexes, and triggers live in
[`docs/data-model.md`](docs/data-model.md) — **that file is the source of
truth**, not this one. Key patterns:

- One workout per athlete per day (`UNIQUE(athlete_id, date)`).
- One set = one row in `workout_sets` (per-set logging, not per-exercise
  aggregates).
- Active assignments use a partial unique index (`WHERE active = 1`).
- Training-max history: multiple rows per athlete+exercise; current = latest
  `effective_date`.
- `updated_at` triggers guard with `WHEN OLD.updated_at = NEW.updated_at` to
  prevent recursion.
- FK `ON DELETE`: CASCADE from athletes, RESTRICT from exercises (protect
  logged history), SET NULL for `users.athlete_id` and
  `workout_reviews.coach_id`.
- Coach ownership: `athletes.coach_id` → `users(id)` — coaches only
  see/manage their assigned athletes.

### SQLite rules

- Always `db.SetMaxOpenConns(1)` — SQLite is single-writer.
- Always set PRAGMAs on open: `journal_mode=WAL`, `busy_timeout=5000`,
  `foreign_keys=ON`.
- Use `modernc.org/sqlite` (pure Go). **Never** import `mattn/go-sqlite3`
  (requires CGO; breaks the static-binary build).
- Backups: `sqlite3 replog.db ".backup backup.db"` (or `just db-backup`) —
  never `cp` a live WAL-mode DB.
- Use `?` placeholders for query parameters, never `fmt.Sprintf`.

## Coding standards

### Backend (Go)

- Stdlib `net/http` patterns — `http.HandlerFunc` / `http.Handler`.
- `chi` for routing with group-based middleware. No full web framework.
- Error handling: wrap with `fmt.Errorf("context: %w", err)`, return up.
- No ORM — write SQL directly in the models layer.
- Keep handlers thin: validate input → call model → return JSON via
  `api.WriteJSON` / `api.WriteError`. Never write raw HTML strings in Go.
- `internal/` packages are not importable outside the module — use it for
  all app code.

### Frontend (React)

- TanStack Query for all server data — don't `useEffect` + `fetch` directly.
- Use the typed `api` client in `web/src/api/` — don't hand-build `fetch()`
  calls in components.
- Components in `web/src/components/`; route components in `web/src/pages/`.
- Use shadcn/ui from `web/src/components/ui/` rather than rebuilding
  primitives. Tailwind utility classes; avoid inline styles.
- Never declare component functions inside other components — hoist to
  module scope. Follow React 19 rules (no `setState` in `useEffect` for
  derivable values; initialize state via `useState(initializer)`).

### API contract

- The OpenAPI spec at `internal/api/openapi/swagger.yaml` is **generated**
  by `swaggo/swag` from handler annotations. Run `just openapi` after
  adding/changing a route or DTO and commit the result — **CI fails on a
  stale spec** (`just openapi-check`). Copy an annotation block from a
  similar handler (Login, Me, Dashboard, ListAthletes are good templates).
- Auth uses session cookies (`SameSite=Lax`) — no JWT, no Authorization
  header. Same-origin in production (SPA served from the Go binary); CORS
  enabled in dev for the Vite dev server.

## Auth & access control

- Three tiers: admin (`is_admin = 1`), coach (`is_coach = 1`), athlete
  (non-coach). Roles overlap (an admin can also coach).
- Admins manage everything. Coaches manage only athletes assigned via
  `athletes.coach_id`. Non-coach users link to one athlete via
  `users.athlete_id` and only see their own. Unlinked non-coach users get
  an informative message, not a blank screen.
- First-run bootstrap creates admin+coach from `REPLOG_ADMIN_USER` /
  `REPLOG_ADMIN_PASS` / `REPLOG_ADMIN_EMAIL`. Session lifetime 30 days,
  `HttpOnly`, `SameSite=Lax`.

## Build / verify before declaring victory

The repo ships a Nix devShell + a `Justfile`. Run `just` to list recipes.

```bash
just dev            # backend (:8080) + Vite (:5173); login admin/admin
just lint           # go vet + golangci-lint + web npm run lint
just test           # go test -count=1 ./...
just build          # frontend bundle + Go binary with embedded SPA
just qa             # openapi-check + lint + test + build  (matches CI)
just openapi        # regenerate the OpenAPI spec after route/DTO changes
just db-reset       # wipe ./dev.db; next run re-bootstraps + reseeds
```

**Required for any change to merge:** `just qa` green (which includes the
OpenAPI staleness check). If you touched a route or DTO, run `just openapi`
and commit the regenerated spec. Without `just`, the equivalents are
`go vet ./...`, `go test -count=1 ./...`, `go build ./cmd/replog`, and
`swag init` per the `openapi` recipe.

## Issue tracking

GitHub Issues is the source of truth for tracked work. Use the `gh` CLI:

```bash
gh issue list                                    # browse open issues
gh issue view <number>                           # view details
gh issue create --title "..." --body "..."       # file new work
gh issue edit <number> --add-label "in-progress" # claim work
gh issue close <number> --comment "..."          # complete work
```

A basic-memory handoff should cite the issue it implements (`[implements]
#NN`); the issue closes when the implementation ships.

## Session completion

When ending a work session, complete ALL steps. Work is NOT complete until
`git push` succeeds.

1. File GitHub issues for any remaining work.
2. Run quality gates if code changed — `just qa` (or `go test ./...`,
   `go vet ./...`, `go build ./cmd/replog`).
3. Update issue status — close finished work, comment on in-progress items.
4. Push to remote: `git pull --rebase` then `git push`.
5. Verify `git status` shows "up to date with origin".

**Handoff work — Coach confirms the ACK before you commit (no branch/PR at
this phase):** when implementing an approved `HOF-NNN`, run `just qa` to
green, then post the `HOF-NNN ACK` describing the change, the qa results, and
the commit you intend to make — and **hold**. Do NOT commit or push yet.
Coach reviews the diff and confirms the ACK in the channel; only after that
confirmation do you commit and push to `main`. The change sits uncommitted in
the working tree until then — that is expected, not stranded work.

**Critical rules:** for non-handoff changes, never stop before pushing (it
strands work locally) — YOU push; if push fails, resolve and retry until it
succeeds. For handoff-tracked work, the Coach-confirms-the-ACK gate above
takes precedence over "push immediately": hold the commit until confirmed,
then push promptly (don't leave confirmed work stranded).

> **Note for Coach (Cowork/Claude):** the host's standing rule is
> review-first and **don't auto-commit** — Coach authors specs and doctrine,
> validates, then stops for the host to review the diff. The push-it-yourself
> discipline above is Copilot's implementation-session rule, not a license
> for Coach to commit spec drafts. See `docs/COACH_VOICE.md`.

## Voice

End-user-facing copy and the Cowork/Claude agent's chat posture follow a
light strength-coach framing — direct, professional, domain-aware, never a
heavy costume, and never making the coaching decisions the human coach owns.
See [`docs/COACH_VOICE.md`](docs/COACH_VOICE.md) before writing user-visible
strings, LLM system prompts, or planning prose.

## Reading code before speccing

Coach has live filesystem access to the repo. Before issuing specs or
architectural recommendations:

1. **Read the actual schema** — [`docs/data-model.md`](docs/data-model.md)
   is the source of truth; confirm column names before speccing migrations.
2. **Read the relevant ADR** in [`docs/adr/`](docs/adr/) — don't re-decide
   something already decided, and amend the ADR if you're changing it.
3. **Read the actual handlers/models** (`internal/api/handlers.go`,
   `internal/models/`) before speccing where to insert something.
4. **Check `gh issue list`** for live tracked-work state; check the
   basic-memory channel for in-flight handoffs. Don't assume from memory.

## Resources

- [`docs/data-model.md`](docs/data-model.md) — schema, ERD, DDL, seed data,
  operational notes
- [`docs/requirements.md`](docs/requirements.md) — user stories + acceptance
  criteria
- [`docs/ui-design.md`](docs/ui-design.md) — design system + component patterns
- [`docs/operations.md`](docs/operations.md) — deployment, backups, secret-key
  handling, upgrades, disaster recovery
- [`docs/seed-catalog.md`](docs/seed-catalog.md) — exercise seed-data format
- [`docs/COACH_VOICE.md`](docs/COACH_VOICE.md) — agent voice + behavior
- [`docs/adr/`](docs/adr/) — architecture decision records
- `internal/api/openapi/swagger.yaml` — generated OpenAPI spec (also served
  at `/api/docs`); regenerate with `just openapi`
