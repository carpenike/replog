package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Athletes CRUD ---

// CreateAthlete creates a new athlete. Coach or admin only.
func (h *Handlers) CreateAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach or admin access required")
		return
	}

	var req struct {
		Name            string `json:"name"`
		Tier            string `json:"tier"`
		Notes           string `json:"notes"`
		Goal            string `json:"goal"`
		DateOfBirth     string `json:"date_of_birth"`
		Grade           string `json:"grade"`
		Gender          string `json:"gender"`
		TrackBodyWeight bool   `json:"track_body_weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	coachID := sql.NullInt64{Int64: user.ID, Valid: true}
	athlete, err := models.CreateAthlete(h.DB, req.Name, req.Tier, req.Notes, req.Goal, req.DateOfBirth, req.Grade, req.Gender, coachID, req.TrackBodyWeight)
	if err != nil {
		log.Printf("api: create athlete: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create athlete")
		return
	}

	WriteJSON(w, http.StatusCreated, AthleteFromModel(athlete))
}

// UpdateAthlete updates an athlete. Coach or admin only.
func (h *Handlers) UpdateAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
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

	if !middleware.CanManageAthlete(user, athlete) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req struct {
		Name            string `json:"name"`
		Tier            string `json:"tier"`
		Notes           string `json:"notes"`
		Goal            string `json:"goal"`
		DateOfBirth     string `json:"date_of_birth"`
		Grade           string `json:"grade"`
		Gender          string `json:"gender"`
		TrackBodyWeight bool   `json:"track_body_weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	updated, err := models.UpdateAthlete(h.DB, id, req.Name, req.Tier, req.Notes, req.Goal, req.DateOfBirth, req.Grade, req.Gender, athlete.CoachID, req.TrackBodyWeight)
	if err != nil {
		log.Printf("api: update athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to update athlete")
		return
	}

	WriteJSON(w, http.StatusOK, AthleteFromModel(updated))
}

// DeleteAthlete deletes an athlete. Coach or admin only.
func (h *Handlers) DeleteAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: get athlete %d for delete: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get athlete")
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	if err := models.DeleteAthlete(h.DB, id); err != nil {
		log.Printf("api: delete athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete athlete")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Exercises CRUD ---

// CreateExercise creates a new exercise. Coach or admin only.
func (h *Handlers) CreateExercise(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach or admin access required")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Tier        string `json:"tier"`
		FormNotes   string `json:"form_notes"`
		DemoURL     string `json:"demo_url"`
		RestSeconds int    `json:"rest_seconds"`
		Featured    bool   `json:"featured"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	exercise, err := models.CreateExercise(h.DB, req.Name, req.Tier, req.FormNotes, req.DemoURL, req.RestSeconds, req.Featured)
	if err != nil {
		log.Printf("api: create exercise: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create exercise")
		return
	}

	WriteJSON(w, http.StatusCreated, ExerciseFromModel(exercise))
}

// UpdateExercise updates an exercise. Coach or admin only.
func (h *Handlers) UpdateExercise(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach or admin access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Tier        string `json:"tier"`
		FormNotes   string `json:"form_notes"`
		DemoURL     string `json:"demo_url"`
		RestSeconds int    `json:"rest_seconds"`
		Featured    bool   `json:"featured"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	exercise, err := models.UpdateExercise(h.DB, id, req.Name, req.Tier, req.FormNotes, req.DemoURL, req.RestSeconds, req.Featured)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "exercise not found")
		return
	}
	if err != nil {
		log.Printf("api: update exercise %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to update exercise")
		return
	}

	WriteJSON(w, http.StatusOK, ExerciseFromModel(exercise))
}

// DeleteExercise deletes an exercise. Coach or admin only.
func (h *Handlers) DeleteExercise(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach or admin access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}

	if err := models.DeleteExercise(h.DB, id); err != nil {
		log.Printf("api: delete exercise %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete exercise")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Preferences ---

// UpdatePreferences updates the authenticated user's preferences.
func (h *Handlers) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var req struct {
		WeightUnit string `json:"weight_unit"`
		Timezone   string `json:"timezone"`
		DateFormat string `json:"date_format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	prefs, err := models.UpsertUserPreferences(h.DB, user.ID, req.WeightUnit, req.Timezone, req.DateFormat)
	if err != nil {
		log.Printf("api: update preferences for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	WriteJSON(w, http.StatusOK, UserPreferencesFromModel(prefs))
}
