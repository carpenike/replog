package middleware

import "net/http"

// SecurityHeaders sets standard security response headers on every request.
// These provide defense-in-depth against common web attacks:
//   - X-Frame-Options: DENY prevents clickjacking
//   - X-Content-Type-Options: nosniff prevents MIME-type sniffing
//   - Referrer-Policy: same-origin limits referrer leakage
//   - Content-Security-Policy: restricts resource loading origins
//
// Note: script-src includes 'unsafe-inline' because the base layout has a
// small inline <script> block for theme persistence and htmx configuration.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' https://fonts.googleapis.com; "+
				"font-src https://fonts.gstatic.com; "+
				"script-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
