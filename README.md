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

- **Go** (1.25+) with `html/template` — server-side rendering, no frontend framework
- **htmx** — all interactivity via `hx-get`, `hx-post`, `hx-swap` attributes; no JS build step
- **SQLite** (WAL mode) via `modernc.org/sqlite` — pure-Go driver, no CGO
- **chi** — lightweight HTTP router with group-based middleware (`github.com/go-chi/chi/v5`)
- **Pico CSS** — classless CSS framework for semantic HTML styling
- **WebAuthn** — passkey authentication via `go-webauthn/webauthn`
- **Shoutrrr** — external notification dispatch (Slack, Discord, email, etc.)
- **Nix flake** — builds a single static binary for NixOS deployment

See [ADR 001](docs/adr/001-tech-stack.md) for rationale.

## Development

```bash
# Prerequisites: Go 1.25+, Nix (optional, for flake build)

# Run locally
go run ./cmd/replog

# Build
go build -o replog ./cmd/replog

# Run tests
go test ./...

# Build with Nix
nix build
```

### Environment for Local Development

```bash
export REPLOG_ADDR=":8080"
export REPLOG_DB_PATH="./dev.db"
export REPLOG_ADMIN_USER="admin"
export REPLOG_ADMIN_PASS="admin"
export REPLOG_ADMIN_EMAIL="admin@localhost"
export REPLOG_SECRET_KEY="dev-only-secret-key-not-for-prod!"
export REPLOG_WEBAUTHN_RPID="localhost"
export REPLOG_WEBAUTHN_ORIGINS="http://localhost:8080"
```

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

- [Requirements](docs/requirements.md) — user stories and acceptance criteria
- [Data Model](docs/data-model.md) — schema, relationships, DDL
- [UI Design](docs/ui-design.md) — design system and component patterns
- [Seed Catalog](docs/seed-catalog.md) — exercise seed data format
- [ADRs](docs/adr/) — architecture decision records
