-- +goose Up
-- Persist the deterministic post-generation lint results (warnings) and the
-- prompt-contract version that produced each draft.
--
--   * warnings        — JSON array of strings surfaced to the coach in the
--                       preview (e.g. an exercise name the LLM invented that
--                       does not resolve against the catalog it was given).
--                       NULL/absent = lint not run or clean.
--   * prompt_version  — llm.PromptVersion at generation time, so quality
--                       changes can be correlated with prompt edits across rows.
ALTER TABLE generations ADD COLUMN warnings       TEXT;
ALTER TABLE generations ADD COLUMN prompt_version TEXT;

-- +goose Down
ALTER TABLE generations DROP COLUMN prompt_version;
ALTER TABLE generations DROP COLUMN warnings;
