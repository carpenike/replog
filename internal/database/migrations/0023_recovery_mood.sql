-- +goose Up

-- Readiness check before a session (gap 10): the session screen asks
-- sleep, soreness, and mood and saves the answers as a recovery check-in.
-- Mood is a new 1-10 answer next to soreness and energy. A low answer
-- flags the session for the coach in review; it never changes the plan.
ALTER TABLE recovery_checkins ADD COLUMN mood INTEGER
    CHECK (mood IS NULL OR (mood >= 1 AND mood <= 10));

-- +goose Down

ALTER TABLE recovery_checkins DROP COLUMN mood;
