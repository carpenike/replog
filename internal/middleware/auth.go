package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/models"
)

type contextKey string

// UserContextKey is exported for use in handler tests that need to inject
// an authenticated user into the request context.
const UserContextKey contextKey = "user"

// PrefsContextKey stores the user's preferences in request context.
const PrefsContextKey contextKey = "prefs"

// UnreadCountContextKey stores the user's unread notification count in request context.
const UnreadCountContextKey contextKey = "unreadCount"

// ServiceCredentialContextKey is set when the caller authenticated with a
// coach service token rather than a browser session. Handlers that assign
// a program, execute a generation, write a training max, or promote an
// athlete refuse that credential. A session of the same user does not
// carry the key, so the browser can still approve.
const ServiceCredentialContextKey contextKey = "serviceCredential"

// writeAuthJSON writes a JSON error without depending on the api package
// (that import would be circular: api imports middleware).
func writeAuthJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
		"code":  status,
	})
}

// writeUnauthorizedJSON writes a 401 JSON response.
func writeUnauthorizedJSON(w http.ResponseWriter, msg string) {
	writeAuthJSON(w, http.StatusUnauthorized, msg)
}

// extractBearer returns the token from an Authorization: Bearer header, or
// empty when the header is absent or not a bearer credential. REST auth
// uses this to accept rlcst_ service tokens and to reject any other bearer,
// including a leftover rlpat_ MCP token, before falling through to a cookie.
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

// RequireAuth gates a handler chain on a browser session or a coach service
// token. Unauthenticated requests get a 401 JSON response — every route this
// guards lives under /api/*, so an XHR-friendly response is the right answer.
//
// A Bearer credential is tried first and is never written into the session.
// Only an rlcst_ service token is accepted; an MCP token (rlpat_) or any
// other bearer is 401, not a fall-through to a cookie that might be sitting
// on the same request. Absent an Authorization header, the scs session is
// the caller, and that path does not set ServiceCredentialContextKey.
func RequireAuth(sm *scs.SessionManager, db *sql.DB, next http.Handler) http.Handler {
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if presented := extractBearer(r); presented != "" {
			authenticateServiceToken(w, r, db, presented, next)
			return
		}

		userID := sm.GetInt64(r.Context(), "userID")
		if userID == 0 {
			writeUnauthorizedJSON(w, "not authenticated")
			return
		}

		user, err := models.GetUserByID(r.Context(), db, userID)
		if err != nil {
			log.Printf("middleware: failed to load user %d: %v", userID, err)
			_ = sm.Destroy(r.Context())
			writeUnauthorizedJSON(w, "session is no longer valid")
			return
		}

		// NOTE: the unread notification count is deliberately NOT loaded here.
		// It was a third query on every authenticated request (including the
		// SPA's ~2s notification polling) but nothing read it back out of the
		// context — the SPA fetches it from the dedicated GET
		// /api/notifications/count endpoint instead. Dropping it removes a
		// per-request DB round-trip that, under SetMaxOpenConns(1), serialized
		// behind every other query.

		next.ServeHTTP(w, r.WithContext(contextWithUser(r.Context(), db, user, false)))
	})

	// LoadAndSave puts a session in the context (Me reads the impersonation
	// key). It writes a cookie only when the session is modified. The bearer
	// path never writes userID, so presenting the token does not mint a
	// browser session that could approve a coaching change.
	return sm.LoadAndSave(authed)
}

// authenticateServiceToken resolves an rlcst_ bearer and attaches the user
// with the service-credential flag. It does not create a session.
func authenticateServiceToken(w http.ResponseWriter, r *http.Request, db *sql.DB, presented string, next http.Handler) {
	if !strings.HasPrefix(presented, models.ServiceTokenPrefix) {
		writeUnauthorizedJSON(w, "invalid token")
		return
	}
	user, err := models.ValidateServiceToken(r.Context(), db, presented)
	if errors.Is(err, models.ErrNotFound) {
		writeUnauthorizedJSON(w, "invalid token")
		return
	}
	if err != nil {
		log.Printf("middleware: service token validate: %v", err)
		writeAuthJSON(w, http.StatusInternalServerError, "authentication failed")
		return
	}
	next.ServeHTTP(w, r.WithContext(contextWithUser(r.Context(), db, user, true)))
}

// contextWithUser attaches the user, their preferences, and — when the
// caller is a service token — the coaching-refusal flag.
func contextWithUser(ctx context.Context, db *sql.DB, user *models.User, service bool) context.Context {
	ctx = context.WithValue(ctx, UserContextKey, user)
	prefs, err := models.GetUserPreferences(ctx, db, user.ID)
	if err != nil {
		log.Printf("middleware: failed to load preferences for user %d: %v", user.ID, err)
		prefs = &models.UserPreferences{
			UserID:     user.ID,
			WeightUnit: models.DefaultWeightUnit,
			Timezone:   models.DefaultTimezone,
			DateFormat: models.DefaultDateFormat,
		}
	}
	ctx = context.WithValue(ctx, PrefsContextKey, prefs)
	if service {
		ctx = context.WithValue(ctx, ServiceCredentialContextKey, true)
	}
	return ctx
}

// IsServiceCredential reports whether this request authenticated with a
// coach service token. A browser session returns false.
func IsServiceCredential(ctx context.Context) bool {
	v, _ := ctx.Value(ServiceCredentialContextKey).(bool)
	return v
}

// UserFromContext retrieves the authenticated user from the request context.
// Returns nil if no user is set (should not happen behind RequireAuth).
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(UserContextKey).(*models.User)
	return u
}

// PrefsFromContext retrieves the user's preferences from the request context.
// Returns nil if no preferences are set.
func PrefsFromContext(ctx context.Context) *models.UserPreferences {
	p, _ := ctx.Value(PrefsContextKey).(*models.UserPreferences)
	return p
}

// CanAccessAthlete checks whether the authenticated user is allowed to access
// the given athlete. Admins can access any athlete; coaches can access athletes
// assigned to them; non-coaches can only access their own linked athlete.
// Loads the athlete from the database to verify coach ownership.
func CanAccessAthlete(ctx context.Context, db *sql.DB, user *models.User, athleteID int64) bool {
	if user.IsAdmin {
		return true
	}
	// Own linked athlete profile.
	if user.AthleteID.Valid && user.AthleteID.Int64 == athleteID {
		return true
	}
	if user.IsCoach {
		athlete, err := models.GetAthleteByID(ctx, db, athleteID)
		if err != nil {
			return false
		}
		return athlete.CoachID.Valid && athlete.CoachID.Int64 == user.ID
	}
	return false
}

// CanManageAthlete checks whether the user can manage (edit/delete/assign) the
// given athlete. Admins can manage any athlete. Coaches can only manage athletes
// where athlete.CoachID matches the user's ID.
func CanManageAthlete(user *models.User, athlete *models.Athlete) bool {
	if user.IsAdmin {
		return true
	}
	if user.IsCoach {
		return athlete.CoachID.Valid && athlete.CoachID.Int64 == user.ID
	}
	return false
}

// ServiceCanAccessAthlete is the visibility of an rlcst_ token. It is the
// bound user's own access and does not grant the admin-wide bypass. A coach
// sees athletes with athletes.coach_id equal to that user. Any user also
// sees the athlete linked on their account, which is the whole of an
// athlete user's view.
func ServiceCanAccessAthlete(ctx context.Context, db *sql.DB, user *models.User, athleteID int64) bool {
	if user == nil || athleteID <= 0 {
		return false
	}
	if user.AthleteID.Valid && user.AthleteID.Int64 == athleteID {
		return true
	}
	if !user.IsCoach {
		return false
	}
	athlete, err := models.GetAthleteByID(ctx, db, athleteID)
	if err != nil {
		return false
	}
	return athlete.CoachID.Valid && athlete.CoachID.Int64 == user.ID
}

// CoachAthleteFilter returns the coach ID to use for filtering athlete lists.
// Admins get sql.NullInt64{} (invalid = no filter, see all athletes).
// Coaches get their own user ID as the filter.
func CoachAthleteFilter(user *models.User) sql.NullInt64 {
	if user.IsAdmin {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: user.ID, Valid: true}
}
