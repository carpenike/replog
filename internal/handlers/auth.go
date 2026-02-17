package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/models"
)

// Auth holds dependencies for authentication handlers.
type Auth struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	Templates TemplateCache
}

// LoginPage renders the login form.
func (a *Auth) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to home.
	if a.Sessions.GetInt64(r.Context(), "userID") != 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Pop flash error from session (set during failed login).
	errorMsg := a.Sessions.PopString(r.Context(), "flash_error")

	data := map[string]any{
		"Error": errorMsg,
	}
	if err := a.Templates["login.html"].ExecuteTemplate(w, "login", data); err != nil {
		log.Printf("handlers: login template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// LoginSubmit processes the login form.
func (a *Auth) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		a.Sessions.Put(r.Context(), "flash_error", "Username and password are required")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := models.Authenticate(a.DB, username, password)
	if err != nil {
		if errors.Is(err, models.ErrNoPassword) {
			log.Printf("handlers: passwordless user %q attempted password login", username)
			a.Sessions.Put(r.Context(), "flash_error", "This account uses passwordless login. Use your login link or passkey.")
		} else {
			log.Printf("handlers: login failed for %q: %v", username, err)
			a.Sessions.Put(r.Context(), "flash_error", "Invalid username or password")
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Renew session token to prevent fixation.
	if err := a.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("handlers: session renew error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	a.Sessions.Put(r.Context(), "userID", user.ID)

	// Ensure default preferences exist for this user.
	if err := models.EnsureUserPreferences(a.DB, user.ID); err != nil {
		log.Printf("handlers: ensure preferences for user %d: %v", user.ID, err)
		// Non-fatal — continue with login.
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout destroys the session and redirects to login.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if err := a.Sessions.Destroy(r.Context()); err != nil {
		log.Printf("handlers: session destroy error: %v", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
