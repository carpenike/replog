-- +goose Up

-- Audit log for privileged identity actions (impersonation start/stop, etc.).
--
-- Persists a durable, append-only record of who did what to whom. The first
-- consumer is the impersonation flow: when an admin/coach starts or stops
-- impersonating another user we write a row here in addition to the existing
-- log.Printf, so the action survives log rotation and is queryable.
--
--   * real_user_id  — the acting (real) identity, always present.
--   * target_user_id — the user acted upon (e.g. the impersonated user); NULL
--     when an action has no distinct target.
--   * action        — a short machine-readable verb, e.g. 'impersonate_start'.
--   * details       — optional free-form context (usernames, ids) for humans.
--
-- Additive only (ADR 002). Writes are best-effort at the call site: a failure
-- to audit must never fail the underlying request.

CREATE TABLE audit_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    real_user_id   INTEGER NOT NULL,
    target_user_id INTEGER,
    action         TEXT NOT NULL,
    details        TEXT,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);

-- +goose Down

DROP INDEX IF EXISTS idx_audit_log_created_at;
DROP TABLE IF EXISTS audit_log;
