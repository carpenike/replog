package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/carpenike/replog/internal/models"
)

// ownedWorkout loads the {workoutID} path workout and checks it belongs to
// the path athlete (ADR 021). It writes the error response and returns
// false when the workout is missing or belongs to someone else.
func (h *Handlers) ownedWorkout(w http.ResponseWriter, r *http.Request, athleteID int64) (*models.Workout, bool) {
	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return nil, false
	}
	workout, err := models.GetWorkoutByID(r.Context(), h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) || (err == nil && workout.AthleteID != athleteID) {
		WriteError(w, http.StatusNotFound, "workout not found")
		return nil, false
	}
	if err != nil {
		log.Printf("api: get workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get workout")
		return nil, false
	}
	return workout, true
}

// ListExerciseSubstitutes lists exercises an athlete can swap in for a
// prescribed one: those sharing at least one movement pattern.
//
//	@Summary      List exercise substitutes
//	@Description  Exercises that share a movement pattern with the given exercise, most shared patterns first. An exercise with no pattern tags has no substitutes. Swapping never changes the program.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id          path      int  true  "Athlete ID"
//	@Param        exerciseID  path      int  true  "Prescribed exercise ID"
//	@Success      200  {array}   api.Exercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/exercises/{exerciseID}/substitutes [get]
func (h *Handlers) ListExerciseSubstitutes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.athleteAccess(w, r); !ok {
		return
	}
	exerciseID, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid exercise ID")
		return
	}
	if _, err := models.GetExerciseByID(r.Context(), h.DB, exerciseID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "exercise not found")
			return
		}
		log.Printf("api: get exercise %d: %v", exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list substitutes")
		return
	}
	subs, err := models.ListExerciseSubstitutes(r.Context(), h.DB, exerciseID)
	if err != nil {
		log.Printf("api: list substitutes for exercise %d: %v", exerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list substitutes")
		return
	}
	out := make([]*Exercise, len(subs))
	for i, e := range subs {
		out[i] = ExerciseFromModel(e)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateExerciseSwap records that the athlete swapped a prescribed
// exercise for a substitute in this workout.
//
//	@Summary      Swap an exercise
//	@Description  Records a swap of a prescribed exercise for one with the same movement pattern, with an optional reason. The coach sees it in review. The program and prescription are unchanged; log the substitute's sets as usual.
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path      int                      true  "Athlete ID"
//	@Param        workoutID  path      int                      true  "Workout ID"
//	@Param        body       body      api.ExerciseSwapRequest  true  "Swap"
//	@Success      201  {object}  api.ExerciseSwap
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/swaps [post]
func (h *Handlers) CreateExerciseSwap(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	workout, ok := h.ownedWorkout(w, r, athleteID)
	if !ok {
		return
	}
	var req ExerciseSwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	switch {
	case req.FromExerciseID <= 0:
		WriteValidationError(w, "from_exercise_id", "is required")
		return
	case req.ToExerciseID <= 0:
		WriteValidationError(w, "to_exercise_id", "is required")
		return
	case req.FromExerciseID == req.ToExerciseID:
		WriteValidationError(w, "to_exercise_id", "must differ from the prescribed exercise")
		return
	case len(req.Reason) > models.MaxSwapReasonLen:
		WriteValidationError(w, "reason", "must be at most 500 characters")
		return
	}
	swap, err := models.CreateExerciseSwap(r.Context(), h.DB, workout.ID, req.FromExerciseID, req.ToExerciseID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNotFound):
			WriteError(w, http.StatusNotFound, "exercise not found")
		case errors.Is(err, models.ErrNoSharedPattern):
			WriteValidationError(w, "to_exercise_id", "must share a movement pattern with the prescribed exercise")
		case errors.Is(err, models.ErrSwapExists):
			WriteError(w, http.StatusConflict, "this exercise was already swapped in this workout")
		case errors.Is(err, models.ErrInvalidInput):
			WriteError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("api: create swap on workout %d: %v", workout.ID, err)
			WriteError(w, http.StatusInternalServerError, "failed to record swap")
		}
		return
	}
	WriteJSON(w, http.StatusCreated, ExerciseSwapFromModel(swap))
}

// DeleteExerciseSwap undoes a swap. Sets already logged stay in the log.
//
//	@Summary      Undo an exercise swap
//	@Tags         Workouts
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        workoutID  path  int  true  "Workout ID"
//	@Param        swapID     path  int  true  "Swap ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/swaps/{swapID} [delete]
func (h *Handlers) DeleteExerciseSwap(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	workout, ok := h.ownedWorkout(w, r, athleteID)
	if !ok {
		return
	}
	swapID, err := strconv.ParseInt(r.PathValue("swapID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid swap ID")
		return
	}
	if err := models.DeleteExerciseSwap(r.Context(), h.DB, workout.ID, swapID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "swap not found")
			return
		}
		log.Printf("api: delete swap %d: %v", swapID, err)
		WriteError(w, http.StatusInternalServerError, "failed to undo swap")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
