# RepLog

Self-hosted workout tracking for kids' resistance training progression and personal lifting logs.

## What It Does

- **Athlete management** — track athletes (kids in a tier-based progression system, or adults running percentage-based programs like 5/3/1, GZCL)
- **Exercise library** — manage exercises with form cues, target reps, and equipment associations
- **Exercise assignments** — assign exercises to athletes with tier-based or custom progressions
- **Set-by-set workout logging** — log every set with weight, reps, and RPE; one workout per athlete per day
- **Training maxes** — track training max history for percentage-based programming
- **Program templates** — create program templates with prescribed sets, progression rules, and cycle reviews
- **Accessory plans** — prescribe supplemental/accessory work per athlete, decoupled from primary programs
- **Workout reviews** — coach approval workflow (approved / needs work)
- **Journal & notes** — unified athlete timeline with coach/athlete notes, pinning, and privacy controls
- **Body weight tracking** — per-athlete body weight log with charting
- **Import / Export** — import from Hevy, Strong; export to JSON/CSV; seed catalog import
- **AI-assisted program generation** — LLM-powered program suggestions via Anthropic, OpenAI, or Ollama (coach reviews all output)
- **Notifications** — in-app and external notifications (via Shoutrrr) with per-type preferences
- **Passkey / WebAuthn** — passwordless login alongside traditional username/password
- **Login tokens** — magic-link / token-based login for easy device setup
- **Equipment management** — equipment catalog with per-athlete and per-exercise associations
- **Avatars** — user avatar upload
- **Goal & tier history** — audit trail for progression changes

**Key principle:** The app is a logbook. A human coach makes all progression decisions — the app never automates coaching.

## Tech Stack

### Backend
- **Go** (1.25+) — single static binary serving a JSON REST API
- **chi** — HTTP router with group-based middleware (`github.com/go-chi/chi/v5`)
- **SQLite** (WAL mode) via `modernc.org/sqlite` — pure-Go driver, no CGO
- **WebAuthn** — passkey authentication via `go-webauthn/webauthn`
- **Shoutrrr** — external notification dispatch (Slack, Discord, email, etc.)

### Frontend
- **React 19 + TypeScript** with **Vite** — component-based SPA
- **shadcn/ui** + **Tailwind CSS v4** — styling and components
- **TanStack Query** — server state and caching
- **React Router v7** — client-side routing

### Deployment
- **Nix flake** — builds the Vite frontend then `go build`s the binary with `web/dist` embedded
- **Single static binary** for NixOS deployment

See [ADR 001](docs/adr/001-tech-stack.md) for the original Go/htmx rationale and [ADR 011](docs/adr/011-api-spa-frontend.md) for the move to a REST API + React SPA.

## Development

### Quick start (recommended)

The repo ships with a Nix flake devShell and a `Justfile`. With Nix and direnv:

```bash
git clone https://github.com/carpenike/replog
cd replog
direnv allow              # one-time: enters the flake devShell on every cd
just dev                  # boots backend + frontend together
```

Then open **<http://localhost:5173>** and log in with **admin / admin**.

What's running:
- Vite dev server on `:5173` — the frontend, with hot-reload
- Go backend on `:8080` — the JSON API, talking to a local SQLite file at `./dev.db`
- Vite proxies `/api` and `/avatars` to the backend, so the browser sees a single origin

On first launch the backend bootstraps the admin user and seeds the exercise catalog automatically. The dev DB lives at `./dev.db` (gitignored). Wipe it with `just db-reset` whenever you need a clean slate.

### Without Nix / direnv

You need Go 1.25+, Node.js 22+, and (optionally) [`just`](https://github.com/casey/just). Then:

```bash
just install              # cd web && npm install
just dev                  # same as above
```

Or skip `just` entirely and run the two processes by hand:

```bash
# Terminal 1
REPLOG_DB_PATH=./dev.db \
REPLOG_ADMIN_USER=admin REPLOG_ADMIN_PASS=admin REPLOG_ADMIN_EMAIL=admin@localhost \
REPLOG_WEBAUTHN_RPID=localhost \
REPLOG_WEBAUTHN_ORIGINS=http://localhost:5173,http://localhost:8080 \
go run ./cmd/replog

# Terminal 2
cd web && npm install && npm run dev
```

### VS Code

`.vscode/tasks.json` provides equivalent tasks:

- **Dev (server + frontend)** — runs both processes together (preferred)
- **Run Server** / **Vite Dev Server** — run them individually
- **Quality Gates (build + vet + test)** — full backend QA
- **Build**, **Test All**, **Test Current Package**, **Vet**

### Common Justfile recipes

```bash
just                # list all recipes
just dev            # run backend + frontend
just build          # build frontend bundle + Go binary
just test           # go test ./...
just lint           # go vet + npm run lint
just qa             # lint + test + build (matches CI)
just db-reset       # wipe ./dev.db and rebootstrap on next run
just db-shell       # open sqlite3 on ./dev.db
just db-backup      # WAL-safe backup of ./dev.db
just build-nix      # nix build
```

### LLM API keys

LLM provider keys (Anthropic, OpenAI) are **not** environment variables. Log in as admin, go to Settings, and configure them there. They're stored encrypted in the database using `REPLOG_SECRET_KEY` (auto-generated on first run if unset).

### Custom env overrides

For anything you don't want committed (a different port, a real LLM key for a personal smoke test, etc.), copy `.env.example` to `.env.local`. direnv loads it automatically; raw shell users can `source .env.local`.

## NixOS Deployment

Add this flake as an input to your nix-config:

```nix
{
  inputs.replog.url = "github:carpenike/replog";
}
```

The binary runs as a systemd service with SQLite stored in `StateDirectory`. See your nix-config repo for the full module (caddy reverse proxy, health checks, backups).

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `REPLOG_ADDR` | `:8080` | Listen address (e.g. `127.0.0.1:8080` to bind loopback only behind a proxy) |
| `REPLOG_DB_PATH` | `replog.db` | Path to SQLite database file |
| `REPLOG_BASE_URL` | *(inferred)* | External base URL (e.g. `https://replog.example.com`). Used for generating absolute URLs and auto-enables secure cookies when scheme is `https` |
| `REPLOG_SECURE_COOKIES` | *(auto)* | Override cookie `Secure` flag (`true`/`false`). Auto-derived from `REPLOG_BASE_URL` scheme if not set |
| `REPLOG_SECRET_KEY` | *(auto-generated)* | Encryption key for sensitive settings stored in DB (LLM API keys, etc.). Auto-generated and persisted if not set |
| `REPLOG_AVATAR_DIR` | `avatars/` (sibling of DB) | Directory for avatar file storage |
| `REPLOG_SEED_CATALOG` | *(embedded)* | Path to a custom seed catalog JSON file (overrides the built-in exercise catalog) |
| `REPLOG_ADMIN_USER` | | Initial admin username (required on first run) |
| `REPLOG_ADMIN_PASS` | | Initial admin password (required on first run) |
| `REPLOG_ADMIN_EMAIL` | | Initial admin email |
| `REPLOG_TRUSTED_PROXIES` | | Comma-separated CIDRs or IPs whose `X-Forwarded-For` headers are trusted for rate limiting (e.g. `127.0.0.1,10.0.0.0/8`) |
| `REPLOG_WEBAUTHN_RPID` | | WebAuthn Relying Party ID (e.g. `replog.example.com`) |
| `REPLOG_WEBAUTHN_ORIGINS` | | Comma-separated WebAuthn origins (e.g. `https://replog.example.com`) |

LLM provider/model settings and notification configuration are managed through the admin settings UI (`/admin/settings`), not environment variables.

### Reverse Proxy

When deploying behind a reverse proxy (Caddy, nginx, etc.):

1. Set `REPLOG_BASE_URL` to the external URL (e.g. `https://replog.example.com`)
2. Set `REPLOG_ADDR` to `127.0.0.1:8080` to restrict direct access
3. Ensure the proxy forwards `Host`, `X-Forwarded-Proto`, and `X-Forwarded-For` headers
4. `REPLOG_SECURE_COOKIES` is auto-derived from the `REPLOG_BASE_URL` scheme — no need to set it separately

## Documentation

- [Operations Guide](docs/operations.md) — production deployment, backups, secret-key handling, upgrades, disaster recovery
- [Requirements](docs/requirements.md) — user stories and acceptance criteria
- [Data Model](docs/data-model.md) — schema, relationships, DDL
- [UI Design](docs/ui-design.md) — design system and component patterns
- [Seed Catalog](docs/seed-catalog.md) — exercise seed data format
- [ADRs](docs/adr/) — architecture decision records
