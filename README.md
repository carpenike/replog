# RepLog

Self-hosted workout tracking for kids' resistance training progression and personal lifting logs.

## What It Does

- Track athletes (kids in a tier-based progression system, or adults running their own programs)
- Manage an exercise library with form cues and target reps
- Assign exercises to athletes and log workouts set-by-set
- Track training maxes for percentage-based programming
- View workout history and progression over time

**Key principle:** The app tracks data. A human coach makes all progression decisions.

## Tech Stack

- **Go** with `html/template` + **htmx** — no JS build step
- **SQLite** (WAL mode) — embedded, zero ops overhead
- **Nix flake** — builds a static binary, consumable by NixOS systems

See [docs/adr/001-tech-stack.md](docs/adr/001-tech-stack.md) for rationale.

## Development

```bash
# Prerequisites: Go 1.24+, Nix (optional, for flake build)

# Run locally
go run ./cmd/replog

# Build
go build -o replog ./cmd/replog

# Build with Nix
nix build
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
| `REPLOG_ADMIN_USER` | | Initial admin username (required on first run) |
| `REPLOG_ADMIN_PASS` | | Initial admin password (required on first run) |
| `REPLOG_ADMIN_EMAIL` | | Initial admin email |
| `REPLOG_WEBAUTHN_RPID` | | WebAuthn Relying Party ID (e.g. `replog.example.com`) |
| `REPLOG_WEBAUTHN_ORIGINS` | | Comma-separated WebAuthn origins (e.g. `https://replog.example.com`) |

### Reverse Proxy

When deploying behind a reverse proxy (Caddy, nginx, etc.):

1. Set `REPLOG_BASE_URL` to the external URL (e.g. `https://replog.example.com`)
2. Set `REPLOG_ADDR` to `127.0.0.1:8080` to restrict direct access
3. Ensure the proxy forwards `Host`, `X-Forwarded-Proto`, and `X-Forwarded-For` headers
4. `REPLOG_SECURE_COOKIES` is auto-derived from the `REPLOG_BASE_URL` scheme — no need to set it separately

## Documentation

- [Requirements](docs/requirements.md) — user stories and acceptance criteria
- [Data Model](docs/data-model.md) — schema, relationships, DDL
- [ADRs](docs/adr/) — architecture decision records

## Build Order

1. ~~Data model design~~ ✅
2. ~~Exercise library CRUD~~ ✅
3. ~~Athlete profiles + exercise assignment~~ ✅
4. ~~Daily workout view + logging (core loop)~~ ✅
5. ~~History / progress view~~ ✅
