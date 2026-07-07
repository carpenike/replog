-- +goose Up

-- Enforce the "one in-flight generation per (athlete, kind)" invariant at the
-- database level (HOF-015 / app review §3a).
--
-- The submit handlers pre-check with PendingOrRunningGenerationForAthlete and
-- then INSERT, but that read-then-write is a check-then-act race: two concurrent
-- submits can both pass the check and both enqueue, burning provider tokens
-- twice and confusing the SPA's resume-on-reload logic. A partial UNIQUE index
-- closes the window — the second INSERT fails with a UNIQUE violation, which the
-- model maps to ErrGenerationInFlight and the handler surfaces as 409.
--
-- Partial (WHERE status IN ('pending','running')) so completed/failed/cancelled
-- rows do not participate: an athlete can have any number of historical
-- generations, just not two simultaneously active ones of the same kind.
CREATE UNIQUE INDEX IF NOT EXISTS idx_generations_inflight
    ON generations(athlete_id, kind)
    WHERE status IN ('pending', 'running');

-- +goose Down

DROP INDEX IF EXISTS idx_generations_inflight;
