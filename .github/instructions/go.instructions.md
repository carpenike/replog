---
applyTo: "**/*.go"
---

# Go Conventions for RepLog

## General

- Target Go 1.24+ — use `chi` router (`github.com/go-chi/chi/v5`) for routing with group-based middleware
- Module path: `github.com/carpenike/replog`
- All application code lives under `internal/` — it is not importable externally

## Error Handling

- Wrap errors with context: `fmt.Errorf("models: get athlete %d: %w", id, err)`
- Check `errors.Is(err, sql.ErrNoRows)` for not-found cases — return a domain-specific error or HTTP 404
- Never silently discard errors — always handle or propagate
- Log errors at the handler level, not in model functions

## HTTP Handlers

- Use `http.HandlerFunc` signature — `func(w http.ResponseWriter, r *http.Request)`
- Register routes on `chi.Router` (`github.com/go-chi/chi/v5`) — routes grouped by auth level
- Keep handlers thin: parse request → validate → call model → render template → handle error
- Return HTML fragments for htmx partial requests, full pages for normal navigation
- Use `http.Error(w, msg, code)` for error responses

## Routing with chi

- Use `chi.NewRouter()` as the top-level router
- Group routes by authorization level using `r.Group()`: public, session-loaded, authenticated, coach-only, admin-only
- Apply middleware via `r.Use()` on groups — middleware is inherited by all routes in the group
- Use `r.Get()`, `r.Post()`, etc. — chi sets stdlib `PathValue` so handlers use `r.PathValue("id")`
- Never apply auth middleware per-route with wrapper functions — rely on group-level `r.Use()` for defense-in-depth
- chi is the only allowed router dependency — no gin, echo, fiber, or other frameworks

## Templates

- Parse templates once at startup with `template.Must(template.ParseFS(...))`
- Use `embed.FS` to embed template files into the binary
- Template functions: register helpers for formatting dates, weights, percentages
- Never construct HTML strings in Go code — always use templates

## Models / Data Access

- One file per domain: `models/athlete.go`, `models/exercise.go`, `models/workout.go`
- Functions take `*sql.DB` or `*sql.Tx` as first parameter
- Write raw SQL — no ORM, no query builder
- Use `?` placeholders for all query parameters — never interpolate values
- Use `sql.NullString`, `sql.NullInt64`, `sql.NullFloat64` for nullable columns
- Scan into structs, not maps

## Testing

- Table-driven tests with `t.Run()` subtests
- Use `testing.TB` interface when helpers need `t.Helper()`
- Test models against a real in-memory SQLite database (`:memory:` with migrations applied)
- Test handlers with `httptest.NewRecorder()` and `httptest.NewRequest()`
- Name test files `*_test.go` in the same package

## Naming

- Exported types use PascalCase: `Athlete`, `WorkoutSet`, `TrainingMax`
- Unexported helpers use camelCase
- Acronyms: `ID` not `Id`, `HTTP` not `Http`, `SQL` not `Sql`
- Receiver names: short (1-2 chars), consistent within a type
