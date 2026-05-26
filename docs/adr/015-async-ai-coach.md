# ADR 015 — Async AI Coach Generation

> Status: **Accepted** · Date: 2026-05-26

## Context

ADR 007 framed the LLM as a research assistant that drafts program proposals a
coach reviews. The first implementation made the LLM call **synchronously**
inside `POST /api/athletes/{id}/generate`:

1. The handler built the athlete context, called the provider, parsed
   CatalogJSON, and returned the result in the same response.
2. The parsed `MappingState` was held in an in-memory `sync.Map`
   (`Handlers.generateCache`) keyed by athlete ID until the coach clicked
   "Save", at which point `POST /generate/execute` looked it up and ran
   `ExecuteCatalogImport`.

That shape produced four real problems in practice:

1. **The HTTP server killed the response before the LLM finished.** The
   `*http.Server` in `cmd/replog/main.go` sets `WriteTimeout: 60 * time.Second`.
   The handler used `context.WithTimeout(r.Context(), 5*time.Minute)` and
   the provider HTTP clients also used 5-minute timeouts. Any Claude Sonnet
   generation over ~60 s — which is most multi-week programs — got its
   response dropped by `net/http` after the tokens had already been spent
   upstream.
2. **The in-memory cache was fragile.** It vanished on every restart /
   redeploy (NixOS service updates, manual restarts), had no TTL, and —
   keyed only by `athlete_id` — silently overwrote a coach's draft if a
   second tab or a second coach generated for the same athlete.
3. **The LLM call was tied to the browser tab.** Closing the tab,
   navigating away, or putting the laptop to sleep cancelled `r.Context()`
   and aborted the upstream request mid-flight.
4. **No audit trail.** Token spend, model name, prompt snapshot, and the
   coach who initiated each draft were not persisted. ADR 007's principle
   that a human reviews every LLM output deserves a more honest record.

## Decision

Run the LLM call in a **detached background goroutine** with a persistent
record per generation. SQLite is the queue and the cache.

### Schema (migration `0002_ai_coach_generations.sql`)

A new `generations` table with status lifecycle
`pending → running → (succeeded | failed | cancelled)`. The succeeded row
holds the full CatalogJSON and reasoning. `executed_at` is set when
`ExecuteCatalogImport` commits the draft to prevent double-import.
Indexed by `(athlete_id, status)` for the resume-on-page-load lookup and
by `(requested_by, created_at DESC)` for per-coach audit views.

### API surface

| Method | Path                                                     | Purpose |
|--------|----------------------------------------------------------|---------|
| GET    | `/api/athletes/{id}/generate`                            | Form data + the latest generation for resume on reload |
| POST   | `/api/athletes/{id}/generate`                            | Enqueue: insert pending row, spawn goroutine, return **202** + `generation_id` |
| GET    | `/api/athletes/{id}/generations/{genID}`                 | Poll status / read result (SPA polls every 2 s) |
| POST   | `/api/athletes/{id}/generations/{genID}/cancel`          | Mark pending/running as cancelled |
| POST   | `/api/athletes/{id}/generations/{genID}/execute`         | Commit the succeeded draft via `ExecuteCatalogImport` |

### Goroutine lifecycle

- Provider misconfiguration fails the POST synchronously (no token spend,
  no row to clean up).
- After `models.CreateGeneration`, the handler creates a
  `context.WithTimeout(context.Background(), 5*time.Minute)` — deliberately
  detached from `r.Context()` so client disconnect cannot cancel the LLM
  call we have already committed to paying for.
- The goroutine is tracked on a `sync.WaitGroup` on `Handlers` so:
  - Tests can deterministically wait for completion (`WaitForGenerations`).
  - Shutdown waits up to 10 s for in-flight generations to land their
    rows before exiting.
- All errors are persisted on the generation row — never returned to the
  HTTP caller, who has already disconnected.

### Server-restart cleanup

`models.ResetStaleRunningGenerations` runs once at startup right after
`RunMigrations` and marks any `pending`/`running` rows from a crashed
prior process as `failed: "Server restarted during generation. Please try
again."`. The SPA then surfaces a real error instead of a forever-spinning
draft.

### Notification

On `succeeded`, the goroutine sends a `generation_complete` in-app
notification to the requesting coach via the existing `notify.Send`
pipeline (ADR 008). The notification deep-links to the athlete's generate
page so the coach is dropped straight into the preview step (the form
endpoint exposes `latest_generation` for resume).

### SPA changes

`web/src/pages/GeneratePage.tsx` now:

- Submits and switches to a `generating` step that polls
  `/generations/{id}` every 2 s using TanStack Query's `refetchInterval`.
- Renders a "safe to close this tab" message — the draft survives.
- On page load, reads `latest_generation` from the form-data response and
  jumps the coach back to polling or preview if a draft is in flight.
- Exposes a Cancel button during `generating`.

## Consequences

### Positive

- **The 60 s `WriteTimeout` cliff is gone.** POSTs return in <1 s.
- **No more wasted tokens** on closed tabs, restarts, or redeploys.
- **Audit trail** — every draft (including rejected ones) is in the DB
  with prompt snapshot, model, token count, duration, and the coach who
  requested it. Aligns with ADR 007's review-before-approve principle.
- **Concurrency-safe.** A second tab or a second coach gets its own
  generation row instead of overwriting the first.
- **Resume works for free.** The form-data endpoint surfaces the latest
  generation; the SPA picks up where the coach left off.

### Negative

- One new table; one extra migration (the first additive one — see
  ADR 002's "Pre-Production Policy").
- Background goroutines hold one SQLite connection during the LLM call.
  With `SetMaxOpenConns(1)` this serializes against other writes, but
  the LLM call itself does no DB work between context-build and
  status-update, so the contention is brief.
- Shutdown best-effort waits 10 s for in-flight generations. Anything
  longer is cleaned up by the next startup's stale-reset.

### Out of scope (deferred)

- A per-coach "draft history" view — the data is there in
  `idx_generations_requested_by_created`, but the UI hasn't been built yet.
- SSE / WebSocket push instead of polling — 2 s polling is fine for the
  current single-family load and matches every other live view in the app.
- A persistent job queue (beanstalkd, river, etc.) — the single-binary
  deploy model and one-active-coach reality don't justify it.

## References

- ADR 002 — Database Migrations (post-prod additive-only policy applied here)
- ADR 007 — LLM-Assisted Program Generation (the human-reviews-everything principle)
- ADR 008 — Notification System (the channel `generation_complete` uses)
- ADR 013 — OpenAPI generation via swag (CI gate that catches stale specs)
