// Package api provides JSON API handlers for the RepLog REST API.
// These handlers wrap existing business logic from the models layer
// and return JSON responses using the DTO types defined in responses.go.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Handlers holds dependencies for API handlers.
type Handlers struct {
	DB       *sql.DB
	Sessions *scs.SessionManager
}

// Me returns the currently authenticated user.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	WriteJSON(w, http.StatusOK, UserFromModel(user))
}

// Login authenticates a user and creates a session.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := models.Authenticate(h.DB, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, models.ErrNoPassword) {
			WriteError(w, http.StatusForbidden, "this account uses passwordless login")
		} else {
			WriteError(w, http.StatusUnauthorized, "invalid username or password")
		}
		return
	}

	if err := h.Sessions.RenewToken(r.Context()); err != nil {
		log.Printf("api: session renew error: %v", err)
		WriteError(w, http.StatusInternalServerError, "session error")
		return
	}

	h.Sessions.Put(r.Context(), "userID", user.ID)

	if err := models.EnsureUserPreferences(h.DB, user.ID); err != nil {
		log.Printf("api: ensure preferences for user %d: %v", user.ID, err)
	}

	WriteJSON(w, http.StatusOK, UserFromModel(user))
}

// Logout destroys the session.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.Sessions.Destroy(r.Context()); err != nil {
		log.Printf("api: session destroy error: %v", err)
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListAthletes returns athlete cards for the authenticated user.
func (h *Handlers) ListAthletes(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	coachFilter := middleware.CoachAthleteFilter(user)

	athletes, err := models.ListAthleteCards(h.DB, coachFilter)
	if err != nil {
		log.Printf("api: list athlete cards: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list athletes")
		return
	}

	result := make([]*AthleteCard, len(athletes))
	for i, a := range athletes {
		result[i] = AthleteCardFromModel(a)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetAthlete returns a single athlete by ID.
func (h *Handlers) GetAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, id) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: get athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get athlete")
		return
	}

	WriteJSON(w, http.StatusOK, AthleteFromModel(athlete))
}

// ListExercises returns the exercise catalog.
func (h *Handlers) ListExercises(w http.ResponseWriter, r *http.Request) {
	tierFilter := r.URL.Query().Get("tier")

	exercises, err := models.ListExercises(h.DB, tierFilter)
	if err != nil {
		log.Printf("api: list exercises: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list exercises")
		return
	}

	result := make([]*Exercise, len(exercises))
	for i, e := range exercises {
		result[i] = ExerciseFromModel(e)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetExercise returns a single exercise by ID.
func (h *Handlers) GetExercise(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	exercise, err := models.GetExerciseByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "exercise not found")
		return
	}
	if err != nil {
		log.Printf("api: get exercise %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get exercise")
		return
	}

	WriteJSON(w, http.StatusOK, ExerciseFromModel(exercise))
}

// ListWorkouts returns paginated workouts for an athlete.
func (h *Handlers) ListWorkouts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, id) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	page, err := models.ListWorkouts(h.DB, id, offset)
	if err != nil {
		log.Printf("api: list workouts for athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to list workouts")
		return
	}

	WriteJSON(w, http.StatusOK, WorkoutPageFromModel(page))
}

// GetPreferences returns the authenticated user's preferences.
func (h *Handlers) GetPreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	prefs, err := models.GetUserPreferences(h.DB, user.ID)
	if err != nil {
		log.Printf("api: get preferences for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get preferences")
		return
	}
	WriteJSON(w, http.StatusOK, UserPreferencesFromModel(prefs))
}
