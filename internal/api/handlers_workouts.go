package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// GetWorkout returns a workout with its sets grouped by exercise.
func (h *Handlers) GetWorkout(w http.ResponseWriter, r *http.Request) {
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

	workout, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "workout not found")
		return
	}
	if err != nil {
		log.Printf("api: get workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get workout")
		return
	}

	groups, err := models.ListSetsByWorkout(h.DB, workoutID)
	if err != nil {
		log.Printf("api: list sets for workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list sets")
		return
	}

	apiGroups := make([]*ExerciseGroup, len(groups))
	for i, g := range groups {
		sets := make([]*WorkoutSet, len(g.Sets))
		for j, s := range g.Sets {
			sets[j] = WorkoutSetFromModel(s)
		}
		apiGroups[i] = &ExerciseGroup{
			ExerciseID:   g.ExerciseID,
			ExerciseName: g.ExerciseName,
			Sets:         sets,
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"workout": WorkoutFromModel(workout),
		"groups":  apiGroups,
	})
}

// CreateWorkout creates a new workout for an athlete.
func (h *Handlers) CreateWorkout(w http.ResponseWriter, r *http.Request) {
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
		Date  string `json:"date"`
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	workout, err := models.CreateWorkout(h.DB, athleteID, req.Date, req.Notes, 0)
	if err != nil {
		log.Printf("api: create workout for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create workout")
		return
	}

	WriteJSON(w, http.StatusCreated, WorkoutFromModel(workout))
}

// DeleteWorkout deletes a workout.
func (h *Handlers) DeleteWorkout(w http.ResponseWriter, r *http.Request) {
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

	if err := models.DeleteWorkout(h.DB, workoutID); err != nil {
		log.Printf("api: delete workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete workout")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AddWorkoutSet adds a set to a workout.
func (h *Handlers) AddWorkoutSet(w http.ResponseWriter, r *http.Request) {
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
		ExerciseID int64   `json:"exercise_id"`
		Reps       int     `json:"reps"`
		Weight     float64 `json:"weight"`
		RPE        float64 `json:"rpe"`
		RepType    string  `json:"rep_type"`
		Category   string  `json:"category"`
		Notes      string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 || req.Reps == 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id and reps are required")
		return
	}
	if req.RepType == "" {
		req.RepType = "reps"
	}
	if req.Category == "" {
		req.Category = "main"
	}

	set, err := models.AddSet(h.DB, workoutID, req.ExerciseID, req.Reps, req.Weight, req.RPE, req.RepType, req.Category, req.Notes)
	if err != nil {
		log.Printf("api: add set to workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to add set")
		return
	}

	// Auto-approve workout if athlete is logging their own sets.
	models.AutoApproveWorkout(h.DB, workoutID, user.ID)

	WriteJSON(w, http.StatusCreated, WorkoutSetFromModel(set))
}

// UpdateWorkoutSet updates a set.
func (h *Handlers) UpdateWorkoutSet(w http.ResponseWriter, r *http.Request) {
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

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	var req struct {
		Reps   int     `json:"reps"`
		Weight float64 `json:"weight"`
		RPE    float64 `json:"rpe"`
		Notes  string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	set, err := models.UpdateSet(h.DB, setID, req.Reps, req.Weight, req.RPE, req.Notes)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "set not found")
		return
	}
	if err != nil {
		log.Printf("api: update set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update set")
		return
	}

	WriteJSON(w, http.StatusOK, WorkoutSetFromModel(set))
}

// DeleteWorkoutSet deletes a set.
func (h *Handlers) DeleteWorkoutSet(w http.ResponseWriter, r *http.Request) {
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

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	if err := models.DeleteSet(h.DB, setID); err != nil {
		log.Printf("api: delete set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete set")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
