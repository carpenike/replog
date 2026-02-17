# ADR 005: Adopt chi Router for Group-Based Middleware

**Status:** Accepted
**Date:** 2026-02-17

## Context

RepLog grew to ~70 routes registered on Go's stdlib `http.ServeMux` (Go 1.22+ method patterns). Authorization was enforced via three manual wrapper functions (`requireAuth`, `requireCoach`, `requireAdmin`) applied individually to every route registration:

```go
mux.Handle("GET /athletes", requireAuth(athletes.List))
mux.Handle("GET /athletes/new", requireCoach(athletes.NewForm))
mux.Handle("POST /athletes/{id}/delete", requireCoach(athletes.Delete))
```

This pattern created a growing risk: a missed or wrong wrapper on any new route would silently result in a privilege escalation vulnerability. The flat route listing in `main.go` made the authorization policy difficult to audit — reviewing which routes have which auth level required reading every line.

## Decision

Adopt `github.com/go-chi/chi/v5` as the router and organize routes into groups by authorization level, with middleware inherited via `r.Use()`:

```go
// Authenticated routes — RequireAuth + CSRF
r.Group(func(r chi.Router) {
    r.Use(withAuth)
    r.Use(withCSRF)
    r.Get("/athletes", athletes.List)
    r.Get("/athletes/{id}", athletes.Show)
})

// Coach-only routes — RequireAuth + CSRF + RequireCoach
r.Group(func(r chi.Router) {
    r.Use(withAuth)
    r.Use(withCSRF)
    r.Use(withCoach)
    r.Get("/athletes/new", athletes.NewForm)
    r.Post("/athletes", athletes.Create)
})
```

## Rationale

### Why chi

- **stdlib compatible:** Uses `http.Handler` and `http.HandlerFunc` — zero changes to existing handler signatures. Chi v5.2+ calls `r.SetPathValue()` so all existing `r.PathValue()` calls in handlers and `req.SetPathValue()` in tests continue to work unchanged.
- **Minimal:** chi is just a router — no framework, no custom context type, no ORM. Aligns with ADR 001's principle of minimal dependencies.
- **Group-based middleware:** Route groups with inherited `r.Use()` make the authorization policy structural and auditable. A route inside a coach group cannot accidentally miss coach middleware.
- **Incremental migration:** Because chi uses stdlib interfaces, the migration was a single-file change to `main.go` with no modifications to handlers, models, middleware, or tests.

### Why now

The authorization wrapper pattern was already a risk at 70 routes. Each new resource added 5-10 routes, compounding the manual application overhead. The cost of migration was low (one file changed, all tests pass), so deferring provided no benefit.

### Why not alternatives

| Alternative | Why not |
|------------|---------|
| Stay with `ServeMux` | Growing authorization risk from manual per-route wrapping |
| `gin` / `echo` | Custom context types (`gin.Context`) would require rewriting all handlers |
| `httprouter` | Different handler signature; less middleware support than chi |
| Homebrew grouping | Inventing a route declaration framework to avoid a battle-tested library adds complexity, not reduces it |

## Consequences

- **Positive:** Authorization policy is now visible in route structure — auditing which routes require coach/admin is reading four clearly labeled groups.
- **Positive:** New routes are added to the appropriate group; middleware is inherited automatically. Cannot forget auth.
- **Positive:** Zero handler or test changes — chi sets stdlib `PathValue`, preserving full backward compatibility.
- **Positive:** `RequestLogger` middleware moved to global `r.Use()` — no longer wrapped around the final handler manually.
- **Negative:** One new dependency (`github.com/go-chi/chi/v5`). Acceptable given chi's stability, minimal API surface, and stdlib compatibility.
- **Negative:** Routes for the same resource (e.g., `/athletes`) are now split across auth groups rather than colocated by resource. This is the correct trade-off — grouping by auth level makes the security policy explicit.
