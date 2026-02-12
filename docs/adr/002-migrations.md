# ADR 002: Database Migrations — Embedded via Goose

**Status:** Accepted
**Date:** 2026-02-12

## Context

The schema will evolve (v1.1 adds RPE, program templates, etc.). We need a migration strategy that:
- Works with a single static binary (no external tooling on the deploy target)
- Runs automatically — no manual migration steps on the NixOS host
- Keeps migration files in version control alongside the code that depends on them

## Decision

Use **[pressly/goose](https://github.com/pressly/goose)** with SQL migration files embedded in the Go binary via `embed.FS`. Migrations run automatically on application startup before the HTTP server starts.

```
internal/
  database/
    migrations/
      0001_initial_schema.sql
      0002_add_rpe.sql
      ...
    migrate.go          # embeds migrations, exposes RunMigrations()
```

## Rationale

- **goose** is lightweight, supports SQLite, and has a clean Go API for embedded usage
- `embed.FS` means migration files compile into the binary — no filesystem dependency at runtime
- Auto-migrate on startup is safe for single-instance SQLite (no parallel migration risk)
- Alternatives considered: `golang-migrate/migrate` (heavier API surface, same outcome), hand-rolled `user_version` pragma (too manual past ~5 migrations)

## Consequences

- The DDL in `docs/data-model.md` becomes the initial `0001_initial_schema.sql` migration
- All schema changes go through numbered migration files — no ad-hoc `ALTER TABLE` in app code
- Rollbacks are possible via goose's down migrations but not expected in practice (single-user system, just fix forward)
