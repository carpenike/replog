package middleware

import (
	"net/http"
	"strings"
)

// DefaultMaxJSONBody is the cap applied to ordinary JSON request bodies. 1 MiB
// is comfortably larger than any legitimate JSON payload the SPA sends while
// still bounding memory a single request can force the server to buffer.
const DefaultMaxJSONBody int64 = 1 << 20

// MaxJSONBody returns middleware that caps the request body at limit bytes via
// http.MaxBytesReader, so an oversized JSON POST is rejected with 413 instead of
// being fully read into memory.
//
// Routes that legitimately carry larger bodies are exempted (see
// exemptFromBodyLimit): multipart uploads (avatars, imports) install their own,
// larger MaxBytesReader in the handler — double-wrapping here with a smaller cap
// would break them — and the bulk import-execute / catalog endpoints carry large
// mapping JSON. The native MCP endpoint streams and is left to its own handler.
func MaxJSONBody(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && !exemptFromBodyLimit(r) {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// exemptFromBodyLimit reports whether a request should skip the JSON body cap.
func exemptFromBodyLimit(r *http.Request) bool {
	// Multipart uploads set their own (larger) cap in the handler.
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return true
	}
	p := r.URL.Path
	switch {
	case strings.HasSuffix(p, "/import/upload"), strings.HasSuffix(p, "/import/execute"):
		return true
	case strings.HasPrefix(p, "/api/catalog/import"):
		return true
	case strings.HasPrefix(p, "/api/mcp"):
		return true
	}
	return false
}
