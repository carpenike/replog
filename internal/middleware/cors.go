package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig holds configuration for the CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is the list of origins allowed to make requests.
	// Use "*" to allow all origins (not recommended for cookie-based auth).
	AllowedOrigins []string
}

// CORS returns middleware that handles Cross-Origin Resource Sharing.
// Required for local development where the Vite dev server runs on a
// different port than the Go backend.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, Accept")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.Header().Set("Vary", "Origin")
			}

			// Handle preflight requests.
			if r.Method == http.MethodOptions && origin != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSFromEnv creates a CORS config from a comma-separated origins string.
// Returns nil if the origins string is empty (CORS disabled).
func CORSFromEnv(origins string) *CORSConfig {
	if origins == "" {
		return nil
	}
	var list []string
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			list = append(list, o)
		}
	}
	if len(list) == 0 {
		return nil
	}
	return &CORSConfig{AllowedOrigins: list}
}
