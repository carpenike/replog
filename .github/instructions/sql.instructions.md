---
applyTo: "**/*.sql"
---

# SQL Conventions for RepLog

## SQLite Specifics

- Target SQLite via `modernc.org/sqlite` — pure Go, no CGO
- All DDL uses `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`
- Use `INTEGER PRIMARY KEY AUTOINCREMENT` for all primary keys
- Boolean columns are `INTEGER` with `CHECK(col IN (0, 1))`
- Use `DATETIME` for timestamps, `DATE` for date-only fields
- Nullable text uses `TEXT` with no `NOT NULL` — default is NULL
- Case-insensitive text matching: use `COLLATE NOCASE` on columns that need it

## Migrations

- Migration files live in `internal/database/migrations/`
- Use goose naming: `NNNN_description.sql` (e.g., `0001_initial_schema.sql`)
- Each file has `-- +goose Up` and `-- +goose Down` sections
- Always include a down migration, even if it's a DROP
- Schema source of truth is `docs/data-model.md` — migrations implement it

## Query Patterns

- Use `?` placeholders — never interpolate values into SQL strings
- Use `RETURNING` clause for INSERT/UPDATE when you need the new row back
- For "current training max": `ORDER BY effective_date DESC LIMIT 1`
- For "active assignments": `WHERE active = 1`
- Always include `LIMIT` on list queries that could grow unbounded

## Foreign Key Behaviors

- `ON DELETE CASCADE` from athletes → workouts, assignments, training maxes
- `ON DELETE RESTRICT` from exercises → workout_sets (protect logged history)
- `ON DELETE SET NULL` from athletes → users.athlete_id (unlink, don't delete user)

## Triggers

- `updated_at` triggers use `WHEN OLD.updated_at = NEW.updated_at` guard to prevent infinite recursion
- Always use `AFTER UPDATE ... FOR EACH ROW` pattern
