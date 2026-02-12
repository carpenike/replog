package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/models"
)

// Auth holds dependencies for authentication handlers.
type Auth struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	Templates *template.Template
}

// LoginPage renders the login form.
func (a *Auth) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to home.
	if a.Sessions.GetInt64(r.Context(), "userID") != 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := map[string]any{
		"Error": r.URL.Query().Get("error"),
	}
	if err := a.Templates.ExecuteTemplate(w, "login", data); err != nil {
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
		http.Redirect(w, r, "/login?error=Username+and+password+are+required", http.StatusSeeOther)
		return
	}

	user, err := models.Authenticate(a.DB, username, password)
	if err != nil {
		log.Printf("handlers: login failed for %q: %v", username, err)
		http.Redirect(w, r, "/login?error=Invalid+username+or+password", http.StatusSeeOther)
		return
	}

	// Renew session token to prevent fixation.
	if err := a.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("handlers: session renew error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	a.Sessions.Put(r.Context(), "userID", user.ID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout destroys the session and redirects to login.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if err := a.Sessions.Destroy(r.Context()); err != nil {
		log.Printf("handlers: session destroy error: %v", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
