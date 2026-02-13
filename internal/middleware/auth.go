package middleware

import (
	"context"
	"database/sql"
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

// RequireAuth redirects unauthenticated users to the login page.
func RequireAuth(sm *scs.SessionManager, db *sql.DB, next http.Handler) http.Handler {
	return sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := sm.GetInt64(r.Context(), "userID")
		if userID == 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := models.GetUserByID(db, userID)
		if err != nil {
			log.Printf("middleware: failed to load user %d: %v", userID, err)
			sm.Destroy(r.Context())
			http.Redirect(w, r, "/login", http.StatusSeeOther)
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

// CanAccessAthlete checks whether the authenticated user is allowed to access
// the given athlete. Coaches can access any athlete; non-coaches can only access
// their own linked athlete. Returns true if access is allowed.
func CanAccessAthlete(user *models.User, athleteID int64) bool {
	if user.IsCoach {
		return true
	}
	return user.AthleteID.Valid && user.AthleteID.Int64 == athleteID
}

// RequireCoach returns 403 if the user is not a coach.
func RequireCoach(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || !user.IsCoach {
			http.Error(w, "Forbidden — coach access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
