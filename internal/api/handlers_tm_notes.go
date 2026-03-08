package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// CreateTrainingMax sets a new training max for an athlete+exercise.
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

	var req struct {
		ExerciseID    int64   `json:"exercise_id"`
		Weight        float64 `json:"weight"`
		EffectiveDate string  `json:"effective_date"`
		Notes         string  `json:"notes"`
	}
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

	WriteJSON(w, http.StatusCreated, TrainingMaxFromModel(tm))
}

// UpdateWorkoutNotes updates the notes on a workout.
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

	var req struct {
		Notes string `json:"notes"`
	}
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
