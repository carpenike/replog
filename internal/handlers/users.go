package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Users handles user management (coach-only).
type Users struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders all users.
func (h *Users) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	users, err := models.ListUsers(h.DB)
	if err != nil {
		log.Printf("handlers: list users: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Users": users,
	}
	if err := h.Templates.Render(w, r, "users_list.html", data); err != nil {
		log.Printf("handlers: render users list: %v", err)
	}
}

// NewForm renders the create-user form.
func (h *Users) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	athletes, err := models.ListAthletes(h.DB)
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
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	email := r.FormValue("email")
	isCoach := r.FormValue("is_coach") == "1"

	if username == "" || password == "" {
		h.renderFormError(w, r, "Username and password are required.", nil)
		return
	}
	if len(password) < 6 {
		h.renderFormError(w, r, "Password must be at least 6 characters.", nil)
		return
	}

	var athleteID sql.NullInt64
	if aidStr := r.FormValue("athlete_id"); aidStr != "" {
		aid, err := strconv.ParseInt(aidStr, 10, 64)
		if err == nil {
			athleteID = sql.NullInt64{Int64: aid, Valid: true}
		}
	}

	_, err := models.CreateUser(h.DB, username, password, email, isCoach, athleteID)
	if errors.Is(err, models.ErrDuplicateUsername) {
		h.renderFormError(w, r, "Username already taken.", nil)
		return
	}
	if err != nil {
		log.Printf("handlers: create user: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// EditForm renders the edit-user form.
func (h *Users) EditForm(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
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

	athletes, err := models.ListAthletes(h.DB)
	if err != nil {
		log.Printf("handlers: list athletes for user form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"EditUser": u,
		"Athletes": athletes,
	}
	if err := h.Templates.Render(w, r, "user_form.html", data); err != nil {
		log.Printf("handlers: render edit user form: %v", err)
	}
}

// Update handles user profile update.
func (h *Users) Update(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.UserFromContext(r.Context())
	if !authUser.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
	email := r.FormValue("email")
	isCoach := r.FormValue("is_coach") == "1"

	if username == "" {
		h.renderFormError(w, r, "Username is required.", u)
		return
	}

	// Validate password early — before committing profile changes.
	newPassword := r.FormValue("password")
	if newPassword != "" && len(newPassword) < 6 {
		h.renderFormError(w, r, "Password must be at least 6 characters.", u)
		return
	}

	var athleteID sql.NullInt64
	athleteIDStr := r.FormValue("athlete_id")
	if athleteIDStr != "" {
		aid, err := strconv.ParseInt(athleteIDStr, 10, 64)
		if err == nil {
			athleteID = sql.NullInt64{Int64: aid, Valid: true}
		}
	}

	_, err = models.UpdateUser(h.DB, id, username, email, athleteID, isCoach)
	if errors.Is(err, models.ErrDuplicateUsername) {
		h.renderFormError(w, r, "Username already taken.", u)
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
	if !authUser.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
	athletes, _ := models.ListAthletes(h.DB)
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
