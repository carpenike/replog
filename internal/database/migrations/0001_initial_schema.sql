-- +goose Up

CREATE TABLE IF NOT EXISTS athletes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL COLLATE NOCASE,
    tier        TEXT    CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')),
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    email           TEXT    UNIQUE COLLATE NOCASE,
    password_hash   TEXT    NOT NULL,
    athlete_id      INTEGER REFERENCES athletes(id) ON DELETE SET NULL,
    is_coach        INTEGER NOT NULL DEFAULT 0 CHECK(is_coach IN (0, 1)),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    tier        TEXT    CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')),
    target_reps INTEGER,
    form_notes  TEXT,
    demo_url    TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS athlete_exercises (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    active          INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    assigned_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deactivated_at  DATETIME
);

CREATE TABLE IF NOT EXISTS training_maxes (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    weight          REAL    NOT NULL,
    effective_date  DATE    NOT NULL,
    notes           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, exercise_id, effective_date)
);

CREATE TABLE IF NOT EXISTS workouts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    date        DATE    NOT NULL,
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, date)
);

CREATE TABLE IF NOT EXISTS workout_sets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id  INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    set_number  INTEGER NOT NULL,
    reps        INTEGER NOT NULL,
    weight      REAL,
    rpe         REAL    CHECK(rpe >= 1 AND rpe <= 10),
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workout_id, exercise_id, set_number)
);

-- Indexes for common query patterns
CREATE UNIQUE INDEX IF NOT EXISTS idx_athlete_exercises_unique_active
    ON athlete_exercises(athlete_id, exercise_id) WHERE active = 1;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_unique_athlete_id
    ON users(athlete_id) WHERE athlete_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_athlete_exercises_athlete_id
    ON athlete_exercises(athlete_id);

CREATE INDEX IF NOT EXISTS idx_workout_sets_workout
    ON workout_sets(workout_id);

CREATE TABLE IF NOT EXISTS body_weights (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    date        DATE    NOT NULL,
    weight      REAL    NOT NULL,
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, date)
);

CREATE INDEX IF NOT EXISTS idx_body_weights_athlete_date
    ON body_weights(athlete_id, date DESC);

-- Session store for alexedwards/scs
CREATE TABLE IF NOT EXISTS sessions (
    token  TEXT PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expiry);

-- Triggers for updated_at timestamps
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_users_updated_at
AFTER UPDATE ON users FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_athletes_updated_at
AFTER UPDATE ON athletes FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE athletes SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_exercises_updated_at
AFTER UPDATE ON exercises FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE exercises SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_workouts_updated_at
AFTER UPDATE ON workouts FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workouts SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_workout_sets_updated_at
AFTER UPDATE ON workout_sets FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workout_sets SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS trigger_workout_sets_updated_at;
DROP TRIGGER IF EXISTS trigger_workouts_updated_at;
DROP TRIGGER IF EXISTS trigger_exercises_updated_at;
DROP TRIGGER IF EXISTS trigger_athletes_updated_at;
DROP TRIGGER IF EXISTS trigger_users_updated_at;

DROP INDEX IF EXISTS idx_sessions_expiry;
DROP INDEX IF EXISTS idx_body_weights_athlete_date;
DROP INDEX IF EXISTS idx_workout_sets_workout;
DROP INDEX IF EXISTS idx_athlete_exercises_athlete_id;
DROP INDEX IF EXISTS idx_users_unique_athlete_id;
DROP INDEX IF EXISTS idx_athlete_exercises_unique_active;

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS body_weights;
DROP TABLE IF EXISTS workout_sets;
DROP TABLE IF EXISTS workouts;
DROP TABLE IF EXISTS training_maxes;
DROP TABLE IF EXISTS athlete_exercises;
DROP TABLE IF EXISTS exercises;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS athletes;
