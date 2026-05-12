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
// Note: script-src includes 'unsafe-inline' because index.html has a small
// inline theme-bootstrap script that runs before React hydrates. Migrating
// to a SHA-256 hash is tracked separately. style-src includes 'unsafe-inline'
// because index.html also inlines a body-background style and Vite injects
// runtime <style> tags for HMR/CSS-modules.
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
					"script-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}
