package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// CreateTrainingMax sets a new training max for an athlete+exercise.
//
//	@Summary      Set training max
//	@Tags         TrainingMaxes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                     true  "Athlete ID"
//	@Param        body  body      api.TrainingMaxRequest  true  "TM"
//	@Success      201  {object}  api.TrainingMax
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/training-maxes [post]
func (h *Handlers) CreateTrainingMax(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req TrainingMaxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 || req.Weight <= 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id and weight are required")
		return
	}

	tm, err := models.SetTrainingMax(h.DB, athleteID, req.ExerciseID, req.Weight, req.EffectiveDate, req.Notes)
	if err != nil {
		log.Printf("api: set training max for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to set training max")
		return
	}

	// Notify the athlete that a training max changed (ADR 008). Best-effort
	// exercise-name lookup for the title; falls back to ID if unavailable.
	exerciseName := fmt.Sprintf("exercise #%d", req.ExerciseID)
	if ex, eerr := models.GetExerciseByID(h.DB, req.ExerciseID); eerr == nil && ex != nil {
		exerciseName = ex.Name
	}
	h.notifyAthlete(athleteID, models.NotifyTMUpdated,
		"Training max updated",
		fmt.Sprintf("%s: %g lbs", exerciseName, req.Weight),
		fmt.Sprintf("/athletes/%d/training-maxes", athleteID))

	WriteJSON(w, http.StatusCreated, TrainingMaxFromModel(tm))
}

// UpdateWorkoutNotes updates the notes on a workout.
//
//	@Summary      Update workout notes
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path      int                       true  "Athlete ID"
//	@Param        workoutID  path      int                       true  "Workout ID"
//	@Param        body       body      api.WorkoutNotesRequest   true  "Notes"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/notes [put]
func (h *Handlers) UpdateWorkoutNotes(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	var req WorkoutNotesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := models.UpdateWorkoutNotes(h.DB, workoutID, req.Notes); err != nil {
		log.Printf("api: update workout %d notes: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update notes")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
