package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.status = code
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status = http.StatusOK
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}

// redactedPaths is the set of URL prefixes whose remaining path segments
// carry a secret (magic-link tokens, etc.) and must be scrubbed from access
// logs. Anything matching is logged as "<prefix>/<redacted>".
var redactedPaths = []string{
	"/api/auth/token/",
}

// scrubPath replaces secret-bearing path tails with "<redacted>" so they
// never reach stdout / journald / external log shippers.
func scrubPath(path string) string {
	for _, prefix := range redactedPaths {
		if strings.HasPrefix(path, prefix) && len(path) > len(prefix) {
			return prefix + "<redacted>"
		}
	}
	return path
}

// RequestLogger logs each HTTP request with method, path, status code, and duration.
// Paths matching redactedPaths have their secret-bearing tail replaced with
// "<redacted>" so login tokens and similar secrets never end up in logs.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		log.Printf("%s %s %d %s", r.Method, scrubPath(r.URL.Path), sw.status, time.Since(start).Round(time.Microsecond))
	})
}
