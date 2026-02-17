package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
)

type csrfContextKey string

// csrfTokenCtxKey is the context key for the CSRF token.
const csrfTokenCtxKey csrfContextKey = "csrf_token"

// CSRFProtect is middleware that generates a CSRF token and stores it in the
// session. On state-changing requests (POST, PUT, DELETE, PATCH) it validates
// that the request includes a matching token in the X-CSRF-Token header or the
// csrf_token form field.
//
// This middleware must run inside scs LoadAndSave (i.e., after RequireAuth)
// so the session is available.
func CSRFProtect(sm *scs.SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensure a token exists in the session.
		token := sm.GetString(r.Context(), "csrf_token")
		if token == "" {
			token = generateCSRFToken()
			sm.Put(r.Context(), "csrf_token", token)
		}

		// Validate on state-changing methods.
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			requestToken := r.Header.Get("X-CSRF-Token")
			if requestToken == "" {
				// Fallback: check form field. ParseForm handles
				// url-encoded bodies; for multipart (file uploads)
				// we also need ParseMultipartForm.
				_ = r.ParseForm()
				requestToken = r.FormValue("csrf_token")
				if requestToken == "" && isMultipart(r) {
					_ = r.ParseMultipartForm(32 << 20) // 32 MB limit
					requestToken = r.FormValue("csrf_token")
				}
			}
			if !csrfTokensMatch(token, requestToken) {
				http.Error(w, "Forbidden — invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		// Store token in context for template rendering.
		ctx := context.WithValue(r.Context(), csrfTokenCtxKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CSRFTokenFromContext retrieves the CSRF token from the request context.
func CSRFTokenFromContext(ctx context.Context) string {
	s, _ := ctx.Value(csrfTokenCtxKey).(string)
	return s
}

// generateCSRFToken returns a 32-byte hex-encoded random string.
func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail on supported platforms.
		panic("csrf: failed to generate random token: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// csrfTokensMatch compares two tokens using constant-time comparison to
// prevent timing attacks.
func csrfTokensMatch(expected, actual string) bool {
	if expected == "" || actual == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

// isMultipart reports whether the request has a multipart/form-data content type.
func isMultipart(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "multipart/form-data")
}
