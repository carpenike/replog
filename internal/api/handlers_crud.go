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
//
//	@Summary      Create athlete
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.AthleteRequest  true  "Athlete"
//	@Success      201  {object}  api.Athlete
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes [post]
func (h *Handlers) CreateAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach or admin access required")
		return
	}

	var req AthleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !validateAthleteFields(w, &req) {
		return
	}

	coachID := sql.NullInt64{Int64: user.ID, Valid: true}
	athlete, err := models.CreateAthlete(r.Context(), h.DB, req.Name, req.Tier, req.Notes, req.Goal, req.DateOfBirth, req.Grade, req.Gender, coachID, req.TrackBodyWeight)
	if err != nil {
		log.Printf("api: create athlete: %v", err)
		WriteDBError(w, err, "failed to create athlete")
		return
	}

	WriteJSON(w, http.StatusCreated, AthleteFromModel(athlete))
}

// UpdateAthlete updates an athlete. Coach or admin only.
//
//	@Summary      Update athlete
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                 true  "Athlete ID"
//	@Param        body  body      api.AthleteRequest  true  "Athlete"
//	@Success      200  {object}  api.Athlete
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id} [put]
func (h *Handlers) UpdateAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	athlete, err := models.GetAthleteByID(r.Context(), h.DB, id)
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

	var req AthleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !validateAthleteFields(w, &req) {
		return
	}

	updated, err := models.UpdateAthlete(r.Context(), h.DB, id, req.Name, req.Tier, req.Notes, req.Goal, req.DateOfBirth, req.Grade, req.Gender, athlete.CoachID, req.TrackBodyWeight)
	if err != nil {
		log.Printf("api: update athlete %d: %v", id, err)
		WriteDBError(w, err, "failed to update athlete")
		return
	}

	WriteJSON(w, http.StatusOK, AthleteFromModel(updated))
}

// DeleteAthlete deletes an athlete. Coach or admin only.
//
//	@Summary      Delete athlete
//	@Tags         Athletes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id} [delete]
func (h *Handlers) DeleteAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	athlete, err := models.GetAthleteByID(r.Context(), h.DB, id)
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

	if err := models.DeleteAthlete(r.Context(), h.DB, id); err != nil {
		log.Printf("api: delete athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete athlete")
		return
	}

	WriteJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

// --- Exercises CRUD ---

// CreateExercise creates a new exercise. Coach or admin only.
//
//	@Summary      Create exercise
//	@Tags         Exercises
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.ExerciseRequest  true  "Exercise"
//	@Success      201  {object}  api.Exercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /exercises [post]
func (h *Handlers) CreateExercise(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach or admin access required")
		return
	}

	var req ExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	exercise, err := models.CreateExercise(r.Context(), h.DB, req.Name, req.Tier, req.FormNotes, req.DemoURL, req.RestSeconds, req.Featured)
	if err != nil {
		log.Printf("api: create exercise: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create exercise")
		return
	}

	WriteJSON(w, http.StatusCreated, ExerciseFromModel(exercise))
}

// UpdateExercise updates an exercise. Coach or admin only.
//
//	@Summary      Update exercise
//	@Tags         Exercises
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                  true  "Exercise ID"
//	@Param        body  body      api.ExerciseRequest  true  "Exercise"
//	@Success      200  {object}  api.Exercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /exercises/{id} [put]
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

	var req ExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	exercise, err := models.UpdateExercise(r.Context(), h.DB, id, req.Name, req.Tier, req.FormNotes, req.DemoURL, req.RestSeconds, req.Featured)
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
//
//	@Summary      Delete exercise
//	@Tags         Exercises
//	@Produce      json
//	@Param        id   path      int  true  "Exercise ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /exercises/{id} [delete]
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

	if err := models.DeleteExercise(r.Context(), h.DB, id); err != nil {
		log.Printf("api: delete exercise %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete exercise")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Preferences ---

// UpdatePreferences updates the authenticated user's preferences.
//
//	@Summary      Update preferences
//	@Tags         Preferences
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.PreferencesRequest  true  "Preferences"
//	@Success      200  {object}  api.UserPreferences
//	@Failure      400  {object}  api.APIError
//	@Failure      401  {object}  api.APIError
//	@Router       /preferences [put]
func (h *Handlers) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var req PreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	prefs, err := models.UpsertUserPreferences(r.Context(), h.DB, user.ID, req.WeightUnit, req.Timezone, req.DateFormat)
	if err != nil {
		log.Printf("api: update preferences for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	WriteJSON(w, http.StatusOK, UserPreferencesFromModel(prefs))
}
