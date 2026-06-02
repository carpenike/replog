package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// harnessExemptRoutes is the small allow-list of production routes the
// test harness intentionally does not mount. Each entry needs a one-line
// reason so future maintainers can decide whether the exemption still
// holds.
var harnessExemptRoutes = map[routeKey]string{
	// No exemptions currently. (The passkey WebAuthn ceremony routes were
	// retired in ADR 019 Phase 1 — HOF-012; webui auth now federates to
	// PocketID and the OIDC start/callback live outside the /api group.)
}

// TestHarnessCoversAllProductionAPIRoutes compares the /api/... routes
// registered in cmd/replog/main.go against the routes the test harness
// (setupTest) mounts and fails if any production route is missing from
// the harness without a documented exemption.
//
// Why this exists: ADR 012 documents that the test harness intentionally
// duplicates the route table from main.go (so the harness file stays
// self-contained and grep-able), but that duplication has a failure
// mode — a route added in main.go without being added here will silently
// 404 in tests that assume it exists. We hit exactly that during the
// 2026-05-12 IDOR fix when the new IDOR tests for ListMissingTMs and
// BatchSetTMs were getting NotFound instead of Forbidden.
//
// This test is the cheap insurance: parse main.go's AST, collect every
// chi method-call literal pattern inside the r.Route("/api", ...) block,
// and assert the harness router covers each one (or has an exemption).
func TestHarnessCoversAllProductionAPIRoutes(t *testing.T) {
	mainPath := findMainGo(t)
	prod := parseAPIRoutesFromMainGo(t, mainPath)
	if len(prod) == 0 {
		t.Fatalf("found 0 /api routes in %s — parser is broken", mainPath)
	}

	env := setupTest(t)
	harness := walkHarnessRoutes(t, env.Router)

	// Build a quick lookup: method -> set of patterns mounted in harness.
	have := make(map[string]map[string]bool, len(harness))
	for _, r := range harness {
		if have[r.method] == nil {
			have[r.method] = map[string]bool{}
		}
		have[r.method][r.pattern] = true
	}

	var missing []string
	for _, r := range prod {
		if have[r.method][r.pattern] {
			continue
		}
		if _, exempt := harnessExemptRoutes[r]; exempt {
			continue
		}
		missing = append(missing, r.method+" "+r.pattern)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("test harness (handlers_test.go) is missing %d /api route(s) wired in cmd/replog/main.go:\n  %s\n\nAdd them to setupTest's chi router so tests for these endpoints don't silently 404. If a route intentionally has no harness coverage, add it to harnessExemptRoutes with a one-line reason.",
			len(missing), strings.Join(missing, "\n  "))
	}

	// Also catch stale exemptions — every entry in harnessExemptRoutes
	// must still correspond to a production route. Otherwise the
	// exemption silently rots.
	prodSet := make(map[routeKey]bool, len(prod))
	for _, r := range prod {
		prodSet[r] = true
	}
	for k := range harnessExemptRoutes {
		if !prodSet[k] {
			t.Errorf("harnessExemptRoutes entry %s %s no longer matches any production route — remove it",
				k.method, k.pattern)
		}
	}
}

type routeKey struct {
	method  string
	pattern string
}

// findMainGo locates cmd/replog/main.go relative to this test file. We
// can't use a hard-coded relative path because `go test ./...` may run
// from elsewhere; runtime.Caller gives us this file's location.
func findMainGo(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — can't locate main.go")
	}
	// internal/api/handlers_routes_test.go -> ../../cmd/replog/main.go
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(repoRoot, "cmd", "replog", "main.go")
}

// parseAPIRoutesFromMainGo parses main.go and extracts every chi method
// call (r.Get / r.Post / r.Put / r.Delete / r.Patch / r.Head / r.Options)
// registered inside the r.Route("/api", ...) block. Returns the
// (method, full /api/... pattern) pairs.
//
// We deliberately scope to the /api group rather than scanning the entire
// file because main.go also mounts top-level public routes (/healthz,
// /api/docs, /avatars/{filename}, ...) that the test harness does not and
// should not need to cover.
func parseAPIRoutesFromMainGo(t *testing.T, path string) []routeKey {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	chiMethods := map[string]string{
		"Get": http.MethodGet, "Post": http.MethodPost,
		"Put": http.MethodPut, "Delete": http.MethodDelete,
		"Patch": http.MethodPatch, "Head": http.MethodHead,
		"Options": http.MethodOptions,
	}

	// Locate the r.Route("/api", func(r chi.Router) { ... }) call
	// and grab its second argument (the FuncLit body) for scoped walking.
	var apiBody *ast.BlockStmt
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Route" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || strings.Trim(lit.Value, `"`) != "/api" {
			return true
		}
		fn, ok := call.Args[1].(*ast.FuncLit)
		if !ok {
			return true
		}
		apiBody = fn.Body
		return false
	})
	if apiBody == nil {
		t.Fatal("could not find r.Route(\"/api\", func(r chi.Router) { ... }) in main.go")
	}

	var out []routeKey
	ast.Inspect(apiBody, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method, ok := chiMethods[sel.Sel.Name]
		if !ok {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		pattern := strings.Trim(lit.Value, `"`)
		if !strings.HasPrefix(pattern, "/") {
			return true
		}
		out = append(out, routeKey{method: method, pattern: "/api" + pattern})
		return true
	})
	return out
}

// walkHarnessRoutes reflects on the chi router built by setupTest and
// returns every (method, pattern) pair mounted under it.
func walkHarnessRoutes(t *testing.T, r chi.Router) []routeKey {
	t.Helper()
	var out []routeKey
	walker := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// chi.Walk surfaces the fully-resolved route, including the
		// /api prefix from r.Route("/api", ...).
		out = append(out, routeKey{method: method, pattern: route})
		return nil
	}
	if err := chi.Walk(r, walker); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	return out
}
