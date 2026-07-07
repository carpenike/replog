package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// GetWorkout returns a workout with its sets grouped by exercise.
//
//	@Summary      Get workout
//	@Tags         Workouts
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        workoutID  path      int  true  "Workout ID"
//	@Success      200  {object}  map[string]interface{}  "workout + sets grouped by exercise"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID} [get]
func (h *Handlers) GetWorkout(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	workout, err := models.GetWorkoutByID(r.Context(), h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "workout not found")
		return
	}
	if err != nil {
		log.Printf("api: get workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to get workout")
		return
	}
	// Guard against cross-athlete access: the workout ID is global, so a caller
	// authorized for this athlete must not be able to read another athlete's
	// workout by guessing its ID.
	if workout.AthleteID != athleteID {
		WriteError(w, http.StatusNotFound, "workout not found")
		return
	}

	groups, err := models.ListSetsByWorkout(r.Context(), h.DB, workoutID)
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
//
//	@Summary      Create workout
//	@Description  One workout per athlete per day (UNIQUE constraint). Date defaults to today if omitted.
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                 true  "Athlete ID"
//	@Param        body  body      api.WorkoutRequest  true  "Workout"
//	@Success      201  {object}  api.Workout
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError  "a resistance workout already exists for this date"
//	@Router       /athletes/{id}/workouts [post]
func (h *Handlers) CreateWorkout(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req WorkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Date == "" {
		req.Date = todayInUserTZ(r)
	} else if !validDate(req.Date) {
		WriteValidationError(w, "date", "must be a valid date in YYYY-MM-DD format")
		return
	}

	// When seeding from the prescription, resolve today's assignment so the
	// workout links to it (advancing cycle progress) and we can transcribe the
	// prescribed sets into the log for the athlete to confirm.
	var seedProgram *models.AthleteProgram
	var assignmentID int64
	if req.FromPrescription {
		tz := "America/New_York"
		if prefs := middleware.PrefsFromContext(r.Context()); prefs != nil {
			tz = prefs.Timezone
		}
		program, perr := models.ResolveAssignment(r.Context(), h.DB, athleteID, time.Now(), tz)
		if perr != nil {
			log.Printf("api: resolve assignment for athlete %d: %v", athleteID, perr)
			WriteError(w, http.StatusInternalServerError, "failed to resolve program")
			return
		}
		if program != nil {
			seedProgram = program
			assignmentID = program.ID
		}
	}

	workout, err := models.CreateWorkout(r.Context(), h.DB, athleteID, req.Date, req.Notes, assignmentID)
	if err != nil {
		if errors.Is(err, models.ErrWorkoutExists) {
			WriteError(w, http.StatusConflict, "a resistance workout already exists for this date")
			return
		}
		log.Printf("api: create workout for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create workout")
		return
	}

	// Seed the prescribed sets, if requested and a program resolved.
	if seedProgram != nil {
		prescription, perr := models.GetPrescription(r.Context(), h.DB, seedProgram, time.Now())
		if perr != nil {
			log.Printf("api: get prescription for seeding workout %d: %v", workout.ID, perr)
		} else if _, serr := models.SeedSetsFromPrescription(r.Context(), h.DB, workout.ID, prescription); serr != nil {
			log.Printf("api: seed prescribed sets for workout %d: %v", workout.ID, serr)
		}
	}

	// Notify the athlete's coach that a workout was logged (ADR 008).
	// Skip when the coach is also the one calling the endpoint — they
	// already know they just made the workout (no point pinging them).
	if athlete, aerr := models.GetAthleteByID(r.Context(), h.DB, athleteID); aerr == nil &&
		athlete.CoachID.Valid && athlete.CoachID.Int64 != user.ID {
		h.notifyCoach(r.Context(), athleteID, models.NotifyWorkoutLogged,
			fmt.Sprintf("%s logged a workout", h.athleteDisplayName(r.Context(), athleteID)),
			req.Date,
			fmt.Sprintf("/athletes/%d/workouts/%d", athleteID, workout.ID))
	}

	WriteJSON(w, http.StatusCreated, WorkoutFromModel(workout))
}

// DeleteWorkout deletes a workout.
//
//	@Summary      Delete workout
//	@Tags         Workouts
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        workoutID  path      int  true  "Workout ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID} [delete]
func (h *Handlers) DeleteWorkout(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	if err := models.DeleteWorkout(r.Context(), h.DB, workoutID, athleteID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "workout not found")
			return
		}
		log.Printf("api: delete workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete workout")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AddWorkoutSet adds a set to a workout.
//
//	@Summary      Add set to workout
//	@Description  rep_type defaults to 'reps' and category to 'main' when omitted; both are CHECK-constrained in the schema.
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path      int                    true  "Athlete ID"
//	@Param        workoutID  path      int                    true  "Workout ID"
//	@Param        body       body      api.WorkoutSetRequest  true  "Set"
//	@Success      201  {object}  api.WorkoutSet
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets [post]
func (h *Handlers) AddWorkoutSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	// Verify the workout belongs to the path athlete before mutating it — the
	// workout ID is global and CanAccessAthlete only authorized {id}.
	workout, err := models.GetWorkoutByID(r.Context(), h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) || (err == nil && workout.AthleteID != athleteID) {
		WriteError(w, http.StatusNotFound, "workout not found")
		return
	}
	if err != nil {
		log.Printf("api: get workout %d for add set: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to add set")
		return
	}

	var req WorkoutSetRequest
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
	// Validate CHECK-constrained enums/ranges at the boundary so violations
	// surface as 400 (with details) instead of a 500 from the DB layer.
	if !validSetRepType(req.RepType) {
		WriteValidationError(w, "rep_type", "must be one of reps, each_side, seconds, distance")
		return
	}
	if !validSetCategory(req.Category) {
		WriteValidationError(w, "category", "must be one of main, supplemental, accessory")
		return
	}
	if req.RPE != 0 && (req.RPE < 1 || req.RPE > 10) {
		WriteValidationError(w, "rpe", "must be between 1 and 10")
		return
	}

	set, err := models.AddSet(r.Context(), h.DB, workoutID, req.ExerciseID, req.Reps, req.Weight, req.RPE, req.RepType, req.Category, req.Notes)
	if err != nil {
		log.Printf("api: add set to workout %d: %v", workoutID, err)
		WriteDBError(w, err, "failed to add set")
		return
	}

	// Auto-approve workout if athlete is logging their own sets.
	// Best-effort: a failure here doesn't change the API contract for
	// the caller (the set was successfully added).
	if err := models.AutoApproveWorkout(r.Context(), h.DB, workoutID, user.ID); err != nil {
		log.Printf("api: auto-approve workout %d: %v", workoutID, err)
	}

	WriteJSON(w, http.StatusCreated, WorkoutSetFromModel(set))
}

// UpdateWorkoutSet updates a set.
//
//	@Summary      Update set
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path      int                          true  "Athlete ID"
//	@Param        workoutID  path      int                          true  "Workout ID"
//	@Param        setID      path      int                          true  "Set ID"
//	@Param        body       body      api.WorkoutSetUpdateRequest  true  "Set"
//	@Success      200  {object}  api.WorkoutSet
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID} [put]
func (h *Handlers) UpdateWorkoutSet(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	var req WorkoutSetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Pointer fields: nil = leave unchanged. Only validate a supplied, non-zero
	// RPE (a supplied 0 explicitly clears it).
	if req.RPE != nil && *req.RPE != 0 && (*req.RPE < 1 || *req.RPE > 10) {
		WriteValidationError(w, "rpe", "must be between 1 and 10")
		return
	}

	set, err := models.UpdateSet(r.Context(), h.DB, setID, athleteID, req.Reps, req.Weight, req.RPE, req.Notes)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "set not found")
		return
	}
	if err != nil {
		log.Printf("api: update set %d: %v", setID, err)
		WriteDBError(w, err, "failed to update set")
		return
	}

	WriteJSON(w, http.StatusOK, WorkoutSetFromModel(set))
}

// DeleteWorkoutSet deletes a set.
//
//	@Summary      Delete set
//	@Tags         Workouts
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        workoutID  path      int  true  "Workout ID"
//	@Param        setID      path      int  true  "Set ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID} [delete]
func (h *Handlers) DeleteWorkoutSet(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	if err := models.DeleteSet(r.Context(), h.DB, setID, athleteID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "set not found")
			return
		}
		log.Printf("api: delete set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete set")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
