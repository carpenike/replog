package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

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

// writeUnauthorizedJSON writes a 401 JSON response. Kept local to this file
// so the middleware package has no dependency on the api package (which would
// be circular: api imports middleware).
func writeUnauthorizedJSON(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
		"code":  http.StatusUnauthorized,
	})
}

// RequireAuth gates a handler chain on having an authenticated session.
// Unauthenticated requests get a 401 JSON response — every route this guards
// today lives under /api/*, so an XHR-friendly response is the right answer.
// (The earlier 303 → /login behavior was carried over from the SSR era and
// caused fetch() callers to receive HTML when expecting JSON.)
func RequireAuth(sm *scs.SessionManager, db *sql.DB, next http.Handler) http.Handler {
	return sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := sm.GetInt64(r.Context(), "userID")
		if userID == 0 {
			writeUnauthorizedJSON(w, "not authenticated")
			return
		}

		user, err := models.GetUserByID(db, userID)
		if err != nil {
			log.Printf("middleware: failed to load user %d: %v", userID, err)
			_ = sm.Destroy(r.Context())
			writeUnauthorizedJSON(w, "session is no longer valid")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)

		// Load user preferences (defaults returned if no row exists).
		prefs, err := models.GetUserPreferences(db, user.ID)
		if err != nil {
			log.Printf("middleware: failed to load preferences for user %d: %v", userID, err)
			// Non-fatal — use defaults.
			prefs = &models.UserPreferences{
				UserID:     user.ID,
				WeightUnit: models.DefaultWeightUnit,
				Timezone:   models.DefaultTimezone,
				DateFormat: models.DefaultDateFormat,
			}
		}
		ctx = context.WithValue(ctx, PrefsContextKey, prefs)

		// Load unread notification count for the sidebar badge.
		unreadCount, err := models.GetUnreadCount(db, user.ID)
		if err != nil {
			log.Printf("middleware: failed to load unread count for user %d: %v", userID, err)
			// Non-fatal — default to 0.
		}
		ctx = context.WithValue(ctx, UnreadCountContextKey, unreadCount)

		next.ServeHTTP(w, r.WithContext(ctx))
	}))
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

// UnreadCountFromContext retrieves the user's unread notification count from context.
// Returns 0 if not set.
func UnreadCountFromContext(ctx context.Context) int {
	count, _ := ctx.Value(UnreadCountContextKey).(int)
	return count
}

// CanAccessAthlete checks whether the authenticated user is allowed to access
// the given athlete. Admins can access any athlete; coaches can access athletes
// assigned to them; non-coaches can only access their own linked athlete.
// Loads the athlete from the database to verify coach ownership.
func CanAccessAthlete(db *sql.DB, user *models.User, athleteID int64) bool {
	if user.IsAdmin {
		return true
	}
	// Own linked athlete profile.
	if user.AthleteID.Valid && user.AthleteID.Int64 == athleteID {
		return true
	}
	if user.IsCoach {
		athlete, err := models.GetAthleteByID(db, athleteID)
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

// CoachAthleteFilter returns the coach ID to use for filtering athlete lists.
// Admins get sql.NullInt64{} (invalid = no filter, see all athletes).
// Coaches get their own user ID as the filter.
func CoachAthleteFilter(user *models.User) sql.NullInt64 {
	if user.IsAdmin {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: user.ID, Valid: true}
}
