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
//   - frame-src allows only the YouTube (no-cookie) and Vimeo players, for
//     exercise demo videos embedded in the session screen.
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

		// Share links (/s/{token} and /api/share/...) are opened with no
		// login: no referrer (the token is in the URL), no indexing, and the
		// strictest CSP the viewer can run under.
		if IsSharePath(r.URL.Path) {
			SetShareHeaders(w.Header())
			next.ServeHTTP(w, r)
			return
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
					"frame-src https://www.youtube-nocookie.com https://player.vimeo.com; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

// IsSharePath reports whether path is a public video share-link route: the
// viewer page /s/{token} or the API under /api/share/.
func IsSharePath(path string) bool {
	return strings.HasPrefix(path, "/s/") || strings.HasPrefix(path, "/api/share/")
}

// ShareViewerCSP is the Content-Security-Policy for share-link responses.
// The viewer page runs the SPA bundle (same script hashes as the app), plays
// one same-origin video and fetches its metadata; it embeds nothing, loads
// nothing from another origin, submits no forms and cannot be framed.
func ShareViewerCSP() string {
	return "default-src 'none'; " +
		"script-src " + spaScriptSrc + "; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"media-src 'self'; " +
		"connect-src 'self'; " +
		"manifest-src 'self'; " +
		"worker-src 'self'; " +
		"frame-src 'none'; " +
		"object-src 'none'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'none'; " +
		"form-action 'none'"
}

// SetShareHeaders sets the headers every share-link response carries,
// success or 404: never cached or stored, never indexed, no referrer, no
// MIME sniffing, and the share CSP.
func SetShareHeaders(h http.Header) {
	h.Set("Cache-Control", "private, no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	h.Set("Content-Security-Policy", ShareViewerCSP())
}
