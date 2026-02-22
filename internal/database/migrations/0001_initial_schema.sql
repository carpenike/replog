-- +goose Up

CREATE TABLE IF NOT EXISTS athletes (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT    NOT NULL COLLATE NOCASE,
    tier                TEXT    CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')),
    notes               TEXT,
    goal                TEXT,
    date_of_birth       DATE,
    grade               TEXT,
    gender              TEXT    CHECK(gender IN ('male', 'female')),
    coach_id            INTEGER REFERENCES users(id) ON DELETE SET NULL,
    track_body_weight   INTEGER NOT NULL DEFAULT 1 CHECK(track_body_weight IN (0, 1)),
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name            TEXT,
    email           TEXT    UNIQUE COLLATE NOCASE,
    password_hash   TEXT,
    athlete_id      INTEGER REFERENCES athletes(id) ON DELETE SET NULL,
    is_coach        INTEGER NOT NULL DEFAULT 0 CHECK(is_coach IN (0, 1)),
    is_admin        INTEGER NOT NULL DEFAULT 0 CHECK(is_admin IN (0, 1)),
    avatar_path     TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_preferences (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    weight_unit TEXT    NOT NULL DEFAULT 'lbs' CHECK(weight_unit IN ('lbs', 'kg')),
    timezone    TEXT    NOT NULL DEFAULT 'America/New_York',
    date_format TEXT    NOT NULL DEFAULT 'Jan 2, 2006',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    tier         TEXT    CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')),
    form_notes   TEXT,
    demo_url     TEXT,
    rest_seconds INTEGER,
    featured     INTEGER NOT NULL DEFAULT 0 CHECK(featured IN (0, 1)),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS athlete_exercises (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    target_reps     INTEGER,
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
    rep_type    TEXT    NOT NULL DEFAULT 'reps' CHECK(rep_type IN ('reps', 'each_side', 'seconds', 'distance')),
    weight      REAL,
    rpe         REAL    CHECK(rpe >= 1 AND rpe <= 10),
    category    TEXT    NOT NULL DEFAULT 'main' CHECK(category IN ('main', 'supplemental', 'accessory')),
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

CREATE INDEX IF NOT EXISTS idx_user_preferences_user_id
    ON user_preferences(user_id);

CREATE INDEX IF NOT EXISTS idx_athlete_exercises_athlete_id
    ON athlete_exercises(athlete_id);

CREATE INDEX IF NOT EXISTS idx_athletes_coach_id
    ON athletes(coach_id);

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

CREATE TABLE IF NOT EXISTS goal_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    goal            TEXT    NOT NULL,
    previous_goal   TEXT,
    set_by          INTEGER REFERENCES users(id) ON DELETE SET NULL,
    effective_date  DATE    NOT NULL DEFAULT (date('now')),
    notes           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_goal_history_athlete_date
    ON goal_history(athlete_id, effective_date DESC, created_at DESC);

CREATE TABLE IF NOT EXISTS tier_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    tier            TEXT    NOT NULL CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')),
    previous_tier   TEXT    CHECK(previous_tier IN ('foundational', 'intermediate', 'sport_performance')),
    set_by          INTEGER REFERENCES users(id) ON DELETE SET NULL,
    effective_date  DATE    NOT NULL DEFAULT (date('now')),
    notes           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tier_history_athlete_date
    ON tier_history(athlete_id, effective_date DESC, created_at DESC);

CREATE TABLE IF NOT EXISTS athlete_notes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    author_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    date        DATE    NOT NULL DEFAULT (date('now')),
    content     TEXT    NOT NULL,
    is_private  INTEGER NOT NULL DEFAULT 0 CHECK(is_private IN (0, 1)),
    pinned      INTEGER NOT NULL DEFAULT 0 CHECK(pinned IN (0, 1)),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_athlete_notes_athlete_date
    ON athlete_notes(athlete_id, date DESC, created_at DESC);

CREATE TABLE IF NOT EXISTS workout_reviews (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id  INTEGER NOT NULL UNIQUE REFERENCES workouts(id) ON DELETE CASCADE,
    coach_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    status      TEXT    NOT NULL CHECK(status IN ('approved', 'needs_work')),
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_workout_reviews_workout_id
    ON workout_reviews(workout_id);

CREATE INDEX IF NOT EXISTS idx_workout_reviews_status
    ON workout_reviews(status);

CREATE TABLE IF NOT EXISTS program_templates (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER REFERENCES athletes(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL COLLATE NOCASE,
    description TEXT,
    num_weeks   INTEGER NOT NULL DEFAULT 1,
    num_days    INTEGER NOT NULL DEFAULT 1,
    is_loop     INTEGER NOT NULL DEFAULT 0 CHECK(is_loop IN (0, 1)),
    audience    TEXT CHECK(audience IN ('youth', 'adult')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Global templates (athlete_id IS NULL) must have unique names.
CREATE UNIQUE INDEX IF NOT EXISTS idx_program_templates_name_global
    ON program_templates(name) WHERE athlete_id IS NULL;

-- Per-athlete templates must have unique names within that athlete.
CREATE UNIQUE INDEX IF NOT EXISTS idx_program_templates_name_athlete
    ON program_templates(athlete_id, name) WHERE athlete_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_program_templates_athlete
    ON program_templates(athlete_id);

CREATE TABLE IF NOT EXISTS prescribed_sets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id     INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    week            INTEGER NOT NULL,
    day             INTEGER NOT NULL,
    set_number      INTEGER NOT NULL,
    reps            INTEGER,
    rep_type        TEXT    NOT NULL DEFAULT 'reps' CHECK(rep_type IN ('reps', 'each_side', 'seconds', 'distance')),
    percentage      REAL,
    absolute_weight REAL,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    notes           TEXT,
    UNIQUE(template_id, week, day, exercise_id, set_number)
);

CREATE INDEX IF NOT EXISTS idx_prescribed_sets_template
    ON prescribed_sets(template_id, week, day);

CREATE TABLE IF NOT EXISTS athlete_programs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    template_id INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE RESTRICT,
    start_date  DATE    NOT NULL,
    active      INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    notes       TEXT,
    goal        TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_athlete_programs_active
    ON athlete_programs(athlete_id) WHERE active = 1;

-- Per-exercise TM increment rules for a program template.
-- Defines how much to suggest bumping the training max after a cycle completes.
CREATE TABLE IF NOT EXISTS progression_rules (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id  INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    exercise_id  INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    increment    REAL    NOT NULL,
    UNIQUE(template_id, exercise_id)
);

CREATE INDEX IF NOT EXISTS idx_progression_rules_template
    ON progression_rules(template_id);

CREATE TABLE IF NOT EXISTS login_tokens (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       TEXT    NOT NULL UNIQUE,
    label       TEXT,
    expires_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_login_tokens_token ON login_tokens(token);
CREATE INDEX IF NOT EXISTS idx_login_tokens_user_id ON login_tokens(user_id);

CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id   BLOB    NOT NULL UNIQUE,
    public_key      BLOB    NOT NULL,
    attestation_type TEXT   NOT NULL DEFAULT '',
    transport       TEXT,
    sign_count      INTEGER NOT NULL DEFAULT 0,
    clone_warning   INTEGER NOT NULL DEFAULT 0 CHECK(clone_warning IN (0, 1)),
    attachment      TEXT    NOT NULL DEFAULT '',
    aaguid          BLOB,
    flags_user_present    INTEGER NOT NULL DEFAULT 0 CHECK(flags_user_present IN (0, 1)),
    flags_user_verified   INTEGER NOT NULL DEFAULT 0 CHECK(flags_user_verified IN (0, 1)),
    flags_backup_eligible INTEGER NOT NULL DEFAULT 0 CHECK(flags_backup_eligible IN (0, 1)),
    flags_backup_state    INTEGER NOT NULL DEFAULT 0 CHECK(flags_backup_state IN (0, 1)),
    label           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user_id ON webauthn_credentials(user_id);

-- Equipment catalog — shared list of equipment types (e.g., "Barbell", "Squat Rack").
CREATE TABLE IF NOT EXISTS equipment (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Equipment required by an exercise. Many-to-many join table.
-- optional = 0 means required, optional = 1 means nice-to-have.
CREATE TABLE IF NOT EXISTS exercise_equipment (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    exercise_id  INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    equipment_id INTEGER NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    optional     INTEGER NOT NULL DEFAULT 0 CHECK(optional IN (0, 1)),
    UNIQUE(exercise_id, equipment_id)
);

CREATE INDEX IF NOT EXISTS idx_exercise_equipment_exercise
    ON exercise_equipment(exercise_id);

CREATE INDEX IF NOT EXISTS idx_exercise_equipment_equipment
    ON exercise_equipment(equipment_id);

-- Equipment available to an athlete. Many-to-many join table.
CREATE TABLE IF NOT EXISTS athlete_equipment (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id   INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    equipment_id INTEGER NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    UNIQUE(athlete_id, equipment_id)
);

CREATE INDEX IF NOT EXISTS idx_athlete_equipment_athlete
    ON athlete_equipment(athlete_id);

CREATE INDEX IF NOT EXISTS idx_athlete_equipment_equipment
    ON athlete_equipment(equipment_id);

-- Accessory plans: per-athlete per-day accessory prescriptions, decoupled from program templates.
CREATE TABLE IF NOT EXISTS accessory_plans (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    day             INTEGER NOT NULL,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    target_sets     INTEGER,
    target_rep_min  INTEGER,
    target_rep_max  INTEGER,
    target_weight   REAL,
    notes           TEXT,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    active          INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, day, exercise_id)
);

CREATE INDEX IF NOT EXISTS idx_accessory_plans_athlete_day
    ON accessory_plans(athlete_id, day) WHERE active = 1;

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
CREATE TRIGGER IF NOT EXISTS trigger_user_preferences_updated_at
AFTER UPDATE ON user_preferences FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE user_preferences SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
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

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_program_templates_updated_at
AFTER UPDATE ON program_templates FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE program_templates SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_athlete_programs_updated_at
AFTER UPDATE ON athlete_programs FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE athlete_programs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_workout_reviews_updated_at
AFTER UPDATE ON workout_reviews FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workout_reviews SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_equipment_updated_at
AFTER UPDATE ON equipment FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE equipment SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_athlete_notes_updated_at
AFTER UPDATE ON athlete_notes FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE athlete_notes SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_accessory_plans_updated_at
AFTER UPDATE ON accessory_plans FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE accessory_plans SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- Notifications — in-app notifications for users.
CREATE TABLE IF NOT EXISTS notifications (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT    NOT NULL,
    title       TEXT    NOT NULL,
    message     TEXT,
    link        TEXT,
    read        INTEGER NOT NULL DEFAULT 0 CHECK(read IN (0, 1)),
    athlete_id  INTEGER REFERENCES athletes(id) ON DELETE CASCADE,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications(user_id, read) WHERE read = 0;
CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications(user_id, created_at DESC);

-- Notification preferences — per-user, per-type opt-in/out for channels.
CREATE TABLE IF NOT EXISTS notification_preferences (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type     TEXT    NOT NULL,
    in_app   INTEGER NOT NULL DEFAULT 1 CHECK(in_app IN (0, 1)),
    external INTEGER NOT NULL DEFAULT 0 CHECK(external IN (0, 1)),
    UNIQUE(user_id, type)
);

CREATE INDEX IF NOT EXISTS idx_notification_preferences_user
    ON notification_preferences(user_id);

-- Application settings — key-value store for runtime configuration.
-- Supports the resolution chain: environment variable → app_settings row → built-in default.
-- Sensitive values (API keys) are stored encrypted (prefixed with "enc:").
CREATE TABLE IF NOT EXISTS app_settings (
    key   TEXT PRIMARY KEY NOT NULL,
    value TEXT NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS app_settings;
DROP INDEX IF EXISTS idx_notification_preferences_user;
DROP TABLE IF EXISTS notification_preferences;
DROP INDEX IF EXISTS idx_notifications_user_created;
DROP INDEX IF EXISTS idx_notifications_user_unread;
DROP TABLE IF EXISTS notifications;

DROP TRIGGER IF EXISTS trigger_accessory_plans_updated_at;
DROP TRIGGER IF EXISTS trigger_equipment_updated_at;
DROP TRIGGER IF EXISTS trigger_workout_reviews_updated_at;
DROP TRIGGER IF EXISTS trigger_athlete_programs_updated_at;
DROP TRIGGER IF EXISTS trigger_program_templates_updated_at;

DROP TRIGGER IF EXISTS trigger_workout_sets_updated_at;
DROP TRIGGER IF EXISTS trigger_workouts_updated_at;
DROP TRIGGER IF EXISTS trigger_exercises_updated_at;
DROP TRIGGER IF EXISTS trigger_athletes_updated_at;
DROP TRIGGER IF EXISTS trigger_user_preferences_updated_at;
DROP TRIGGER IF EXISTS trigger_users_updated_at;

DROP INDEX IF EXISTS idx_sessions_expiry;
DROP INDEX IF EXISTS idx_athlete_equipment_equipment;
DROP INDEX IF EXISTS idx_athlete_equipment_athlete;
DROP INDEX IF EXISTS idx_exercise_equipment_equipment;
DROP INDEX IF EXISTS idx_exercise_equipment_exercise;
DROP INDEX IF EXISTS idx_webauthn_credentials_user_id;
DROP INDEX IF EXISTS idx_login_tokens_user_id;
DROP INDEX IF EXISTS idx_login_tokens_token;
DROP INDEX IF EXISTS idx_workout_reviews_status;
DROP INDEX IF EXISTS idx_workout_reviews_workout_id;
DROP INDEX IF EXISTS idx_user_preferences_user_id;
DROP INDEX IF EXISTS idx_progression_rules_template;
DROP INDEX IF EXISTS idx_program_templates_athlete;
DROP INDEX IF EXISTS idx_program_templates_name_athlete;
DROP INDEX IF EXISTS idx_program_templates_name_global;
DROP INDEX IF EXISTS idx_athlete_programs_active;
DROP INDEX IF EXISTS idx_prescribed_sets_template;
DROP TRIGGER IF EXISTS trigger_athlete_notes_updated_at;

DROP INDEX IF EXISTS idx_athlete_notes_athlete_date;
DROP INDEX IF EXISTS idx_tier_history_athlete_date;
DROP INDEX IF EXISTS idx_goal_history_athlete_date;
DROP INDEX IF EXISTS idx_body_weights_athlete_date;
DROP INDEX IF EXISTS idx_workout_sets_workout;
DROP INDEX IF EXISTS idx_athlete_exercises_athlete_id;
DROP INDEX IF EXISTS idx_users_unique_athlete_id;
DROP INDEX IF EXISTS idx_athlete_exercises_unique_active;

DROP TABLE IF EXISTS accessory_plans;
DROP INDEX IF EXISTS idx_accessory_plans_athlete_day;
DROP TABLE IF EXISTS athlete_equipment;
DROP TABLE IF EXISTS exercise_equipment;
DROP TABLE IF EXISTS equipment;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS webauthn_credentials;
DROP TABLE IF EXISTS login_tokens;
DROP TABLE IF EXISTS workout_reviews;
DROP TABLE IF EXISTS progression_rules;
DROP TABLE IF EXISTS athlete_programs;
DROP TABLE IF EXISTS prescribed_sets;
DROP TABLE IF EXISTS program_templates;
DROP TABLE IF EXISTS athlete_notes;
DROP TABLE IF EXISTS tier_history;
DROP TABLE IF EXISTS goal_history;
DROP TABLE IF EXISTS body_weights;
DROP TABLE IF EXISTS workout_sets;
DROP TABLE IF EXISTS workouts;
DROP TABLE IF EXISTS training_maxes;
DROP TABLE IF EXISTS athlete_exercises;
DROP TABLE IF EXISTS exercises;
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS athletes;
