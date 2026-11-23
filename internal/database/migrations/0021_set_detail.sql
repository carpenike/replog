-- +goose Up

-- Prescription detail the coach import already stores on
-- program_block_exercises, now carried onto prescribed_sets so the
-- prescription read shows it: a rep range top (reps is the bottom),
-- an RIR range, warm-up (non-working) sets, and a superset/circuit
-- group key (A, B, ...). Hand-built templates can set the same columns.
ALTER TABLE prescribed_sets ADD COLUMN reps_max INTEGER
    CHECK (reps_max IS NULL OR reps_max >= 0);
ALTER TABLE prescribed_sets ADD COLUMN rir_min INTEGER
    CHECK (rir_min IS NULL OR (rir_min >= 0 AND rir_min <= 10));
ALTER TABLE prescribed_sets ADD COLUMN rir_max INTEGER
    CHECK (rir_max IS NULL OR (rir_max >= 0 AND rir_max <= 10));
ALTER TABLE prescribed_sets ADD COLUMN non_working INTEGER NOT NULL DEFAULT 0
    CHECK (non_working IN (0, 1));
ALTER TABLE prescribed_sets ADD COLUMN group_key TEXT;

-- Logged set type. Warm-ups are kept in the log but left out of volume,
-- PRs, e1RM, streaks, and set counts. Existing rows are working sets.
ALTER TABLE workout_sets ADD COLUMN set_type TEXT NOT NULL DEFAULT 'working'
    CHECK (set_type IN ('working', 'warmup', 'drop', 'failure'));

-- Superset/circuit group key on accessory plan lines.
ALTER TABLE accessory_plans ADD COLUMN group_key TEXT;

-- Re-derive the detail for sets already expanded from a coach import.
-- An imported day lists each exercise once (prescribed_sets is unique on
-- template, week, day, exercise, set), so the line matches on those keys.
-- Group keys letter the superset/circuit blocks of a day in block order.
UPDATE prescribed_sets
   SET reps_max    = src.reps_max,
       rir_min     = src.rir_min,
       rir_max     = src.rir_max,
       non_working = src.non_working,
       group_key   = src.group_key
  FROM (
        SELECT d.template_id, d.week, d.day_number, pbe.exercise_id,
               CASE WHEN pbe.reps_max IS NOT NULL
                         AND pbe.reps_max > COALESCE(pbe.reps_min, pbe.reps_max)
                    THEN pbe.reps_max END AS reps_max,
               pbe.rir_min, pbe.rir_max, pbe.non_working,
               g.group_key
          FROM program_block_exercises pbe
          JOIN program_blocks b ON b.id = pbe.block_id
          JOIN program_days d ON d.id = b.day_id
          LEFT JOIN (
                SELECT id,
                       char(64 + ROW_NUMBER() OVER (PARTITION BY day_id ORDER BY block_order)) AS group_key
                  FROM program_blocks
                 WHERE block_type IN ('superset', 'circuit')
          ) g ON g.id = b.id
       ) AS src
 WHERE prescribed_sets.template_id = src.template_id
   AND prescribed_sets.week = src.week
   AND prescribed_sets.day = src.day_number
   AND prescribed_sets.exercise_id = src.exercise_id;

-- +goose Down

ALTER TABLE accessory_plans DROP COLUMN group_key;
ALTER TABLE workout_sets DROP COLUMN set_type;
ALTER TABLE prescribed_sets DROP COLUMN group_key;
ALTER TABLE prescribed_sets DROP COLUMN non_working;
ALTER TABLE prescribed_sets DROP COLUMN rir_max;
ALTER TABLE prescribed_sets DROP COLUMN rir_min;
ALTER TABLE prescribed_sets DROP COLUMN reps_max;
