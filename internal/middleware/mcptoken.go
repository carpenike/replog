package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/carpenike/replog/internal/models"
)

// MCPTokenAuth authenticates requests to the native MCP server (/api/mcp)
// using RepLog's own opaque bearer tokens (ADR 019 Phases 2+3 — HOF-013).
//
// This replaces the retired JWKS/RS256 verifier: RepLog is now its own OAuth
// Authorization Server (see internal/mcpoauth) and mints `rlpat_` opaque
// tokens. The token IS the caller identity — it is the SHA-256-hashed handle
// to a row in mcp_tokens whose user_id FK is the authenticated RepLog user.
//
// Validation order (cheapest-first, fail-closed):
//  1. extract the Bearer credential; empty → 401 missing-bearer-token;
//  2. prefix-gate on `rlpat_` BEFORE any DB hit (a non-MCP credential is
//     rejected without a query);
//  3. hash + lookup + revoked/expired checks (models.ValidateMCPToken);
//     any miss → 401 invalid-token (no unknown-user branch: a token cannot
//     exist without a valid user_id FK);
//  4. mcp_enabled gate → 403 mcp-not-enabled (the admin kill-switch still
//     applies after a valid token resolves to a user).
//
// On success it attaches the user + prefs to the request context under the
// SAME keys the scs-cookie RequireAuth middleware uses (UserContextKey /
// PrefsContextKey), so downstream handlers that call UserFromContext /
// CanAccessAthlete / CanManageAthlete work without modification.
type MCPTokenAuth struct {
	db *sql.DB
	// prmResourceURL is the path-suffixed RFC 9728 Protected Resource Metadata
	// URL advertised in the WWW-Authenticate header on a 401, so a client can
	// discover the Authorization Server and begin the OAuth dance.
	prmResourceURL string
}

// NewMCPTokenAuth constructs the opaque-token middleware. prmResourceURL is the
// path-suffixed Protected Resource Metadata URL (e.g.
// "<origin>/.well-known/oauth-protected-resource/api/mcp"); it may be empty in
// tests, in which case no resource_metadata hint is emitted.
func NewMCPTokenAuth(db *sql.DB, prmResourceURL string) *MCPTokenAuth {
	return &MCPTokenAuth{db: db, prmResourceURL: prmResourceURL}
}

func (m *MCPTokenAuth) wwwAuthenticate(errCode string) string {
	v := `Bearer realm="replog", error="` + errCode + `"`
	if m.prmResourceURL != "" {
		v += `, resource_metadata="` + m.prmResourceURL + `"`
	}
	return v
}

// Middleware wraps a handler with opaque-token authentication.
func (m *MCPTokenAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" {
			writeBearerError(w, http.StatusUnauthorized, "missing-bearer-token",
				m.wwwAuthenticate("invalid_request"))
			return
		}

		// Prefix-gate before any DB work: a credential that isn't one of our
		// tokens is rejected without touching the database.
		if !strings.HasPrefix(token, models.MCPTokenPrefix) {
			writeBearerError(w, http.StatusUnauthorized, "invalid-token",
				m.wwwAuthenticate("invalid_token"))
			return
		}

		user, err := models.ValidateMCPToken(r.Context(), m.db, token)
		if errors.Is(err, models.ErrNotFound) {
			writeBearerError(w, http.StatusUnauthorized, "invalid-token",
				m.wwwAuthenticate("invalid_token"))
			return
		}
		if err != nil {
			log.Printf("middleware: mcp token validate: %v", err)
			writeBearerError(w, http.StatusInternalServerError, "lookup-failed", "")
			return
		}

		if !user.MCPEnabled {
			writeBearerError(w, http.StatusForbidden, "mcp-not-enabled", "")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)

		// Same defaults-on-error semantics as RequireAuth.
		prefs, err := models.GetUserPreferences(r.Context(), m.db, user.ID)
		if err != nil {
			log.Printf("middleware: mcp token prefs lookup for user %d: %v", user.ID, err)
			prefs = &models.UserPreferences{
				UserID:     user.ID,
				WeightUnit: models.DefaultWeightUnit,
				Timezone:   models.DefaultTimezone,
				DateFormat: models.DefaultDateFormat,
			}
		}
		ctx = context.WithValue(ctx, PrefsContextKey, prefs)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearer reads the Bearer credential from the Authorization header. The
// prefix match is case-insensitive; the returned value is trimmed.
func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// writeBearerError emits a JSON 4xx/5xx response with a stable `reason` slug
// the operator can pattern-match on. The optional wwwAuthenticate header is set
// per RFC 6750 / RFC 9728 to point the client at the Authorization Server.
func writeBearerError(w http.ResponseWriter, status int, reason, wwwAuthenticate string) {
	w.Header().Set("Content-Type", "application/json")
	if wwwAuthenticate != "" {
		w.Header().Set("WWW-Authenticate", wwwAuthenticate)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":  "unauthorized",
		"reason": reason,
		"code":   status,
	})
}
