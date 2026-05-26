-- +goose Up

-- AI Coach generations — persistent record of every LLM program-draft request.
--
-- Replaces the in-memory generateCache (sync.Map) used by the original
-- synchronous /generate flow. The handler now returns immediately after
-- inserting a `pending` row and kicks off a background goroutine that runs
-- the LLM call with a detached context. The SPA polls the status endpoint
-- and the coach later executes the saved catalog_json into program_templates.
--
-- Benefits over the in-memory cache:
--   * Survives server restart and coach disconnect (no wasted tokens).
--   * Eliminates the 60 s server WriteTimeout cliff for /generate.
--   * Audit trail of every draft (token spend, model, prompt snapshot) —
--     matches ADR 007's "human reviews every LLM output" principle.
--   * Concurrent requests for the same athlete no longer overwrite each other.

CREATE TABLE IF NOT EXISTS generations (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id    INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    requested_by  INTEGER NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    status        TEXT    NOT NULL CHECK(status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),

    -- Snapshot of the GenerationRequest as JSON. Lets us re-run or audit
    -- with the exact inputs the coach originally submitted.
    request_json  TEXT    NOT NULL,

    -- LLM output. catalog_json is the parsed, validated CatalogJSON ready
    -- for ExecuteCatalogImport. reasoning is the <reasoning> block.
    catalog_json  TEXT,
    reasoning     TEXT,

    -- Provider metadata for audit / cost tracking.
    model         TEXT,
    tokens_used   INTEGER NOT NULL DEFAULT 0,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    stop_reason   TEXT,

    -- On failure, the user-friendly error message returned to the coach.
    error         TEXT,

    -- Set when ExecuteCatalogImport has committed the program — prevents
    -- a coach from accidentally importing the same draft twice.
    executed_at   DATETIME,

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at    DATETIME,
    completed_at  DATETIME
);

-- Lookups for the "is there an in-flight draft for this athlete?" check
-- that the form data endpoint runs to support resume on page reload.
CREATE INDEX IF NOT EXISTS idx_generations_athlete_status
    ON generations(athlete_id, status);

-- Audit views by coach (e.g. "show me my recent drafts").
CREATE INDEX IF NOT EXISTS idx_generations_requested_by_created
    ON generations(requested_by, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_generations_requested_by_created;
DROP INDEX IF EXISTS idx_generations_athlete_status;
DROP TABLE IF EXISTS generations;
