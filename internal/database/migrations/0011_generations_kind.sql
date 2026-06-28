-- +goose Up

-- Generation kind discriminator (HOF-015 — Ad-hoc WOD generator).
--
-- The async generation pipeline (ADR 015) is now reused for two distinct
-- artifacts:
--   * 'program' — the existing multi-week AI Coach draft, committed via
--     ExecuteCatalogImport into an UNASSIGNED program_template.
--   * 'wod'     — a single-session, Sarge-circuit "workout of the day"
--     committed as a discipline='resistance', assignment_id NULL ad-hoc
--     workout (sets seeded from the generated CatalogJSON, log-or-discard).
--
-- The column lets the two kinds coexist without contention: the program
-- GeneratePage resume path and the per-athlete duplicate-submit guard
-- filter to kind='program', so a WOD in flight neither blocks nor bleeds
-- into a normal program draft (and vice versa). Resolves HOF-015 review
-- findings (d) and the in-flight contention question.
--
-- Additive only (ADR 002 — the binary is already deployed with a populated
-- DB; existing rows default to 'program', preserving current behavior).

ALTER TABLE generations ADD COLUMN kind TEXT NOT NULL DEFAULT 'program'
    CHECK(kind IN ('program', 'wod'));

-- Per-athlete in-flight / resume lookups now filter by kind as well.
CREATE INDEX IF NOT EXISTS idx_generations_athlete_kind_status
    ON generations(athlete_id, kind, status);

-- +goose Down

-- SQLite >= 3.35 (modernc.org/sqlite ships a newer engine) supports
-- DROP COLUMN. Down is documented but never run in production per ADR 002.
DROP INDEX IF EXISTS idx_generations_athlete_kind_status;
ALTER TABLE generations DROP COLUMN kind;
