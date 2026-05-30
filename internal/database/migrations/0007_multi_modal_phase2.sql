-- +goose Up

-- Multi-modal athletic logbook, Phase 2 (ADR 018 / HOF-009 / #27).
--
-- Purely ADDITIVE. The session parent `workouts` already carries the
-- `discipline` discriminator (migration 0006), so this migration adds new
-- detail tables only — no parent rebuild, no constraint change, no
-- `defer_foreign_keys` dance (that was forced in 0006 solely by the UNIQUE
-- widening). Each detail row hangs off a `workouts` row via
-- `workout_id ... ON DELETE CASCADE`, mirroring `throwing_sessions`.
--
-- Adds three first-class disciplines — conditioning, sport-skill, recovery —
-- plus a child interval table for conditioning. The weekly cross-modal load
-- view (GET /athletes/{id}/load) is a pure read computed in the model layer;
-- it needs no schema of its own.

-- ---------------------------------------------------------------------------
-- Conditioning sessions — detail row for a discipline='conditioning' workout.
-- One per conditioning workout (the parent carries athlete + date).
CREATE TABLE IF NOT EXISTS conditioning_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id       INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    modality         TEXT    NOT NULL
                     CHECK(modality IN ('run','row','bike','sprint','circuit','swim','other')),
    session_type     TEXT    NOT NULL
                     CHECK(session_type IN ('steady','interval','sprint','tempo')),
    total_distance   REAL,
    distance_unit    TEXT    CHECK(distance_unit IN ('m','km','yd','mi')),
    duration_seconds INTEGER,
    avg_hr           INTEGER,
    rpe              REAL    CHECK(rpe IS NULL OR (rpe >= 1 AND rpe <= 10)),
    notes            TEXT,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_conditioning_sessions_workout
    ON conditioning_sessions(workout_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_conditioning_sessions_updated_at
AFTER UPDATE ON conditioning_sessions FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE conditioning_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Conditioning intervals — child rows of a conditioning session (one-to-many,
-- like workout_sets → workouts). Written once with their session and read back
-- ordered by interval_number; immutable, so no updated_at/trigger.
CREATE TABLE IF NOT EXISTS conditioning_intervals (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    conditioning_session_id INTEGER NOT NULL
                            REFERENCES conditioning_sessions(id) ON DELETE CASCADE,
    interval_number         INTEGER NOT NULL,
    work_seconds            INTEGER,
    work_distance           REAL,
    rest_seconds            INTEGER,
    notes                   TEXT,
    UNIQUE(conditioning_session_id, interval_number)
);

CREATE INDEX IF NOT EXISTS idx_conditioning_intervals_session
    ON conditioning_intervals(conditioning_session_id, interval_number);

-- ---------------------------------------------------------------------------
-- Skill sessions — detail row for a discipline='skill' workout (sport-skill
-- work: batting, fielding, agility, med-ball, etc.). `load_kg` records
-- med-ball / implement load as a youth-safety datum — it is logged data, never
-- a prescribed target (ADR 018 #7: no weighted-implement automation).
CREATE TABLE IF NOT EXISTS skill_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id       INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    skill_type       TEXT    NOT NULL
                     CHECK(skill_type IN ('batting','fielding','throwing_accuracy','agility','medball','sprint','other')),
    rep_count        INTEGER,
    load_kg          REAL,
    velocity         REAL,
    duration_seconds INTEGER,
    notes            TEXT,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_skill_sessions_workout
    ON skill_sessions(workout_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_skill_sessions_updated_at
AFTER UPDATE ON skill_sessions FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE skill_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Recovery check-ins — detail row for a discipline='recovery' workout. A
-- SUBJECTIVE manual check-in (sleep/soreness/energy). Objective wearable sleep
-- lives in `bio_samples` (source='watch_import'); the two are surfaced
-- separately and never summed — the load view excludes both (recovery is a
-- recovery signal, not training load).
CREATE TABLE IF NOT EXISTS recovery_checkins (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id  INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    sleep_hours REAL,
    soreness    INTEGER CHECK(soreness IS NULL OR (soreness >= 1 AND soreness <= 10)),
    energy      INTEGER CHECK(energy IS NULL OR (energy >= 1 AND energy <= 10)),
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_recovery_checkins_workout
    ON recovery_checkins(workout_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_recovery_checkins_updated_at
AFTER UPDATE ON recovery_checkins FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE recovery_checkins SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down

-- Additive migration; reverse by dropping the new tables. Dropping
-- conditioning_sessions cascades to conditioning_intervals. Parent `workouts`
-- rows of these disciplines are left intact (they were never rebuilt here).
DROP TABLE IF EXISTS recovery_checkins;
DROP TABLE IF EXISTS skill_sessions;
DROP TABLE IF EXISTS conditioning_intervals;
DROP TABLE IF EXISTS conditioning_sessions;
