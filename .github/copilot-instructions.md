# RepLog — Copilot Instructions

## Project Overview

RepLog is a self-hosted web app for tracking resistance training workouts. It serves a single family — kids following a tier-based progression system and adults running percentage-based programs (5/3/1, GZCL, etc.).

**Key principle:** The app is a logbook. A human coach makes all progression decisions — the app never automates coaching.

## Tech Stack

### Backend (Go)
- **Go** (1.25+) — single static binary, JSON REST API only (no SSR)
- **chi** — HTTP router with group-based middleware (`github.com/go-chi/chi/v5`)
- **SQLite** (WAL mode) via `modernc.org/sqlite` — pure-Go driver, no CGO
- **pressly/goose** — SQL migrations embedded in binary via `embed.FS`
- **alexedwards/scs** — session management (SQLite store)
- **go-webauthn/webauthn** — passkey / WebAuthn authentication
- **containrrr/shoutrrr** — external notification dispatch (Slack, Discord, email, etc.)
- **bcrypt** via `golang.org/x/crypto/bcrypt` — password hashing

### Frontend (React SPA)
- **React 19 + TypeScript** — component-based UI, no SSR
- **Vite** — dev server and production bundler
- **shadcn/ui** — components copied into `web/src/components/ui/`
- **Tailwind CSS v4** — utility-first styling
- **TanStack Query** (`@tanstack/react-query`) — server state, caching, refetching
- **React Router v7** — client-side routing
- **lucide-react** — icon set

### Deployment
- **Nix flake** — builds the Vite frontend then `go build`s the binary with `web/dist` embedded
- **Single binary** — the Go binary serves the API and the embedded SPA from one process

## Project Structure

```
cmd/replog/
  main.go                     # Entrypoint: DB init, migrations, router, server start

internal/
  api/                        # JSON REST API handlers (the only HTTP layer)
  database/
    migrations/
      0001_initial_schema.sql # DDL from docs/data-model.md
    migrate.go                # embed.FS + goose RunMigrations()
    db.go                     # Open DB, set PRAGMAs, return *sql.DB
    seed.go                   # Exercise catalog seeding from JSON
  importers/                  # Workout import parsers (Hevy, Strong, RepLog JSON, catalog)
  llm/                        # LLM integration (Anthropic, OpenAI, Ollama providers)
  middleware/                 # Auth, CORS, logging, rate limiting, security headers
  models/                     # Data access layer (queries, not ORM)
  notify/                     # Notification dispatch (in-app + external via shoutrrr)
  passkeys/                   # WebAuthn ceremony endpoints (JSON, used by SPA)
  scheduler/                  # Background maintenance (token cleanup, etc.)

web/                          # React SPA (Vite)
  embed.go                    # //go:embed all:dist
  src/
    api/                      # Typed API client and types
    components/               # Shared components (Layout, ui/)
    pages/                    # Route components (one per page)
  index.html
  package.json
  vite.config.ts

avatars/                      # Avatar file storage (runtime, not embedded)

docs/
  data-model.md               # Schema source of truth (27 tables, DDL, ERD)
  requirements.md             # v1.0 user stories and acceptance criteria
  ui-design.md                # Design system and component patterns
  openapi/                  # Generated OpenAPI spec (do not edit by hand)
    swagger.yaml            # Run `just openapi` to regenerate from swag annotations
  seed-catalog.md             # Exercise seed data format
  adr/                        # Architecture decision records (13 ADRs)

flake.nix                     # Nix build (frontend + backend)
```

## Architecture Decisions

Read the ADRs before making changes that affect these areas:

- [ADR 001](docs/adr/001-tech-stack.md) — Original Go + SQLite + htmx rationale (superseded by ADR 011 for the frontend)
- [ADR 002](docs/adr/002-migrations.md) — Goose migrations with embed.FS, auto-run on startup
- [ADR 003](docs/adr/003-auth-sessions.md) — bcrypt + scs, env var bootstrap, coach vs non-coach access
- [ADR 004](docs/adr/004-ui-pico-css.md) — Pico CSS (historical — frontend has moved to React + Tailwind via ADR 011)
- [ADR 005](docs/adr/005-chi-router.md) — Adopt chi router for group-based middleware
- [ADR 006](docs/adr/006-import-export.md) — Workout import / export
- [ADR 007](docs/adr/007-llm-coach.md) — LLM-assisted program generation
- [ADR 008](docs/adr/008-notifications.md) — Notification system
- [ADR 009](docs/adr/009-wizard-framework.md) — Wizard framework for setup flows
- [ADR 010](docs/adr/010-supplemental-programs.md) — Supplemental programs: multiple active programs per athlete
- [ADR 011](docs/adr/011-api-spa-frontend.md) — REST API + React SPA frontend (current architecture)
- [ADR 012](docs/adr/012-api-test-harness.md) — Shared API test harness (`internal/api/handlers_test.go`)
- [ADR 013](docs/adr/013-openapi-swag.md) — OpenAPI spec generated via `swaggo/swag` annotations

## Database Schema

27 tables: `athletes`, `users`, `user_preferences`, `exercises`, `athlete_exercises`, `training_maxes`, `workouts`, `workout_sets`, `body_weights`, `goal_history`, `tier_history`, `athlete_notes`, `workout_reviews`, `program_templates`, `prescribed_sets`, `athlete_programs`, `progression_rules`, `accessory_plans`, `login_tokens`, `webauthn_credentials`, `equipment`, `exercise_equipment`, `athlete_equipment`, `notifications`, `notification_preferences`, `sessions`, `app_settings`.

Full DDL, constraints, indexes, and triggers are in `docs/data-model.md` — that file is the source of truth.

Key patterns:
- One workout per athlete per day (`UNIQUE(athlete_id, date)`)
- One set = one row in `workout_sets` (per-set logging, not per-exercise aggregates)
- Active assignments use partial unique index (`WHERE active = 1`)
- Training max history: multiple rows per athlete+exercise, current = latest `effective_date`
- `updated_at` triggers use `WHEN OLD.updated_at = NEW.updated_at` guard to prevent recursion
- Foreign key ON DELETE: CASCADE from athletes, RESTRICT from exercises (protect logged history), SET NULL for users.athlete_id and workout_reviews.coach_id
- Coach ownership: `athletes.coach_id` FK to `users(id)` — coaches only see/manage their assigned athletes

## SQLite Rules

- Always call `db.SetMaxOpenConns(1)` — SQLite is single-writer
- Always set PRAGMAs on connection open: `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`
- Use `modernc.org/sqlite` (pure Go) — never import `mattn/go-sqlite3` (requires CGO)
- Backups: `sqlite3 replog.db ".backup backup.db"` — never `cp` a live WAL-mode DB
- Use `?` placeholders for query parameters, not `fmt.Sprintf`

## Coding Standards

### Backend
- Use stdlib `net/http` patterns — `http.HandlerFunc`, `http.Handler` interfaces
- Use `chi` router (`github.com/go-chi/chi/v5`) for routing with group-based middleware
- No full web framework (no gin, echo, fiber) — chi is the only router dependency
- Error handling: wrap with `fmt.Errorf("context: %w", err)`, return errors up
- No ORM — write SQL queries directly in the models layer
- Keep handlers thin: validate input → call model → return JSON via `api.WriteJSON` / `api.WriteError`
- All handler responses are JSON — never write raw HTML strings in Go code
- `internal/` packages are not importable outside this module — use it for all app code

### Frontend
- Use TanStack Query for all server data — don't `useEffect` + `fetch` directly
- Use the typed `api` client in `web/src/api/` — don't construct `fetch()` calls in components
- Components live in `web/src/components/`; route components live in `web/src/pages/`
- Use shadcn/ui components from `web/src/components/ui/` rather than building primitives from scratch
- Tailwind utility classes for styling; avoid inline styles
- Never declare component functions inside other components — hoist them to module scope
- Follow React 19 rules: don't `setState` inside `useEffect` for derivable values; initialize state from props in `useState(initializer)`

### API contract
- The OpenAPI spec at `internal/api/openapi/swagger.yaml` is **generated** by [swaggo/swag](https://github.com/swaggo/swag) from annotations on each handler. Run `just openapi` after adding/changing a route or DTO. CI fails if the committed spec is stale.
- The spec is served at `/api/docs` (Swagger UI) and `/api/docs/openapi.yaml`.
- When adding a new handler, copy the annotation block from a similar handler in `internal/api/handlers.go` (Login, Me, Dashboard, ListAthletes are good templates).
- Auth uses session cookies (SameSite=Lax) — no JWT, no Authorization header
- Same-origin in production (SPA served from the Go binary); CORS enabled in dev for the Vite dev server

## Auth & Access Control

- Three tiers: admin (`is_admin = 1`), coach (`is_coach = 1`), athlete (non-coach)
- Roles overlap: an admin can also be a coach, an athlete can also be a coach
- Admins see and manage all athletes, exercises, assignments, workouts, and users
- Coaches see and manage only athletes assigned to them via `athletes.coach_id`
- Non-coaches (athletes) are linked to one athlete via `users.athlete_id` — can only view/log their own
- Unlinked non-coach users see an informative message, not a blank screen
- User management is admin-only
- First-run bootstrap: create admin+coach from `REPLOG_ADMIN_USER`, `REPLOG_ADMIN_PASS`, `REPLOG_ADMIN_EMAIL` env vars
- Session lifetime: 30 days, `HttpOnly`, `SameSite=Lax`

## Build & Run

```bash
go run ./cmd/replog            # Run locally
go build -o replog ./cmd/replog # Build binary
go test ./...                  # Run tests
go vet ./...                   # Static analysis
nix build                     # Build via Nix flake
```

## Issue Tracking

GitHub Issues is the source of truth for tracked work. Use the `gh` CLI:

```bash
gh issue list                                       # Browse open issues
gh issue view <number>                              # View details
gh issue create --title "..." --body "..."          # File new work
gh issue edit <number> --add-label "in-progress"    # Claim work
gh issue close <number> --comment "..."             # Complete work
```

## Session Completion

When ending a work session, complete ALL steps. Work is NOT complete until `git push` succeeds.

1. File GitHub issues for any remaining work
2. Run quality gates if code changed — `go test ./...`, `go vet ./...`, `go build ./cmd/replog`
3. Update issue status — close finished work, comment on in-progress items
4. Push to remote:
   ```bash
   git pull --rebase
   git push
   ```
5. Verify `git status` shows "up to date with origin"

**Critical rules:**
- NEVER stop before pushing — that leaves work stranded locally
- NEVER say "ready to push when you are" — YOU must push
- If push fails, resolve and retry until it succeeds

## Resources

- `docs/data-model.md` — complete schema, ERD, DDL, seed data, operational notes
- `docs/requirements.md` — all v1.0 user stories with acceptance criteria
- `docs/ui-design.md` — design system and component patterns
- `docs/operations.md` — production deployment, backups, secret-key handling, upgrades
- `internal/api/openapi/swagger.yaml` — generate3 OpenAPI spec for the REST API (also at `/api/docs`); regenerate with `just openapi`
- `docs/seed-catalog.md` — exercise seed data format
- `docs/adr/` — architecture decision records (10 ADRs)
