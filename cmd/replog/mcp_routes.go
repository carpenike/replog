package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/carpenike/replog/internal/api"
	"github.com/carpenike/replog/internal/middleware"
)

// mcpRoute is one entry in the curated /api-mcp/* mount list (HOF-004).
//
// The list is deliberately exhaustive — only handlers explicitly enumerated
// here are reachable via bearer auth. Session-bound handlers (Me, the
// impersonation pair) and the coaching-decision execute path are NOT in
// this list and that absence IS the doctrine: ADR 007 / 015's "no
// automated coaching" line + the spec's `[forbidden]` block.
//
// Tests in mcp_routes_test.go assert this list is exactly what chi mounts.
type mcpRoute struct {
	method  string
	pattern string
	handler http.HandlerFunc
}

// mcpRouteList returns the canonical /api-mcp/* mount list. Exposed as a
// function (not a package var) so the test harness can build one against
// a test-double *api.Handlers without coupling to the production wiring.
func mcpRouteList(h *api.Handlers) []mcpRoute {
	return []mcpRoute{
		// --- Group A: reads (direct) ---
		{http.MethodGet, "/dashboard", h.Dashboard},
		{http.MethodGet, "/athletes/{id}", h.GetAthlete},
		{http.MethodGet, "/athletes/{id}/workouts", h.ListWorkouts},
		{http.MethodGet, "/athletes/{id}/workouts/{workoutID}", h.GetWorkout},
		{http.MethodGet, "/athletes/{id}/prescription", h.GetPrescription},
		{http.MethodGet, "/athletes/{id}/training-maxes", h.ListTrainingMaxes},
		{http.MethodGet, "/athletes/{id}/exercises/{exerciseID}/training-maxes", h.GetTrainingMaxHistory},
		{http.MethodGet, "/athletes/{id}/journal", h.ListJournalEntries},
		{http.MethodGet, "/athletes/{id}/programs", h.ListAthletePrograms},
		{http.MethodGet, "/athletes/{id}/equipment", h.ListAthleteEquipment},

		// --- Group B: clerical writes (direct, gated by CanManageAthlete) ---
		{http.MethodPost, "/athletes/{id}/workouts", h.CreateWorkout},
		{http.MethodPost, "/athletes/{id}/workouts/{workoutID}/sets", h.AddWorkoutSet},
		{http.MethodPut, "/athletes/{id}/workouts/{workoutID}/sets/{setID}", h.UpdateWorkoutSet},
		{http.MethodDelete, "/athletes/{id}/workouts/{workoutID}/sets/{setID}", h.DeleteWorkoutSet},
		{http.MethodPut, "/athletes/{id}/workouts/{workoutID}/notes", h.UpdateWorkoutNotes},
		{http.MethodPost, "/athletes/{id}/body-weights", h.CreateBodyWeight},
		{http.MethodPost, "/athletes/{id}/notes", h.CreateAthleteNote},

		// --- Group C: program draft (enqueue + status only) ---
		// `…/generations/{genID}/execute` is INTENTIONALLY OMITTED — the
		// commit step stays on the webui where the human's click is the
		// approval. See HOF-004 [forbidden].
		{http.MethodPost, "/athletes/{id}/generate", h.GenerateSubmit},
		{http.MethodGet, "/athletes/{id}/generations/{genID}", h.GenerationStatus},
	}
}

// mountMCPRoutes registers the curated /api-mcp/* route group on the
// given chi router. The bearer middleware verifies JWTs minted by the
// homelab-mcp OAuth AS; limiter caps per-IP request rate so a flood
// cannot starve webui logins (separate bucket from the webui limiter).
func mountMCPRoutes(r chi.Router, bearer *middleware.BearerAuth, limiter *middleware.RateLimiter, h *api.Handlers) {
	routes := mcpRouteList(h)
	r.Route("/api-mcp", func(r chi.Router) {
		r.Use(limiter.Limit)
		r.Use(bearer.Middleware)
		for _, route := range routes {
			r.Method(route.method, route.pattern, route.handler)
		}
	})
}
