# ADR 001: Tech Stack — Go + SQLite + htmx

**Status:** Accepted
**Date:** 2026-02-12

## Context

RepLog is a self-hosted web app for tracking resistance training. It serves a single family (≈4 users), handles ~100 rows/month of workout data, and deploys on a NixOS home server via systemd. The deployment target is a separate nix-config repo that consumes this project as a flake.

We need a stack that produces a single static binary with zero runtime dependencies, minimal operational overhead, and no JavaScript build toolchain.

## Decision

- **Backend:** Go with `html/template` for server-side rendering
- **Interactivity:** htmx for dynamic UI without a JS build step
- **Database:** SQLite in WAL mode, embedded in the Go binary via `modernc.org/sqlite` (pure Go, no CGO)
- **Auth:** Simple session cookie with bcrypt passwords — no SSO, no OAuth
- **Deployment:** Nix flake producing a static binary; NixOS module with systemd service (in nix-config repo)

## Rationale

### Why Go

- Compiles to a single static binary — trivial to package in a Nix derivation
- `html/template` is stdlib, no framework dependency
- Strong SQLite ecosystem (`modernc.org/sqlite` for pure Go, `crawshaw.io/sqlite` for CGO)
- Fast cold starts, low memory footprint — appropriate for a home server
- Team familiarity

### Why SQLite

- 4 users, ~100 rows/month — PostgreSQL is wildly overkill
- Embedded in the binary — no separate database service to manage
- WAL mode handles concurrent reads with single-writer without issue at this scale
- Backup is `cp replog.db replog.db.bak` (or restic file path)
- `StateDirectory` in systemd keeps the DB file in a predictable location
- No connection pooling, no network latency, no auth configuration

### Why htmx (not React/Vue/Svelte)

- No JavaScript build step — htmx is a single `<script>` tag
- Server-rendered HTML means Go templates are the only view layer
- CRUD app with forms and lists — htmx's `hx-get`, `hx-post`, `hx-swap` cover 100% of the interactivity
- Dramatically less complexity than an SPA + API architecture
- Progressive enhancement — works without JS for basic form submissions

### Why not [alternatives]

| Alternative | Why not |
|------------|---------|
| PostgreSQL | Operational overhead (service, auth, backups) for 4 users and 100 rows/month |
| React/Next.js | JS build toolchain, node_modules, hydration complexity — all for a CRUD app |
| Python/Django | Works, but doesn't produce a static binary; Nix packaging is more complex |
| Rust | Longer development time for marginal performance gains we don't need |

## Consequences

- **Positive:** Single binary deployment, zero runtime dependencies, simple Nix packaging, fast iteration with `go run`
- **Positive:** No JS build step means the full stack is Go templates + htmx — one language, one build
- **Negative:** htmx has limits for complex client-side state (drag-and-drop, offline mode) — acceptable for v1
- **Negative:** SQLite single-writer constraint is fine at this scale but would require migration if the app ever needed multi-server deployment
- **Accepted risk:** `modernc.org/sqlite` (pure Go) is slightly slower than CGO sqlite3 — irrelevant at this data volume
