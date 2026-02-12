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

const userContextKey contextKey = "user"

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

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

// UserFromContext retrieves the authenticated user from the request context.
// Returns nil if no user is set (should not happen behind RequireAuth).
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}
