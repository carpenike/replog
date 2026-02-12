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
# Prerequisites: Go 1.22+, Nix (optional, for flake build)

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

## Documentation

- [Requirements](docs/requirements.md) — user stories and acceptance criteria
- [Data Model](docs/data-model.md) — schema, relationships, DDL
- [ADRs](docs/adr/) — architecture decision records

## Build Order

1. ~~Data model design~~ ✅
2. Exercise library CRUD
3. Athlete profiles + exercise assignment
4. Daily workout view + logging (core loop)
5. History / progress view
