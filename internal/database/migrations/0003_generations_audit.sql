-- +goose Up

-- Persist the assembled athlete context + the final system/user prompt sent
-- to the LLM provider on every generation. HOF-001 (#13) — corrects ADR 015's
-- inaccurate "prompt snapshot" claim on 0002. Without these columns we have
-- no audit trail of what minors' data actually left to a third-party API.
--
-- Additive only (ADR 002 pre-prod policy). Existing rows keep NULL for both
-- columns; only generations created after this migration carry the audit
-- payload.

ALTER TABLE generations ADD COLUMN context_json TEXT;
ALTER TABLE generations ADD COLUMN prompt       TEXT;

-- +goose Down

-- SQLite supports DROP COLUMN since 3.35 (modernc.org/sqlite >= equivalent).
-- Down is documented but we never run it in production (ADR 002).
ALTER TABLE generations DROP COLUMN prompt;
ALTER TABLE generations DROP COLUMN context_json;
