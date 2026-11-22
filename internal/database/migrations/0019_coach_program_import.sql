-- +goose Up

-- Coach-authored program import (ADR 024).
-- A service token stores a locked template. Re-importing the same name
-- adds a version; it does not update the previous row. Approved names
-- stay unique per version, so version 2 of a name can sit beside version 1.

DROP INDEX IF EXISTS idx_program_templates_name_global;
DROP INDEX IF EXISTS idx_program_templates_name_athlete;

CREATE UNIQUE INDEX idx_program_templates_name_global
    ON program_templates(name, version) WHERE athlete_id IS NULL AND status = 'approved';

CREATE UNIQUE INDEX idx_program_templates_name_athlete
    ON program_templates(athlete_id, name, version) WHERE athlete_id IS NOT NULL AND status = 'approved';

-- Catalog fields the coach import can set. Existing reads keep their
-- column lists; unilateral defaults to 0 for rows already in the table.
ALTER TABLE exercises ADD COLUMN unilateral INTEGER NOT NULL DEFAULT 0
    CHECK (unilateral IN (0, 1));

CREATE TABLE exercise_aliases (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    alias       TEXT    NOT NULL COLLATE NOCASE,
    UNIQUE (alias)
);

CREATE INDEX idx_exercise_aliases_exercise ON exercise_aliases(exercise_id);

CREATE TABLE exercise_muscles (
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    muscle      TEXT    NOT NULL COLLATE NOCASE,
    PRIMARY KEY (exercise_id, muscle)
);

-- One row per imported template. locked = 1 means coach-set fields do
-- not change in place. author is the outside coach (no RepLog login).
CREATE TABLE program_imports (
    template_id       INTEGER PRIMARY KEY REFERENCES program_templates(id) ON DELETE CASCADE,
    source_name       TEXT    NOT NULL,
    author            TEXT    NOT NULL,
    repeat_cycle      TEXT,
    schedule_weekdays TEXT,
    upcoming_count    INTEGER,
    locked            INTEGER NOT NULL DEFAULT 1 CHECK (locked IN (0, 1)),
    start_date        DATE
);

CREATE TABLE program_days (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id        INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    week               INTEGER NOT NULL,
    day_number         INTEGER NOT NULL,
    day_order          INTEGER NOT NULL,
    label              TEXT    NOT NULL,
    weekday            INTEGER,
    kind               TEXT,
    estimated_minutes  INTEGER,
    exercise_count     INTEGER NOT NULL,
    equipment_json     TEXT    NOT NULL,
    instructions       TEXT,
    UNIQUE (template_id, week, day_number),
    UNIQUE (template_id, week, day_order)
);

CREATE INDEX idx_program_days_template ON program_days(template_id, week, day_number);

CREATE TABLE program_blocks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    day_id        INTEGER NOT NULL REFERENCES program_days(id) ON DELETE CASCADE,
    block_order   INTEGER NOT NULL,
    block_type    TEXT    NOT NULL CHECK (block_type IN ('straight', 'superset', 'circuit')),
    rounds        INTEGER,
    sets          INTEGER,
    rest_seconds  INTEGER,
    rest_text     TEXT,
    non_working   INTEGER NOT NULL DEFAULT 0 CHECK (non_working IN (0, 1)),
    UNIQUE (day_id, block_order)
);

CREATE INDEX idx_program_blocks_day ON program_blocks(day_id, block_order);

-- Coach columns are the imported prescription. ai_* is a later suggestion
-- for a field the coach left blank. Gap-fill updates only ai_*.
CREATE TABLE program_block_exercises (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id         INTEGER NOT NULL REFERENCES program_blocks(id) ON DELETE CASCADE,
    exercise_id      INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    sort_order       INTEGER NOT NULL,
    sets             INTEGER,
    reps_min         INTEGER,
    reps_max         INTEGER,
    rir_min          INTEGER,
    rir_max          INTEGER,
    per_side         INTEGER NOT NULL DEFAULT 0 CHECK (per_side IN (0, 1)),
    rest_seconds     INTEGER,
    rest_text        TEXT,
    notes            TEXT,
    non_working      INTEGER NOT NULL DEFAULT 0 CHECK (non_working IN (0, 1)),
    ai_sets          INTEGER,
    ai_reps_min      INTEGER,
    ai_reps_max      INTEGER,
    ai_rir_min       INTEGER,
    ai_rir_max       INTEGER,
    ai_rest_seconds  INTEGER,
    ai_notes         TEXT,
    ai_load          REAL,
    UNIQUE (block_id, sort_order)
);

CREATE INDEX idx_program_block_exercises_block ON program_block_exercises(block_id, sort_order);

-- +goose Down

DROP INDEX IF EXISTS idx_program_block_exercises_block;
DROP TABLE IF EXISTS program_block_exercises;

DROP INDEX IF EXISTS idx_program_blocks_day;
DROP TABLE IF EXISTS program_blocks;

DROP INDEX IF EXISTS idx_program_days_template;
DROP TABLE IF EXISTS program_days;

DROP TABLE IF EXISTS program_imports;

DROP TABLE IF EXISTS exercise_muscles;
DROP INDEX IF EXISTS idx_exercise_aliases_exercise;
DROP TABLE IF EXISTS exercise_aliases;

ALTER TABLE exercises DROP COLUMN unilateral;

DROP INDEX IF EXISTS idx_program_templates_name_global;
DROP INDEX IF EXISTS idx_program_templates_name_athlete;

CREATE UNIQUE INDEX idx_program_templates_name_global
    ON program_templates(name) WHERE athlete_id IS NULL AND status = 'approved';

CREATE UNIQUE INDEX idx_program_templates_name_athlete
    ON program_templates(athlete_id, name) WHERE athlete_id IS NOT NULL AND status = 'approved';
