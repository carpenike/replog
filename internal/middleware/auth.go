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

// UnreadCountContextKey stores the user's unread notification count in request context.
const UnreadCountContextKey contextKey = "unreadCount"

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

// ErrorRenderer is a function that renders a styled error page. Middleware
// accepts this as a parameter to avoid importing the handlers package.
type ErrorRenderer func(w http.ResponseWriter, r *http.Request, status int, title, message string)

// RequireCoach returns 403 if the user is not a coach or admin.
// If onError is nil, falls back to plain text http.Error.
func RequireCoach(onError ErrorRenderer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || (!user.IsCoach && !user.IsAdmin) {
			if onError != nil {
				onError(w, r, http.StatusForbidden, "Access Denied", "You need coach or admin permissions to access this page. Please contact your administrator if you believe this is an error.")
			} else {
				http.Error(w, "Forbidden — coach access required", http.StatusForbidden)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin returns 403 if the user is not an admin.
// If onError is nil, falls back to plain text http.Error.
func RequireAdmin(onError ErrorRenderer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			if onError != nil {
				onError(w, r, http.StatusForbidden, "Access Denied", "You need admin permissions to access this page. Please contact your administrator if you believe this is an error.")
			} else {
				http.Error(w, "Forbidden — admin access required", http.StatusForbidden)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
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
