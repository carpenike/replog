# RepLog — Copilot Instructions

## Project Overview

RepLog is a self-hosted web app for tracking resistance training workouts. It serves a single family — kids following a tier-based progression system and adults running percentage-based programs (5/3/1, GZCL, etc.).

**Key principle:** The app is a logbook. A human coach makes all progression decisions — the app never automates coaching.

## Tech Stack

- **Go** (1.24+) with `html/template` — server-side rendering, no frontend framework
- **htmx** — all interactivity via `hx-get`, `hx-post`, `hx-swap` attributes; no JS build step
- **SQLite** (WAL mode) via `modernc.org/sqlite` — pure Go driver, no CGO
- **pressly/goose** — SQL migrations embedded in binary via `embed.FS`
- **alexedwards/scs** — session management (SQLite or signed cookie store)
- **bcrypt** via `golang.org/x/crypto/bcrypt` — password hashing
- **Nix flake** — builds a single static binary for NixOS deployment

## Project Structure

```
cmd/replog/
  main.go                     # Entrypoint: DB init, migrations, router, server start

internal/
  database/
    migrations/
      0001_initial_schema.sql # DDL from docs/data-model.md
    migrate.go                # embed.FS + goose RunMigrations()
    db.go                     # Open DB, set PRAGMAs, return *sql.DB
  handlers/                   # HTTP handlers grouped by domain
  middleware/                  # Auth, logging, etc.
  models/                     # Data access layer (queries, not ORM)
  templates/                  # html/template files

static/                       # htmx script, CSS, static assets

docs/
  data-model.md               # Schema source of truth (7 tables, DDL, ERD)
  requirements.md             # v1.0 user stories and acceptance criteria
  adr/                        # Architecture decision records

flake.nix                     # Nix build (buildGoModule)
```

## Architecture Decisions

Read the ADRs before making changes that affect these areas:

- [ADR 001](docs/adr/001-tech-stack.md) — Go + SQLite + htmx rationale
- [ADR 002](docs/adr/002-migrations.md) — Goose migrations with embed.FS, auto-run on startup
- [ADR 003](docs/adr/003-auth-sessions.md) — bcrypt + scs, env var bootstrap, coach vs non-coach access

## Database Schema

7 tables: `athletes`, `users`, `exercises`, `athlete_exercises`, `training_maxes`, `workouts`, `workout_sets`.

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

- Use stdlib `net/http` patterns — `http.HandlerFunc`, `http.ServeMux`
- No web framework (no gin, echo, chi, fiber) — keep dependencies minimal
- Error handling: wrap with `fmt.Errorf("context: %w", err)`, return errors up
- No ORM — write SQL queries directly in the models layer
- Keep handlers thin: validate input → call model → render template
- Use `html/template` for all HTML rendering — never write raw HTML strings in Go code
- `internal/` packages are not importable outside this module — use it for all app code

## htmx Patterns

- Return HTML fragments from endpoints, not JSON
- Use `hx-target` and `hx-swap` for partial page updates
- Use `hx-boost` on navigation links for SPA-like feel without JS
- Server returns appropriate HTTP status codes; htmx handles swap behavior
- Forms submit via `hx-post` with `hx-swap="outerHTML"` for inline updates

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
nix build                     # Build via Nix flake
```

## Issue Tracking (bd)

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Session Completion

When ending a work session, complete ALL steps. Work is NOT complete until `git push` succeeds.

1. File issues for remaining work (`bd` or GitHub issues)
2. Run quality gates if code changed — `go test ./...`, `go vet ./...`, `go build ./cmd/replog`
3. Update issue status — close finished work, update in-progress items
4. Push to remote:
   ```bash
   git pull --rebase
   bd sync
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
- `docs/adr/` — architecture decision records
