-- +goose Up

-- Barbell loading setup per athlete (their gym). The bar, the collars,
-- and each plate carry their own unit, separate from the logging unit in
-- user_preferences: an athlete can log in lbs on kg competition plates.
-- An athlete with no row uses the default kg competition setup.
CREATE TABLE plate_setups (
    athlete_id    INTEGER PRIMARY KEY REFERENCES athletes(id) ON DELETE CASCADE,
    bar_weight    REAL    NOT NULL CHECK (bar_weight >= 0),
    bar_unit      TEXT    NOT NULL CHECK (bar_unit IN ('kg', 'lbs')),
    collar_weight REAL    NOT NULL DEFAULT 0 CHECK (collar_weight >= 0),
    collar_unit   TEXT    NOT NULL DEFAULT 'kg' CHECK (collar_unit IN ('kg', 'lbs')),
    use_collars   INTEGER NOT NULL DEFAULT 0 CHECK (use_collars IN (0, 1)),
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Plate inventory: pairs is how many matched pairs of this plate the gym
-- has (one plate per sleeve per pair).
CREATE TABLE plate_setup_plates (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id INTEGER NOT NULL REFERENCES plate_setups(athlete_id) ON DELETE CASCADE,
    weight     REAL    NOT NULL CHECK (weight > 0),
    unit       TEXT    NOT NULL CHECK (unit IN ('kg', 'lbs')),
    pairs      INTEGER NOT NULL CHECK (pairs >= 0),
    UNIQUE (athlete_id, weight, unit)
);

CREATE INDEX idx_plate_setup_plates_athlete ON plate_setup_plates(athlete_id);

-- +goose StatementBegin
CREATE TRIGGER trigger_plate_setups_updated_at
AFTER UPDATE ON plate_setups FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE plate_setups SET updated_at = CURRENT_TIMESTAMP WHERE athlete_id = NEW.athlete_id;
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS trigger_plate_setups_updated_at;
DROP INDEX IF EXISTS idx_plate_setup_plates_athlete;
DROP TABLE IF EXISTS plate_setup_plates;
DROP TABLE IF EXISTS plate_setups;
