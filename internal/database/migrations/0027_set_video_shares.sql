-- +goose Up

-- Form-check video share links. A person with access to a set's clip (the
-- athlete, their coach or an admin) can mint a link that lets someone with
-- no login watch that ONE clip until the link expires. Only the SHA-256 hash
-- of the random token is stored; the token itself is shown once, when the
-- link is created, and is never persisted or logged. Expiry is chosen at
-- creation (24 h, 7 d or 30 d; the API rejects anything else). Links are
-- revoked one at a time (revoked_at); deleting or replacing the clip
-- removes its row, and this cascade removes every link to it at once.
CREATE TABLE set_video_shares (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id         INTEGER NOT NULL REFERENCES workout_set_videos(id) ON DELETE CASCADE,
    token_hash       TEXT    NOT NULL UNIQUE CHECK (length(token_hash) = 64),
    label            TEXT    CHECK (label IS NULL OR length(label) <= 100),
    created_by       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       DATETIME NOT NULL,
    revoked_at       DATETIME,
    revoked_by       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    last_accessed_at DATETIME,
    access_count     INTEGER NOT NULL DEFAULT 0 CHECK (access_count >= 0)
);

CREATE INDEX idx_set_video_shares_video ON set_video_shares(video_id);

-- +goose Down

DROP INDEX IF EXISTS idx_set_video_shares_video;
DROP TABLE IF EXISTS set_video_shares;
