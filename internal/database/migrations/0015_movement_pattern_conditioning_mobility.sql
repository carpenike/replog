-- +goose Up
-- ---------------------------------------------------------------------------
-- Issue #33: widen the movement-pattern vocabulary with 'conditioning' and
-- 'mobility' so conditioning/locomotion implements (battle ropes, sprints,
-- shuffles) and cool-down mobility work can be tagged and enter a
-- methodology's pattern-scoped exercise catalog. Without a tag these
-- exercises are invisible to generation even though the sarge-circuit
-- methodology definition explicitly calls for them.
--
-- SQLite can't ALTER a CHECK constraint, so both pattern tables are rebuilt
-- (same recipe as the 0008 throwing_sessions rebuild). Both are leaf link
-- tables: outbound FKs only, nothing references them, no triggers. Every
-- existing row survives unchanged; only the set of *future* legal pattern
-- values grows — additive in spirit (ADR 002).
PRAGMA defer_foreign_keys = ON;

-- 1. exercise_movement_patterns: identical shape, CHECK widened.
CREATE TABLE exercise_movement_patterns_new (
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    pattern     TEXT    NOT NULL CHECK(pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground', 'conditioning', 'mobility')),
    PRIMARY KEY (exercise_id, pattern)
);

INSERT INTO exercise_movement_patterns_new (exercise_id, pattern)
SELECT exercise_id, pattern FROM exercise_movement_patterns;

DROP TABLE exercise_movement_patterns;
ALTER TABLE exercise_movement_patterns_new RENAME TO exercise_movement_patterns;

CREATE INDEX IF NOT EXISTS idx_exercise_movement_patterns_pattern
    ON exercise_movement_patterns(pattern);

-- 2. methodology_allowed_patterns: identical shape, CHECK widened.
CREATE TABLE methodology_allowed_patterns_new (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    pattern        TEXT    NOT NULL CHECK(pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground', 'conditioning', 'mobility')),
    PRIMARY KEY (methodology_id, pattern)
);

INSERT INTO methodology_allowed_patterns_new (methodology_id, pattern)
SELECT methodology_id, pattern FROM methodology_allowed_patterns;

DROP TABLE methodology_allowed_patterns;
ALTER TABLE methodology_allowed_patterns_new RENAME TO methodology_allowed_patterns;

-- +goose Down
-- ---------------------------------------------------------------------------
-- Reverse: narrow the CHECK back to the six Dan John tags. Rows carrying the
-- new values have no representation in the old vocabulary, so they are
-- dropped on the way down (best-effort; the exercises themselves survive,
-- they just return to untagged).
PRAGMA defer_foreign_keys = ON;

CREATE TABLE exercise_movement_patterns_old (
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    pattern     TEXT    NOT NULL CHECK(pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground')),
    PRIMARY KEY (exercise_id, pattern)
);

INSERT INTO exercise_movement_patterns_old (exercise_id, pattern)
SELECT exercise_id, pattern FROM exercise_movement_patterns
WHERE pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground');

DROP TABLE exercise_movement_patterns;
ALTER TABLE exercise_movement_patterns_old RENAME TO exercise_movement_patterns;

CREATE INDEX IF NOT EXISTS idx_exercise_movement_patterns_pattern
    ON exercise_movement_patterns(pattern);

CREATE TABLE methodology_allowed_patterns_old (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    pattern        TEXT    NOT NULL CHECK(pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground')),
    PRIMARY KEY (methodology_id, pattern)
);

INSERT INTO methodology_allowed_patterns_old (methodology_id, pattern)
SELECT methodology_id, pattern FROM methodology_allowed_patterns
WHERE pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground');

DROP TABLE methodology_allowed_patterns;
ALTER TABLE methodology_allowed_patterns_old RENAME TO methodology_allowed_patterns;
