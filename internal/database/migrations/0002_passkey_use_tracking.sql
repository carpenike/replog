-- +goose Up
ALTER TABLE webauthn_credentials ADD COLUMN use_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE webauthn_credentials ADD COLUMN last_used_at DATETIME;

-- +goose Down
ALTER TABLE webauthn_credentials DROP COLUMN use_count;
ALTER TABLE webauthn_credentials DROP COLUMN last_used_at;
