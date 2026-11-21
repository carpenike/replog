-- +goose Up

-- Coach service token (phase 6). One long-lived bearer for the coach
-- account the homelab connector uses. The plaintext is never stored:
-- token_hash is the lowercase-hex SHA-256 of the full presented secret,
-- including the rlcst_ prefix, matching login_tokens and mcp_tokens.
--
-- There is no role column. The row only identifies the user. Handlers
-- refuse this credential on program assign, generation execute,
-- training-max writes, and promote. A browser session of the same user
-- is a different credential and can still approve.

CREATE TABLE service_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash   TEXT NOT NULL UNIQUE,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label        TEXT,
    revoked_at   TIMESTAMP,
    last_used_at TIMESTAMP,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_service_tokens_user ON service_tokens(user_id);

-- +goose Down

DROP INDEX IF EXISTS idx_service_tokens_user;
DROP TABLE IF EXISTS service_tokens;
