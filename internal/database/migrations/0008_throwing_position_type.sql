-- +goose Up
-- ---------------------------------------------------------------------------
-- HOF-010 / #28: make position/infield throwing first-class.
--
-- Widen throwing_sessions.throw_type's CHECK to add 'position'. SQLite can't
-- ALTER a CHECK constraint, so we rebuild the table. throwing_sessions is a
-- leaf: its only FK is OUTBOUND (workout_id -> workouts), and grep confirms
-- nothing references throwing_sessions(id). So this rebuild is a strict subset
-- of the 0006 workouts rebuild — no inbound-FK juggling.
--
-- We use `PRAGMA defer_foreign_keys=ON`, which is transaction-safe: it defers
-- FK enforcement to COMMIT, at which point the outbound workout_id FK is
-- checked once. We preserve every row id (and workout_id) verbatim, so the
-- check passes; if a row were orphaned, goose's COMMIT would fail and roll the
-- whole migration back. Additive in spirit — every existing row survives
-- unchanged; only the set of *future* legal throw_type values grows.
PRAGMA defer_foreign_keys = ON;

-- 1. New shape: identical to 0006's throwing_sessions, CHECK widened to add
--    'position'.
CREATE TABLE throwing_sessions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id  INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    throw_type  TEXT    NOT NULL
                CHECK(throw_type IN ('game','bullpen','lesson','long_toss','catch','flat_ground','position')),
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

-- 2. Copy every row, preserving ids and all column values.
INSERT INTO throwing_sessions_new
    (id, workout_id, throw_type, throw_count, max_intent, velocity,
     fatigue, pain, source, team, notes, created_at, updated_at)
SELECT id, workout_id, throw_type, throw_count, max_intent, velocity,
       fatigue, pain, source, team, notes, created_at, updated_at
FROM throwing_sessions;

-- 3. Swap. Dropping the old table also drops its index + trigger; recreate.
DROP TABLE throwing_sessions;
ALTER TABLE throwing_sessions_new RENAME TO throwing_sessions;

-- 4. Recreate the index and updated_at trigger that lived on the old table.
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

-- +goose Down
-- ---------------------------------------------------------------------------
-- Reverse: narrow the CHECK back to the 0006 set. Any 'position' rows are
-- remapped to 'catch' on the way down so the narrowed CHECK can't reject them
-- (down-migration is best-effort; 'position' is the closest non-pitching peer).
PRAGMA defer_foreign_keys = ON;

CREATE TABLE throwing_sessions_old (
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

INSERT INTO throwing_sessions_old
    (id, workout_id, throw_type, throw_count, max_intent, velocity,
     fatigue, pain, source, team, notes, created_at, updated_at)
SELECT id, workout_id,
       CASE WHEN throw_type = 'position' THEN 'catch' ELSE throw_type END,
       throw_count, max_intent, velocity,
       fatigue, pain, source, team, notes, created_at, updated_at
FROM throwing_sessions;

DROP TABLE throwing_sessions;
ALTER TABLE throwing_sessions_old RENAME TO throwing_sessions;

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
