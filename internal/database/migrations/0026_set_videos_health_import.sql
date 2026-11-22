-- +goose Up

-- Form-check videos (gap 12). One short clip per logged set. The file lives
-- on disk under REPLOG_VIDEO_DIR (never the public avatar directory) and is
-- streamed back only through the authenticated set route. file_name is a
-- random server-chosen name; the client's filename is never used. A coach
-- can leave one comment on the clip for the review. Deleting the set (or
-- its workout or athlete) cascades the row; the scheduler sweeps files no
-- row references.
CREATE TABLE workout_set_videos (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id        INTEGER NOT NULL UNIQUE REFERENCES workout_sets(id) ON DELETE CASCADE,
    file_name     TEXT    NOT NULL UNIQUE,
    content_type  TEXT    NOT NULL CHECK (content_type IN ('video/mp4', 'video/quicktime', 'video/webm')),
    size_bytes    INTEGER NOT NULL CHECK (size_bytes > 0),
    uploaded_by   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    coach_comment TEXT,
    commented_by  INTEGER REFERENCES users(id) ON DELETE SET NULL,
    commented_at  DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Health import (gap 14). Watch imports are idempotent per athlete, metric
-- and recorded_at: the importer writes recorded_at normalized to UTC
-- 'YYYY-MM-DD HH:MM:SS' and this index rejects a second copy. Manual rows
-- are left alone (they may already hold duplicates); the importer also
-- skips a sample when any row, manual or imported, has the same key.
CREATE UNIQUE INDEX idx_bio_samples_import_key
    ON bio_samples(athlete_id, metric, recorded_at)
    WHERE source = 'watch_import';

-- +goose Down

DROP INDEX IF EXISTS idx_bio_samples_import_key;
DROP TABLE IF EXISTS workout_set_videos;
