# ADR 012 — Shared API Test Harness

**Status:** Accepted
**Date:** 2026-05-12

## Context

After ADR 011 the backend became JSON-only and grew to ~120 endpoints across
20+ handler files. The pre-SPA test suite was a mix of styles: some handlers
were tested with `httptest.NewRecorder` against a single handler function,
some used hand-rolled `*sql.DB` fixtures, some were tested only through the
old SSR layer that no longer existed. Coverage was patchy and inconsistent,
and several real bugs were sitting un-caught:

- `models.AutoApproveWorkout`'s error return was being silently dropped
  (errcheck — failed lint for 49 commits without anyone noticing).
- Catalog import and AI-generate executors panicked in production because
  `MappingState.Parsed` was nil after gob round-trip.
- `/api/admin/settings` returned sensitive plaintext alongside the masked
  preview (a small but real info-leak).
- `/api/*` endpoints replied with `303 -> /login` instead of `401 JSON`,
  breaking the SPA's auth-recovery flow.
- The workout-set logger was inserting `rep_type = "standard"` while the
  schema's CHECK constraint required `'reps'`, so set logging was just
  broken.

None of these would have shipped if there had been a uniform way to write
"send a real request through the real router with a real session cookie and
assert on the JSON response." The decision is to standardize on exactly that.

## Decision

Adopt a single shared test harness in
[`internal/api/handlers_test.go`](../../internal/api/handlers_test.go)
that mounts **every** production `/api` route on a real `chi` router with
the real `scs` session manager and a fresh in-memory SQLite database with
all migrations applied.

### Shape of the harness

```go
type testEnv struct {
    DB       *sql.DB
    Sessions *scs.SessionManager
    Handlers *Handlers
    Router   chi.Router
}

func setupTest(t *testing.T) *testEnv
```

`setupTest` is the only entry point. It:

1. Opens `database.Open(":memory:")` and runs `database.RunMigrations`.
2. Builds an `scs.SessionManager` with the same lifetime / cookie attrs as
   production, backed by the in-memory DB (`t.Cleanup` closes the DB).
3. Constructs a real `*Handlers` with a `t.TempDir()` for `AvatarDir`.
4. Wires a `chi.Router` whose `/api` group mirrors `cmd/replog/main.go`
   route-for-route — same middleware order, same auth groups, same path
   patterns. This is intentionally a copy, not a shared helper, because
   "the routes the harness exposes are the routes prod exposes" is the
   invariant that matters; a shared helper would let drift creep in
   silently.

### Convenience methods on `*testEnv`

| Method | Purpose |
|--------|---------|
| `createUser(t, name, isCoach, isAdmin)` | Inserts a user with password `"password123"` and ensures default preferences. |
| `createAthlete(t, name, coachID)` | Inserts an athlete owned by the given coach. |
| `loginAs(t, user)` | Issues `POST /api/login` and returns the `Set-Cookie` jar. |
| `do(t, method, path, body, cookies)` | Sends a request through the router. `body` accepts `nil`, `string`, `[]byte`, or any JSON-marshallable value. |

Resource-specific helpers (`createExercise`, `createWorkout`, etc.) live in
the test files that use them, not in `handlers_test.go` — only the helpers
used by *every* test file get promoted to the shared harness.

### Top-level assertion helpers

```go
func requireStatus(t *testing.T, rr *httptest.ResponseRecorder, want int)
func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, out any)
```

Tests stay short and obvious: `setupTest` -> `createUser`/`createAthlete`
-> `loginAs` -> `do` -> `requireStatus` + `decodeJSON`.

### Dependency injection for non-deterministic deps

Production handlers that talk to external services hold a function-typed
field on `*Handlers` so tests can swap it. The first such field is
`Handlers.LLMProviderFactory func(*sql.DB) (llm.Provider, error)`, which
defaults to `llm.NewProviderFromSettings` and is overridden in
`handlers_generate_test.go` with `llm.MockProvider`. New external
integrations follow the same pattern — never reach for an interface mock
framework or HTTP middleware injection.

## Rationale

### Why a real router and real session manager

The bugs that motivated this ADR all lived at the seams: middleware order
(401 vs 303), session encoding (gob round-trip), CHECK constraints (rep
type defaulting), DTO redaction (admin settings). Each one would slip past
a "call the handler function directly" test because the failure was
*between* the handler and something else. Mounting the production router
with the production middleware stack on a real DB catches them
automatically.

### Why in-memory SQLite per test

- Fresh state per `t.Run` makes parallelism free if we want it later.
- All the DDL (CHECK constraints, triggers, partial unique indexes) is
  exercised — no schema mocking.
- The pure-Go `modernc.org/sqlite` driver means no CGO and no test
  containers; `go test ./...` just works in CI and on the laptop.
- Total test runtime stays in the ~16-second range for ~140 API tests.

### Why a copy of the route table, not a shared helper

The harness intentionally duplicates the `/api` route table from
`main.go`. This is the only piece a future contributor must keep in sync.
The trade-off:

- Pro: when a route is added in `main.go` but not the harness (or vice
  versa) the test suite catches it instantly — usually as a 404 from
  whatever new test is being written for the new route.
- Pro: the harness file is self-contained and readable; you can see every
  endpoint the tests reach in one place.
- Con: two places to update on every new route. Acceptable; there is no
  shared abstraction that would catch routing bugs better than two
  developers reading the same file.

### Why not testify / ginkgo / fancy assertion library

Stdlib `testing` + tiny `requireStatus` / `decodeJSON` helpers cover every
case so far without dragging in another dependency or a different test
DSL. If we ever need richer assertions we can add them locally.

## Conventions

- One test file per handler file: `handlers_workouts.go` ->
  `handlers_workouts_test.go`. Keeps file navigation obvious.
- Test names follow `TestHandler_Scenario`:
  `TestCreateWorkout_OneWorkoutPerDay`, `TestUpdateSetting_RedactsSecret`.
  Scenarios that regress a previously-fixed bug get a comment pointing
  to the commit, e.g.
  `// TestAddWorkoutSet_DefaultRepTypeAndCategory regresses ef4f7b3`.
- Always cover at least: success path, IDOR (other coach forbidden),
  unauthenticated (401), and any input validation branches.
- Always assert the JSON body, not just the status code — the bugs that
  prompted this ADR were status-code-correct but body-wrong.

## Consequences

### Positive

- ~140 API tests across 10 handler test files, all using one harness.
  Total backend test runtime ~16 s.
- IDOR coverage is now structural — every coach-scoped endpoint has at
  least one "coach B cannot touch coach A's athlete" test.
- Bugs caught while writing the harness paid back the harness's own cost
  many times over. A new endpoint without a 401 + IDOR test now stands
  out in code review.
- `Handlers.LLMProviderFactory` establishes a clear pattern for future
  external-service injection points.

### Negative

- Route table duplicated between `main.go` and `handlers_test.go`. Drift
  would be caught by tests but is still a manual sync step.
- All API tests live in package `api` and so share a process. We have not
  needed `t.Parallel()` yet — if/when we do, the per-test in-memory DB is
  already isolated, but the shared `init()`-registered gob types and any
  package-level state would need an audit.

### Neutral

- Test files do not use the `_test` external package — they live in
  package `api` so they can poke at unexported fields like
  `Handlers.generateCache` if needed. Not currently exploited; available.
