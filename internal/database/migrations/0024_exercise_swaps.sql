-- +goose Up

-- Athlete exercise swaps (gap 11): during a session an athlete can swap a
-- prescribed exercise for one with the same movement pattern. The swap is
-- logged on the workout with an optional reason so the coach sees it in
-- review. The program itself is never changed.
CREATE TABLE workout_exercise_swaps (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id       INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    from_exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    to_exercise_id   INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    reason           TEXT,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (from_exercise_id <> to_exercise_id),
    UNIQUE (workout_id, from_exercise_id)
);

CREATE INDEX idx_workout_exercise_swaps_workout ON workout_exercise_swaps(workout_id);

-- +goose Down

DROP INDEX IF EXISTS idx_workout_exercise_swaps_workout;
DROP TABLE IF EXISTS workout_exercise_swaps;
