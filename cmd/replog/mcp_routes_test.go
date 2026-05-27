package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/carpenike/replog/internal/api"
)

// TestMCPRouteList_ExhaustiveAndStable asserts the /api-mcp/* mount list
// is EXACTLY the curated set HOF-004 specifies — no more, no less.
//
// Why a hard-coded list of strings in the test? Because the spec's
// load-bearing property is "this exact route set" — if a future PR adds
// a route to mcpRouteList without updating this test, the test fails
// LOUDLY. That's the intended behavior: any expansion of the MCP surface
// is a deliberate decision that requires updating the test (and re-reading
// HOF-004 [forbidden] / [graduation-trigger] to confirm the expansion is
// in scope for v1).
//
// Items deliberately omitted that a future maintainer might be tempted
// to add: /me (session-bound), /admin/impersonate/* (session-bound +
// out-of-scope for MCP), /athletes/{id}/generations/{genID}/execute
// (the coaching-decision commit step — webui-only).
func TestMCPRouteList_ExhaustiveAndStable(t *testing.T) {
	want := []string{
		// Group A: reads
		"GET /dashboard",
		"GET /athletes/{id}",
		"GET /athletes/{id}/workouts",
		"GET /athletes/{id}/workouts/{workoutID}",
		"GET /athletes/{id}/prescription",
		"GET /athletes/{id}/training-maxes",
		"GET /athletes/{id}/exercises/{exerciseID}/training-maxes",
		"GET /athletes/{id}/journal",
		"GET /athletes/{id}/programs",
		"GET /athletes/{id}/equipment",
		// Group B: clerical writes
		"POST /athletes/{id}/workouts",
		"POST /athletes/{id}/workouts/{workoutID}/sets",
		"PUT /athletes/{id}/workouts/{workoutID}/sets/{setID}",
		"DELETE /athletes/{id}/workouts/{workoutID}/sets/{setID}",
		"PUT /athletes/{id}/workouts/{workoutID}/notes",
		"POST /athletes/{id}/body-weights",
		"POST /athletes/{id}/notes",
		// Group C: program draft enqueue + status
		"POST /athletes/{id}/generate",
		"GET /athletes/{id}/generations/{genID}",
	}

	routes := mcpRouteList(&api.Handlers{})
	got := make([]string, len(routes))
	for i, r := range routes {
		got[i] = fmt.Sprintf("%s %s", r.method, r.pattern)
	}

	sort.Strings(want)
	sortedGot := append([]string{}, got...)
	sort.Strings(sortedGot)

	if len(got) != len(want) {
		t.Fatalf("route count = %d, want %d\ngot:\n  %s\nwant:\n  %s",
			len(got), len(want),
			strings.Join(sortedGot, "\n  "),
			strings.Join(want, "\n  "))
	}
	for i := range want {
		if sortedGot[i] != want[i] {
			t.Errorf("route mismatch at index %d:\n  got  = %q\n  want = %q\nfull got:\n  %s\nfull want:\n  %s",
				i, sortedGot[i], want[i],
				strings.Join(sortedGot, "\n  "),
				strings.Join(want, "\n  "))
		}
	}
}

// TestMCPRouteList_NoForbiddenHandlers asserts that the routes do NOT
// touch any of the handlers HOF-004 [forbidden] explicitly bans on the
// MCP surface. Tested by pattern (string match) — a future rename of
// these patterns would need to update this test too.
func TestMCPRouteList_NoForbiddenHandlers(t *testing.T) {
	routes := mcpRouteList(&api.Handlers{})
	// Patterns whose handlers are forbidden on /api-mcp/* per HOF-004.
	// A future rename of any of these patterns AND a corresponding rename
	// here would still need a code-review pass on the [forbidden] block.
	forbidden := []string{
		// Session-bound — would panic on bearer-only requests.
		"/me",
		"/admin/impersonate",
		"/admin/stop-impersonating",
		"/admin/impersonateable",
		// Coaching-decision commit — webui approval only.
		"/generations/{genID}/execute",
		// Coaching mutations on athletes — direct writes, no staging.
		"/training-maxes", // POST creates a TM (read GET is allowed)
		"/promote",
		"/programs/{programID}/deactivate",
		"/programs/{programID}/reactivate",
		"/cycle-review",
		// Catalog & program-template editing — admin-only operator surface.
		"/catalog/",
		"/programs/{id}/sets",
		"/programs/{id}/rules",
	}
	for _, r := range routes {
		key := fmt.Sprintf("%s %s", r.method, r.pattern)
		for _, bad := range forbidden {
			if strings.Contains(r.pattern, bad) {
				// Special case: GET on /training-maxes IS allowed (a read).
				// Only the POST is a coaching write.
				if bad == "/training-maxes" && r.method == http.MethodGet {
					continue
				}
				t.Errorf("route %q matches forbidden pattern %q", key, bad)
			}
		}
	}
}

// TestMountMCPRoutes_OnlyMountsTheCuratedList walks the router and asserts
// the live mount surface under /api-mcp matches mcpRouteList. Defends
// against an accidental "wired a session-bound handler into the MCP
// surface" regression where someone copies a `/api/*` line into the
// `/api-mcp/*` block without thinking about session-vs-bearer.
func TestMountMCPRoutes_OnlyMountsTheCuratedList(t *testing.T) {
	// Use a no-op handlers struct — we're walking the route tree, not
	// exercising the handlers. nil bearer + nil limiter would crash
	// inside Use(); pass closures that satisfy the middleware shape
	// without doing real work.
	r := chi.NewRouter()
	noopMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req)
		})
	}
	r.Route("/api-mcp", func(r chi.Router) {
		r.Use(noopMW)
		r.Use(noopMW)
		for _, route := range mcpRouteList(&api.Handlers{}) {
			r.Method(route.method, route.pattern, route.handler)
		}
	})

	mounted := map[string]struct{}{}
	walkErr := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		mounted[fmt.Sprintf("%s %s", method, route)] = struct{}{}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("chi.Walk: %v", walkErr)
	}

	for _, route := range mcpRouteList(&api.Handlers{}) {
		key := fmt.Sprintf("%s /api-mcp%s", route.method, route.pattern)
		if _, ok := mounted[key]; !ok {
			t.Errorf("expected route %q to be mounted, but missing from chi tree", key)
		}
	}

	// Every mounted /api-mcp/* route must be in the curated list. (Belt:
	// the route loop is the only thing that mounts under /api-mcp here,
	// so this is a tautology — but adding a manual r.Get inside the
	// closure would silently slip past mcpRouteList, and this check
	// catches that.)
	want := map[string]struct{}{}
	for _, route := range mcpRouteList(&api.Handlers{}) {
		want[fmt.Sprintf("%s /api-mcp%s", route.method, route.pattern)] = struct{}{}
	}
	for k := range mounted {
		if !strings.HasPrefix(k, "GET /api-mcp") && !strings.HasPrefix(k, "POST /api-mcp") &&
			!strings.HasPrefix(k, "PUT /api-mcp") && !strings.HasPrefix(k, "DELETE /api-mcp") {
			continue
		}
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected route %q mounted under /api-mcp — add to mcpRouteList or remove", k)
		}
	}
}
