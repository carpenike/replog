package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Users handles user management (admin-only).
type Users struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	Templates TemplateCache
	BaseURL   string // External base URL for login links. If empty, inferred from request.
}

// List renders all users.
func (h *Users) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	users, err := models.ListUsers(h.DB)
	if err != nil {
		log.Printf("handlers: list users: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Pop flash data from session (set after passwordless user creation).
	flashSuccess := ""
	flashTokenURL := ""
	if h.Sessions != nil {
		flashSuccess = h.Sessions.PopString(r.Context(), "flash_success")
		flashTokenURL = h.Sessions.PopString(r.Context(), "flash_token_url")
	}

	data := map[string]any{
		"Users":        users,
		"Success":      flashSuccess,
		"NewTokenURL":  flashTokenURL,
	}
	if err := h.Templates.Render(w, r, "users_list.html", data); err != nil {
		log.Printf("handlers: render users list: %v", err)
	}
}

// NewForm renders the create-user form.
func (h *Users) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	athletes, err := models.ListAvailableAthletes(h.DB, 0)
	if err != nil {
		log.Printf("handlers: list athletes for user form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athletes": athletes,
	}
	if err := h.Templates.Render(w, r, "user_form.html", data); err != nil {
		log.Printf("handlers: render new user form: %v", err)
	}
}

// Create handles user creation.
func (h *Users) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	username := r.FormValue("username")
	name := r.FormValue("name")
	password := r.FormValue("password")
	email := r.FormValue("email")
	isCoach := r.FormValue("is_coach") == "1"
	isAdmin := r.FormValue("is_admin") == "1"

	if username == "" {
		h.renderFormError(w, r, "Username is required.", nil)
		return
	}
	if password != "" && len(password) < 6 {
		h.renderFormError(w, r, "Password must be at least 6 characters.", nil)
		return
	}

	var athleteID sql.NullInt64
	aidStr := r.FormValue("athlete_id")
	if aidStr == "__new__" {
		// Inline athlete creation. Default to username if no name provided.
		newAthleteName := r.FormValue("new_athlete_name")
		if newAthleteName == "" {
			newAthleteName = strings.ToUpper(username[:1]) + username[1:]
		}
		athlete, err := models.CreateAthlete(h.DB, newAthleteName, "", "", "", sql.NullInt64{}, true)
		if err != nil {
			log.Printf("handlers: inline create athlete: %v", err)
			h.renderFormError(w, r, "Failed to create athlete.", nil)
			return
		}
		athleteID = sql.NullInt64{Int64: athlete.ID, Valid: true}
	} else if aidStr != "" {
		aid, err := strconv.ParseInt(aidStr, 10, 64)
		if err == nil {
			athleteID = sql.NullInt64{Int64: aid, Valid: true}
		}
	}

	newUser, err := models.CreateUser(h.DB, username, name, password, email, isCoach, isAdmin, athleteID)
	if errors.Is(err, models.ErrDuplicateUsername) {
		h.renderFormError(w, r, "Username already taken.", nil)
		return
	}
	if errors.Is(err, models.ErrAthleteAlreadyLinked) {
		h.renderFormError(w, r, "This athlete is already linked to another user.", nil)
		return
	}
	if err != nil {
		log.Printf("handlers: create user: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// For passwordless users, auto-generate a login token and flash the URL.
	if !newUser.HasPassword() && h.Sessions != nil {
		label := r.FormValue("token_label")
		if label == "" {
			label = "auto"
		}
		lt, err := models.CreateLoginToken(h.DB, newUser.ID, label, nil)
		if err != nil {
			log.Printf("handlers: auto-create token for user %d: %v", newUser.ID, err)
		} else {
			var loginURL string
			if h.BaseURL != "" {
				loginURL = fmt.Sprintf("%s/auth/token/%s", h.BaseURL, lt.Token)
			} else {
				scheme := "http"
				if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
					scheme = "https"
				}
				loginURL = fmt.Sprintf("%s://%s/auth/token/%s", scheme, r.Host, lt.Token)
			}
			h.Sessions.Put(r.Context(), "flash_success",
				fmt.Sprintf("User %q created successfully. Copy the login link below.", newUser.Username))
			h.Sessions.Put(r.Context(), "flash_token_url", loginURL)
		}
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// EditForm renders the edit-user form.
func (h *Users) EditForm(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	u, err := models.GetUserByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get user %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	athletes, err := models.ListAvailableAthletes(h.DB, u.AthleteID.Int64)
	if err != nil {
		log.Printf("handlers: list athletes for user form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tokens, err := models.ListLoginTokensByUser(h.DB, u.ID)
	if err != nil {
		log.Printf("handlers: list login tokens for user %d: %v", u.ID, err)
		// Non-fatal — render without tokens.
		tokens = nil
	}

	data := map[string]any{
		"EditUser": u,
		"Athletes": athletes,
		"Tokens":   tokens,
	}
	if err := h.Templates.Render(w, r, "user_form.html", data); err != nil {
		log.Printf("handlers: render edit user form: %v", err)
	}
}

// Update handles user profile update.
func (h *Users) Update(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	u, err := models.GetUserByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get user %d for update: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	username := r.FormValue("username")
	name := r.FormValue("name")
	email := r.FormValue("email")
	isCoach := r.FormValue("is_coach") == "1"
	isAdmin := r.FormValue("is_admin") == "1"

	if username == "" {
		h.renderFormError(w, r, "Username is required.", u)
		return
	}

	// Validate password early — before committing profile changes.
	// Block password changes for passwordless users.
	newPassword := r.FormValue("password")
	if newPassword != "" {
		if !u.HasPassword() {
			h.renderFormError(w, r, "This account uses passwordless login. Password cannot be set.", u)
			return
		}
		if len(newPassword) < 6 {
			h.renderFormError(w, r, "Password must be at least 6 characters.", u)
			return
		}
	}

	var athleteID sql.NullInt64
	athleteIDStr := r.FormValue("athlete_id")
	if athleteIDStr != "" {
		aid, err := strconv.ParseInt(athleteIDStr, 10, 64)
		if err == nil {
			athleteID = sql.NullInt64{Int64: aid, Valid: true}
		}
	}

	_, err = models.UpdateUser(h.DB, id, username, name, email, athleteID, isCoach, isAdmin)
	if errors.Is(err, models.ErrDuplicateUsername) {
		h.renderFormError(w, r, "Username already taken.", u)
		return
	}
	if errors.Is(err, models.ErrAthleteAlreadyLinked) {
		h.renderFormError(w, r, "This athlete is already linked to another user.", u)
		return
	}
	if err != nil {
		log.Printf("handlers: update user %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Update password if provided (already validated above).
	if newPassword != "" {
		if err := models.UpdatePassword(h.DB, id, newPassword); err != nil {
			log.Printf("handlers: update password for user %d: %v", id, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// Delete handles user deletion.
func (h *Users) Delete(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Prevent deleting yourself.
	if id == authUser.ID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	if err := models.DeleteUser(h.DB, id); err != nil {
		log.Printf("handlers: delete user %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// renderFormError re-renders the user form with an error message.
func (h *Users) renderFormError(w http.ResponseWriter, r *http.Request, msg string, u *models.User) {
	var exceptAthleteID int64
	if u != nil && u.AthleteID.Valid {
		exceptAthleteID = u.AthleteID.Int64
	}
	athletes, err := models.ListAvailableAthletes(h.DB, exceptAthleteID)
	if err != nil {
		log.Printf("handlers: list available athletes: %v", err)
	}
	data := map[string]any{
		"Error":    msg,
		"EditUser": u,
		"Athletes": athletes,
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := h.Templates.Render(w, r, "user_form.html", data); err != nil {
		log.Printf("handlers: render user form error: %v", err)
	}
}
