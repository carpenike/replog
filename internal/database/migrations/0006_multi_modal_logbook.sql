-- +goose Up

-- Multi-modal athletic logbook, Phase 1 (ADR 018 / HOF-008 / #26).
--
-- Turns the single-discipline session model into a multi-modal one and lands
-- the throwing / arm-care safety surface. Additive only (ADR 002 pre-prod
-- policy is moot here — this migration is shaped to be safe even post-prod:
-- it preserves every existing `workouts` row and defaults them to
-- discipline='resistance', so current resistance behaviour is unchanged).
--
-- The session parent keeps the name `workouts` (renaming would churn ~23 SQL
-- query sites across 7 model files for zero functional gain — see HOF-008
-- DISCUSSION). We add a `discipline` discriminator and widen per-day
-- uniqueness from UNIQUE(athlete_id, date) to UNIQUE(athlete_id, date,
-- discipline) so an athlete can log a lift AND a throwing session on the same
-- date.
--
-- Widening that constraint requires a SQLite table rebuild. goose runs each
-- migration inside a transaction, where `PRAGMA foreign_keys=OFF` is a NO-OP.
-- We therefore use `PRAGMA defer_foreign_keys=ON`, which IS transaction-safe:
-- it suspends foreign-key enforcement until COMMIT, then runs the check once.
-- `workout_sets` and `workout_reviews` reference `workouts(id)` by name; we
-- preserve every id during the copy, so the deferred check passes at COMMIT.
-- A belt-and-suspenders `PRAGMA foreign_key_check` runs before COMMIT and
-- aborts the migration if anything dangles.

PRAGMA defer_foreign_keys = ON;

-- 1. New parent shape: same columns + discipline, widened UNIQUE.
CREATE TABLE workouts_new (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id    INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    assignment_id INTEGER REFERENCES athlete_programs(id) ON DELETE SET NULL,
    date          DATE    NOT NULL,
    discipline    TEXT    NOT NULL DEFAULT 'resistance'
                  CHECK(discipline IN ('resistance','conditioning','throwing','skill','recovery')),
    notes         TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- assignment_id only ever attaches to a coach-prescribed resistance
    -- session. Non-resistance disciplines are log-and-flag (no program), so
    -- this invariant protects GetPrescription's position counter, which keys
    -- off assignment_id and date.
    CHECK(assignment_id IS NULL OR discipline = 'resistance'),
    UNIQUE(athlete_id, date, discipline)
);

-- 2. Copy every existing row; existing sessions are resistance by definition.
INSERT INTO workouts_new (id, athlete_id, assignment_id, date, discipline, notes, created_at, updated_at)
SELECT id, athlete_id, assignment_id, date, 'resistance', notes, created_at, updated_at
FROM workouts;

-- 3. Swap. Dropping `workouts` also drops its index + trigger; recreate below.
DROP TABLE workouts;
ALTER TABLE workouts_new RENAME TO workouts;

-- 4. Recreate the index and updated_at trigger that lived on the old table.
CREATE INDEX IF NOT EXISTS idx_workouts_assignment_id
    ON workouts(assignment_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_workouts_updated_at
AFTER UPDATE ON workouts FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workouts SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- defer_foreign_keys=ON resets to OFF at COMMIT, at which point every deferred
-- FK constraint (workout_sets, workout_reviews → workouts) is checked once. We
-- preserved all ids in the copy above, so the check passes; if a row were
-- orphaned, goose's COMMIT would fail and roll the whole migration back.

-- ---------------------------------------------------------------------------
-- Season phases — coach-recorded off/pre/in-season windows per athlete+sport.
-- Drives load expectations and is surfaced as journal events.
CREATE TABLE IF NOT EXISTS athlete_season_phases (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    sport      TEXT,
    phase      TEXT    NOT NULL CHECK(phase IN ('off','pre','in')),
    start_date DATE    NOT NULL,
    end_date   DATE,
    notes      TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_athlete_season_phases_athlete
    ON athlete_season_phases(athlete_id, start_date);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_athlete_season_phases_updated_at
AFTER UPDATE ON athlete_season_phases FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE athlete_season_phases SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Throwing sessions — the detail row for a discipline='throwing' workout.
-- One throwing_session per throwing workout (the parent carries date/athlete).
CREATE TABLE IF NOT EXISTS throwing_sessions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id  INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    throw_type  TEXT    NOT NULL
                CHECK(throw_type IN ('game','bullpen','lesson','long_toss','catch','flat_ground')),
    throw_count INTEGER,
    max_intent  INTEGER,
    velocity    REAL,
    fatigue     INTEGER NOT NULL DEFAULT 0 CHECK(fatigue IN (0,1)),
    pain        INTEGER NOT NULL DEFAULT 0 CHECK(pain IN (0,1)),
    source      TEXT    NOT NULL DEFAULT 'program' CHECK(source IN ('program','external')),
    team        TEXT,
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_throwing_sessions_workout
    ON throwing_sessions(workout_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_throwing_sessions_updated_at
AFTER UPDATE ON throwing_sessions FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE throwing_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Pitch Smart reference limits (MLB / USA Baseball). READ-ONLY advisory
-- reference data — the app computes a coach-facing flag (rest days owed,
-- daily max), never an auto-action and never a hard log-block. rest_thresholds
-- is a JSON array of {"max":N,"rest":N} rows ordered ascending by `max`.
CREATE TABLE IF NOT EXISTS pitch_smart_limits (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    age_min         INTEGER NOT NULL,
    age_max         INTEGER NOT NULL,
    daily_max       INTEGER NOT NULL,
    rest_thresholds TEXT    NOT NULL
);

INSERT INTO pitch_smart_limits (age_min, age_max, daily_max, rest_thresholds) VALUES
    (7,  8,  50,  '[{"max":20,"rest":0},{"max":35,"rest":1},{"max":50,"rest":2},{"max":1000,"rest":3}]'),
    (9,  10, 75,  '[{"max":20,"rest":0},{"max":35,"rest":1},{"max":50,"rest":2},{"max":65,"rest":3},{"max":1000,"rest":4}]'),
    (11, 12, 85,  '[{"max":20,"rest":0},{"max":35,"rest":1},{"max":50,"rest":2},{"max":65,"rest":3},{"max":1000,"rest":4}]'),
    (13, 14, 95,  '[{"max":20,"rest":0},{"max":35,"rest":1},{"max":50,"rest":2},{"max":65,"rest":3},{"max":1000,"rest":4}]'),
    (15, 16, 95,  '[{"max":30,"rest":0},{"max":45,"rest":1},{"max":60,"rest":2},{"max":75,"rest":3},{"max":1000,"rest":4}]'),
    (17, 18, 105, '[{"max":30,"rest":0},{"max":45,"rest":1},{"max":60,"rest":2},{"max":75,"rest":3},{"max":1000,"rest":4}]');

-- ---------------------------------------------------------------------------
-- Bio samples — append-only manual or watch-imported biometric readings
-- (e.g. resting HR, HRV, sleep). No updated_at trigger: rows are immutable.
CREATE TABLE IF NOT EXISTS bio_samples (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    recorded_at DATETIME NOT NULL,
    metric      TEXT    NOT NULL,
    value       REAL    NOT NULL,
    unit        TEXT,
    source      TEXT    NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','watch_import')),
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bio_samples_athlete
    ON bio_samples(athlete_id, recorded_at);

-- +goose Down

DROP TABLE IF EXISTS bio_samples;
DROP TABLE IF EXISTS pitch_smart_limits;
DROP TABLE IF EXISTS throwing_sessions;
DROP TABLE IF EXISTS athlete_season_phases;

-- Reverse the workouts rebuild: drop the discipline column and restore the
-- original UNIQUE(athlete_id, date). Mirror of the Up rebuild.
PRAGMA defer_foreign_keys = ON;

CREATE TABLE workouts_old (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id    INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    assignment_id INTEGER REFERENCES athlete_programs(id) ON DELETE SET NULL,
    date          DATE    NOT NULL,
    notes         TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, date)
);

-- Only resistance rows can survive the narrowed uniqueness; any non-resistance
-- rows are dropped on down-migration (their detail tables are already gone).
INSERT INTO workouts_old (id, athlete_id, assignment_id, date, notes, created_at, updated_at)
SELECT id, athlete_id, assignment_id, date, notes, created_at, updated_at
FROM workouts WHERE discipline = 'resistance';

DROP TABLE workouts;
ALTER TABLE workouts_old RENAME TO workouts;

CREATE INDEX IF NOT EXISTS idx_workouts_assignment_id
    ON workouts(assignment_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_workouts_updated_at
AFTER UPDATE ON workouts FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workouts SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd
