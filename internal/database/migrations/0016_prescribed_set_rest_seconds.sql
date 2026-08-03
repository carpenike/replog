-- +goose Up
ALTER TABLE prescribed_sets
    ADD COLUMN rest_seconds INTEGER CHECK(rest_seconds IS NULL OR rest_seconds >= 0);

-- +goose Down
ALTER TABLE prescribed_sets DROP COLUMN rest_seconds;