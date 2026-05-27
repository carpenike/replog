package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	frontend "github.com/carpenike/replog/web"
)

// TestSpaFallback_StaleAssetReturns404 is the regression test for the
// "'text/html' is not a valid JavaScript MIME type" error users hit
// after a deploy: their cached index.html references chunk filenames
// that the new build no longer has on disk, the SPA imports them via
// fetch, and the old fallback served index.html for the missing path —
// which the browser then refused to evaluate as JavaScript.
//
// The fix: missing /assets/* paths must 404 (not serve HTML).
func TestSpaFallback_StaleAssetReturns404(t *testing.T) {
	h := spaFallbackHandler()

	req := httptest.NewRequest(http.MethodGet, "/assets/NotificationsList-DEADBEEF.js", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for stale asset, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); strings.HasPrefix(ct, "text/html") && rr.Code != http.StatusNotFound {
		t.Errorf("must not serve HTML for /assets/* misses (got Content-Type=%q)", ct)
	}
}

// TestSpaFallback_UnknownRouteServesIndex confirms the SPA-route side
// still works: any non-asset URL (a React Router path the server doesn't
// otherwise handle) gets index.html so client-side routing kicks in.
func TestSpaFallback_UnknownRouteServesIndex(t *testing.T) {
	h := spaFallbackHandler()

	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA route, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected HTML for SPA route, got Content-Type=%q", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control=no-cache on SPA HTML (so browsers revalidate after deploys), got %q", cc)
	}
	if body := rr.Body.String(); !strings.Contains(body, "<!doctype html") && !strings.Contains(body, "<!DOCTYPE html") {
		t.Errorf("expected index.html body, got %q", body[:min(120, len(body))])
	}
}

// TestSpaFallback_HashedAssetIsImmutable confirms /assets/* responses
// carry a long-lived immutable Cache-Control so subsequent requests for
// the same hashed filename hit the browser cache (Vite already hashes,
// so a new deploy means a new filename — never a cache-poison risk).
func TestSpaFallback_HashedAssetIsImmutable(t *testing.T) {
	h := spaFallbackHandler()

	// Discover an actual asset name from the embedded dist so this test
	// stays valid across rebuilds (Vite renames chunks every build).
	asset := findFirstAsset(t)
	if asset == "" {
		t.Skip("no /assets/ files embedded (running without web/dist?)")
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/"+asset, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for embedded asset %q, got %d", asset, rr.Code)
	}
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("expected immutable Cache-Control on hashed asset, got %q", cc)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// findFirstAsset returns the first filename found under dist/assets in
// the embedded frontend filesystem, or "" if none exists (dev / no build).
func findFirstAsset(t *testing.T) string {
	t.Helper()
	dist, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return ""
	}
	var found string
	_ = fs.WalkDir(dist, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		found = strings.TrimPrefix(path, "assets/")
		return fs.SkipAll
	})
	return found
}
