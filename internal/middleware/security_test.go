package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// runSecurityHeaders builds a tiny chain that runs SecurityHeaders in
// front of an empty handler and returns the response recorder.
func runSecurityHeaders(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)
	return rr
}

func TestSecurityHeaders_BaselineHeaders(t *testing.T) {
	rr := runSecurityHeaders(t, "/")
	checks := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "same-origin",
	}
	for h, want := range checks {
		if got := rr.Header().Get(h); got != want {
			t.Errorf("%s = %q, want %q", h, got, want)
		}
	}
	if perm := rr.Header().Get("Permissions-Policy"); !strings.Contains(perm, "camera=()") {
		t.Errorf("Permissions-Policy = %q, want substring 'camera=()'", perm)
	}
}

func TestSecurityHeaders_HSTSGated(t *testing.T) {
	// Default (HSTS disabled): no header.
	EnableHSTS(false)
	t.Cleanup(func() { EnableHSTS(false) })

	rr := runSecurityHeaders(t, "/")
	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS off: header = %q, want empty", got)
	}

	EnableHSTS(true)
	rr = runSecurityHeaders(t, "/")
	if got := rr.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=") {
		t.Errorf("HSTS on: header = %q, want substring 'max-age='", got)
	}
}

func TestSecurityHeaders_DocsCSPAllowsUnpkg(t *testing.T) {
	rr := runSecurityHeaders(t, "/api/docs")
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "https://unpkg.com") {
		t.Errorf("docs CSP = %q, want substring 'https://unpkg.com'", csp)
	}
}

// --- SPA script-src CSP (issue #8) ---

func TestSetSPAScriptHashes_DefaultIsUnsafeInline(t *testing.T) {
	// Reset to default — empty list means dev / unbuilt-frontend.
	SetSPAScriptHashes(nil)
	t.Cleanup(func() { SetSPAScriptHashes(nil) })

	rr := runSecurityHeaders(t, "/")
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("default script-src missing 'unsafe-inline':\n  %s", csp)
	}
}

func TestSetSPAScriptHashes_WithHashesDropsUnsafeInline(t *testing.T) {
	t.Cleanup(func() { SetSPAScriptHashes(nil) })

	hashes := []string{"abc123==", "def456=="}
	SetSPAScriptHashes(hashes)

	rr := runSecurityHeaders(t, "/")
	csp := rr.Header().Get("Content-Security-Policy")

	// Both hashes appear, properly quoted with the sha256- prefix.
	for _, h := range hashes {
		want := "'sha256-" + h + "'"
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing hash %s:\n  %s", want, csp)
		}
	}
	// 'self' is preserved.
	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP missing 'self':\n  %s", csp)
	}
	// 'unsafe-inline' is gone — the entire point of issue #8.
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") ||
		strings.Contains(csp, "'unsafe-inline'") &&
			strings.Index(csp, "'unsafe-inline'") < strings.Index(csp, "img-src") {
		// We only check the script-src segment — style-src 'unsafe-inline'
		// is intentional and lives later in the directive.
		scriptSrc := extractDirective(csp, "script-src")
		if strings.Contains(scriptSrc, "'unsafe-inline'") {
			t.Errorf("script-src still contains 'unsafe-inline':\n  %s", scriptSrc)
		}
	}
}

func TestSetSPAScriptHashes_DocsCSPIsUnaffected(t *testing.T) {
	// The Swagger UI relaxed CSP must keep 'unsafe-inline' regardless
	// of what we set for the SPA script-src — different code paths,
	// different security needs.
	SetSPAScriptHashes([]string{"abc123=="})
	t.Cleanup(func() { SetSPAScriptHashes(nil) })

	rr := runSecurityHeaders(t, "/api/docs/openapi.yaml")
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline' https://unpkg.com") {
		t.Errorf("docs CSP changed unexpectedly:\n  %s", csp)
	}
}

// extractDirective returns the value of a single CSP directive from a
// full Content-Security-Policy header value.
func extractDirective(csp, name string) string {
	for _, part := range strings.Split(csp, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, name+" ") || part == name {
			return part
		}
	}
	return ""
}
