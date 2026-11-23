-- +goose Up

-- The program week and day a resistance session was, stored on the session
-- (ADR 028). Before this, the day was derived by counting the sessions linked
-- to the same assignment in date order, so a backfilled or out-of-order
-- session could only ever be "the next day". Both columns are NULL on rows
-- written before this migration, on unlinked sessions, and on non-resistance
-- disciplines; those keep the derived (counted) day. The app writes both or
-- neither.
ALTER TABLE workouts ADD COLUMN program_week INTEGER CHECK (program_week IS NULL OR program_week >= 1);
ALTER TABLE workouts ADD COLUMN program_day  INTEGER CHECK (program_day IS NULL OR program_day >= 1);

-- +goose Down

ALTER TABLE workouts DROP COLUMN program_day;
ALTER TABLE workouts DROP COLUMN program_week;
