package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/carpenike/replog/internal/models"
)

// OnBehalfCredentialEnv is the dedicated on-behalf secret. It is not
// REPLOG_SERVICE_TOKEN and it is not an rlcst_ bearer. Empty disables
// the on-behalf path.
const OnBehalfCredentialEnv = "REPLOG_ON_BEHALF_CREDENTIAL"

// OnBehalfOfHeader carries the caller's verified email. RequireAuth
// accepts it only together with a matching OnBehalfCredentialEnv value.
// Every other auth path deletes it before the handler runs.
const OnBehalfOfHeader = "X-RepLog-On-Behalf-Of"

// OnBehalfContextKey is set when this request authenticated with the
// on-behalf credential and a mapped user. It is not a service credential,
// and it is not the mapped user's full session either: the request runs as
// a copy of that user with IsAdmin cleared, and only the coach-data routes
// in onBehalfRouteAllowed are reachable (ADR 025).
const OnBehalfContextKey contextKey = "onBehalf"

// onBehalfAuditAction is the audit_log.action for a successful on-behalf
// authentication. Details stay a fixed flag so the credential and the
// header value are never written.
const onBehalfAuditAction = "on_behalf"

// LogOnBehalfConfig reports whether the on-behalf path is active. It logs
// the env name only, never the secret.
func LogOnBehalfConfig() {
	switch onBehalfMode() {
	case onBehalfDisabled:
		log.Printf("On-behalf auth disabled: %s is unset", OnBehalfCredentialEnv)
	case onBehalfReservedPrefix:
		log.Printf("On-behalf auth disabled: %s must not start with %s", OnBehalfCredentialEnv, models.ServiceTokenPrefix)
	default:
		log.Printf("On-behalf auth enabled")
	}
}

type onBehalfModeKind int

const (
	onBehalfDisabled onBehalfModeKind = iota
	onBehalfReservedPrefix
	onBehalfEnabled
)

func onBehalfMode() onBehalfModeKind {
	v := os.Getenv(OnBehalfCredentialEnv)
	if v == "" {
		return onBehalfDisabled
	}
	if strings.HasPrefix(v, models.ServiceTokenPrefix) {
		return onBehalfReservedPrefix
	}
	return onBehalfEnabled
}

// onBehalfCredentialMatches reports whether presented is the configured
// on-behalf secret. Both sides are hashed before the compare so the check
// stays constant-time for secrets of different lengths. An empty setting,
// or a value that uses the rlcst_ prefix, never matches.
func onBehalfCredentialMatches(presented string) bool {
	if onBehalfMode() != onBehalfEnabled || presented == "" {
		return false
	}
	expected := os.Getenv(OnBehalfCredentialEnv)
	sumP := sha256.Sum256([]byte(presented))
	sumE := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(sumP[:], sumE[:]) == 1
}

// authenticateOnBehalf maps the verified email to a user and attaches a
// scoped copy of that user. A missing or unknown email is 403. It does not
// fall back to a coach-service token, a session cookie, or any default
// user, and it does not write a session.
//
// The scope is enforced here, before any handler runs:
//   - Only the coach-data routes in onBehalfRouteAllowed are reachable.
//     Token issue and revoke, user and login-token management, admin
//     settings, catalog import, impersonation, preferences, avatars, and
//     logout are 403 whoever the mapped user is.
//   - The attached user has IsAdmin cleared, so an admin mapped user gets
//     coach-level access (athletes they coach, plus their own linked
//     athlete), never the admin-wide bypass in CanAccessAthlete,
//     CanManageAthlete, or CoachAthleteFilter.
func authenticateOnBehalf(w http.ResponseWriter, r *http.Request, db *sql.DB, email string, next http.Handler) {
	if !onBehalfRouteAllowed(r) {
		log.Printf("middleware: on-behalf rejected: route not available on behalf")
		writeAuthJSON(w, http.StatusForbidden, "not available on behalf of a user")
		return
	}
	if email == "" {
		log.Printf("middleware: on-behalf rejected: missing email")
		writeAuthJSON(w, http.StatusForbidden, "on-behalf email is required")
		return
	}
	user, err := models.GetUserByEmail(r.Context(), db, email)
	if errors.Is(err, models.ErrNotFound) {
		log.Printf("middleware: on-behalf rejected: email not mapped")
		writeAuthJSON(w, http.StatusForbidden, "no user for that email")
		return
	}
	if err != nil {
		log.Printf("middleware: on-behalf lookup failed")
		writeAuthJSON(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	if err := models.WriteAuditLog(r.Context(), db, user.ID, sql.NullInt64{}, onBehalfAuditAction, "on_behalf=true"); err != nil {
		log.Printf("middleware: on-behalf audit write failed")
	}
	log.Printf("middleware: on-behalf authenticated user id=%d on_behalf=true", user.ID)

	scoped := onBehalfUser(user)
	ctx := contextWithUser(r.Context(), db, scoped, false)
	ctx = context.WithValue(ctx, OnBehalfContextKey, true)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// onBehalfUser is the identity an on-behalf request runs as: a copy of the
// mapped user without admin rights. The stored row is not changed.
func onBehalfUser(user *models.User) *models.User {
	scoped := *user
	scoped.IsAdmin = false
	return &scoped
}

// onBehalfRouteAllowed is the allowlist for on-behalf requests. Anything not
// listed is refused, so a new route is closed to on-behalf callers until it
// is added here on purpose. Paths must be clean and unescaped, so a
// dot-segment or an encoded slash cannot reach a route this list would not
// match.
//
// Allowed:
//   - GET /api/me, GET /api/dashboard
//   - GET /api/athletes and GET /api/athletes/{id}
//   - any method under /api/athletes/{id}/... (sets, sessions, notes, coach
//     reads, coach exercises, program preview, import, assign, gap-fill).
//     Handlers still check that the caller coaches that athlete.
//   - GET under /api/exercises, /api/programs, /api/equipment
//   - GET /api/reviews/pending
func onBehalfRouteAllowed(r *http.Request) bool {
	p := r.URL.Path
	if r.URL.RawPath != "" || p == "" || path.Clean(p) != p || strings.Contains(p, "//") {
		return false
	}
	if !strings.HasPrefix(p, "/api/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(p, "/api/"), "/")
	get := r.Method == http.MethodGet || r.Method == http.MethodHead
	switch parts[0] {
	case "me", "dashboard":
		return get && len(parts) == 1
	case "athletes":
		if len(parts) <= 2 {
			return get
		}
		return parts[1] != "" && parts[2] != ""
	case "exercises", "programs", "equipment":
		return get
	case "reviews":
		return get && len(parts) == 2 && parts[1] == "pending"
	default:
		return false
	}
}

// IsOnBehalf reports whether this request authenticated via the on-behalf
// credential and a mapped user.
func IsOnBehalf(ctx context.Context) bool {
	v, _ := ctx.Value(OnBehalfContextKey).(bool)
	return v
}
