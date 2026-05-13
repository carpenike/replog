package middleware

import (
	"net/http"
	"strings"
)

// hstsEnabled controls whether SecurityHeaders emits a Strict-Transport-Security
// response header. Set by EnableHSTS(true) at startup when the server is
// reachable over HTTPS (REPLOG_BASE_URL is https:// or REPLOG_SECURE_COOKIES=true).
// Defaults to false so a developer running over plaintext localhost does not
// pin their browser to HTTPS for the listening port.
var hstsEnabled bool

// EnableHSTS toggles the Strict-Transport-Security response header. Should be
// called once at startup, before any requests are served.
func EnableHSTS(enabled bool) {
	hstsEnabled = enabled
}

// spaScriptSrc holds the script-src directive value for SPA responses.
// Built at startup from inline-script SHA-256 hashes extracted from
// web/dist/index.html (see SetSPAScriptHashes). Falls back to a permissive
// 'unsafe-inline' when no hashes are supplied — that path is hit only
// in dev when the frontend has not been built yet.
var spaScriptSrc = "'self' 'unsafe-inline'"

// SetSPAScriptHashes accepts the base64 SHA-256 hashes (without the
// "sha256-" prefix) of every inline <script> body in the SPA's
// index.html and rebuilds the script-src directive used by
// SecurityHeaders for SPA responses. Pass an empty slice to fall back
// to 'unsafe-inline' (suitable for dev-server / unbuilt-frontend
// scenarios). Should be called once at startup, before any requests
// are served.
//
// Hashes are quoted per the CSP spec: 'sha256-<base64>'.
func SetSPAScriptHashes(hashes []string) {
	if len(hashes) == 0 {
		spaScriptSrc = "'self' 'unsafe-inline'"
		return
	}
	parts := make([]string, 0, len(hashes)+1)
	parts = append(parts, "'self'")
	for _, h := range hashes {
		parts = append(parts, "'sha256-"+h+"'")
	}
	spaScriptSrc = strings.Join(parts, " ")
}

// SecurityHeaders sets standard security response headers on every request.
// These provide defense-in-depth against common web attacks:
//   - X-Frame-Options: DENY prevents clickjacking
//   - X-Content-Type-Options: nosniff prevents MIME-type sniffing
//   - Referrer-Policy: same-origin limits referrer leakage
//   - Permissions-Policy: disables browser features the SPA does not use
//   - Strict-Transport-Security: pins HTTPS for one year (only when HSTS
//     is enabled — see EnableHSTS)
//   - Content-Security-Policy: restricts resource loading origins
//
// CSP details:
//   - script-src for SPA responses is built at startup from SHA-256
//     hashes of every inline <script> in web/dist/index.html (via
//     SetSPAScriptHashes). When the frontend has not been built we
//     fall back to 'unsafe-inline'.
//   - style-src still includes 'unsafe-inline' because index.html has
//     an inline <style> block and shadcn/Radix portals inject runtime
//     stylesheets. Migrating that off 'unsafe-inline' is a separate
//     unsolved problem.
//   - The /api/docs (Swagger UI) endpoints get a relaxed CSP because
//     Swagger UI loads scripts and styles from unpkg.com.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()")

		if hstsEnabled {
			// One year, include subdomains, eligible for preload list.
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Relax CSP for API docs (Swagger UI loads from CDN).
		if strings.HasPrefix(r.URL.Path, "/api/docs") {
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"style-src 'self' 'unsafe-inline' https://unpkg.com; "+
					"script-src 'self' 'unsafe-inline' https://unpkg.com; "+
					"img-src 'self' data: https://unpkg.com; "+
					"connect-src 'self' https://unpkg.com")
		} else {
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"script-src "+spaScriptSrc+"; "+
					"img-src 'self' data:; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}
