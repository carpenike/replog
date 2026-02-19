package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Workouts holds dependencies for workout handlers.
type Workouts struct {
	DB        *sql.DB
	Templates TemplateCache
}

// checkAthleteAccess verifies the user can access the given athlete.
// Returns the parsed athlete ID and true on success, or 0 and false after
// writing an error/forbidden response.
func checkAthleteAccess(db *sql.DB, tc TemplateCache, w http.ResponseWriter, r *http.Request) (int64, bool) {
	user := middleware.UserFromContext(r.Context())
	athleteIDStr := r.PathValue("id")
	athleteID, err := strconv.ParseInt(athleteIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return 0, false
	}
	if !middleware.CanAccessAthlete(db, user, athleteID) {
		tc.Forbidden(w, r)
		return 0, false
	}
	return athleteID, true
}

// List renders the workout history for an athlete.
func (h *Workouts) List(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for workouts: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	page, err := models.ListWorkouts(h.DB, athleteID, offset)
	if err != nil {
		log.Printf("handlers: list workouts for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":    athlete,
		"Workouts":   page.Workouts,
		"HasMore":    page.HasMore,
		"NextOffset": offset + models.WorkoutPageSize,
	}
	if err := h.Templates.Render(w, r, "workouts_list.html", data); err != nil {
		log.Printf("handlers: workouts list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new workout form.
func (h *Workouts) NewForm(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for new workout: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
		"Today":   time.Now().Format("2006-01-02"),
	}
	if err := h.Templates.Render(w, r, "workout_form.html", data); err != nil {
		log.Printf("handlers: workout form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new workout form submission.
func (h *Workouts) Create(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	notes := r.FormValue("notes")

	workout, err := models.CreateWorkout(h.DB, athleteID, date, notes)
	if errors.Is(err, models.ErrWorkoutExists) {
		// Redirect to the existing workout for that date.
		existing, getErr := models.GetWorkoutByAthleteDate(h.DB, athleteID, date)
		if getErr == nil {
			http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(existing.ID, 10), http.StatusSeeOther)
			return
		}
		athlete, _ := models.GetAthleteByID(h.DB, athleteID)
		data := map[string]any{
			"Athlete": athlete,
			"Today":   time.Now().Format("2006-01-02"),
			"Error":   "A workout already exists for " + date,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "workout_form.html", data)
		return
	}
	if err != nil {
		log.Printf("handlers: create workout: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Auto-approve when a coach/admin creates a workout on behalf of an athlete.
	user := middleware.UserFromContext(r.Context())
	if user.IsCoach || user.IsAdmin {
		if err := models.AutoApproveWorkout(h.DB, workout.ID, user.ID); err != nil {
			log.Printf("handlers: auto-approve workout %d: %v", workout.ID, err)
			// Non-fatal — the workout was created successfully.
		}
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workout.ID, 10), http.StatusSeeOther)
}

// Show renders the workout detail page with logged sets and add-set form.
func (h *Workouts) Show(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	user := middleware.UserFromContext(r.Context())

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	workout, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Verify the workout belongs to the specified athlete.
	if workout.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	data, err := h.loadWorkoutShowData(user, athlete, workout)
	if err != nil {
		log.Printf("handlers: load workout show data %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Surface validation errors from AddSet/UpdateSet redirects.
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		data["SetError"] = errMsg
	}
	// Sticky exercise: if redirected from AddSet, pre-select the exercise.
	if eidStr := r.URL.Query().Get("exercise_id"); eidStr != "" {
		data["SelectedExerciseID"], _ = strconv.ParseInt(eidStr, 10, 64)
	}

	if err := h.Templates.Render(w, r, "workout_detail.html", data); err != nil {
		log.Printf("handlers: workout detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// loadWorkoutShowData fetches all data needed for the workout detail page.
// Fatal queries return errors; non-fatal queries log and continue with nil/zero values.
func (h *Workouts) loadWorkoutShowData(user *models.User, athlete *models.Athlete, workout *models.Workout) (map[string]any, error) {
	athleteID := athlete.ID
	workoutID := workout.ID

	groups, err := models.ListSetsByWorkout(h.DB, workoutID)
	if err != nil {
		return nil, fmt.Errorf("list sets: %w", err)
	}

	// Load assigned exercises first, then all exercises for the full library.
	assigned, err := models.ListActiveAssignments(h.DB, athleteID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}

	allExercises, err := models.ListExercises(h.DB, "")
	if err != nil {
		return nil, fmt.Errorf("list exercises: %w", err)
	}

	// Build set of assigned exercise IDs.
	assignedIDs := make(map[int64]bool)
	for _, a := range assigned {
		assignedIDs[a.ExerciseID] = true
	}

	// Unassigned exercises (full library minus assigned).
	var unassigned []*models.Exercise
	for _, e := range allExercises {
		if !assignedIDs[e.ID] {
			unassigned = append(unassigned, e)
		}
	}

	// Load current training maxes for the athlete to display in the daily view.
	currentTMs, err := models.ListCurrentTrainingMaxes(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: list training maxes for athlete %d: %v", athleteID, err)
		// Non-fatal — continue without TM data.
	}
	tmByExercise := make(map[int64]*models.TrainingMax)
	for _, tm := range currentTMs {
		tmByExercise[tm.ExerciseID] = tm
	}

	// Load today's prescription if the athlete has an active program.
	// Use the workout's date (not necessarily "today") so historical workouts
	// still show what was prescribed on that date.
	var prescription *models.Prescription
	workoutDate, parseErr := time.Parse("2006-01-02", workout.Date)
	if parseErr != nil {
		// SQLite may return full RFC3339 timestamps for DATE columns.
		workoutDate, parseErr = time.Parse(time.RFC3339, workout.Date)
	}
	if parseErr == nil {
		prescription, err = models.GetPrescription(h.DB, athleteID, workoutDate)
		if err != nil {
			log.Printf("handlers: get prescription for athlete %d on %s: %v", athleteID, workout.Date, err)
			// Non-fatal — continue without prescription data.
		}
	}

	// Load "last time" data for each exercise in the logged groups.
	// Shows what the athlete did in their previous session for each exercise.
	lastSession := make(map[int64]*models.LastSessionSummary)
	dateForLookup := workout.Date
	if len(dateForLookup) > 10 {
		dateForLookup = dateForLookup[:10]
	}
	for _, g := range groups {
		if prev, err := models.LastSessionSets(h.DB, athleteID, g.ExerciseID, dateForLookup); err == nil && prev != nil {
			lastSession[g.ExerciseID] = prev
		}
	}

	// Build a map of exercise_id → logged set count for the prescription scaffold.
	loggedSetCounts := make(map[int64]int)
	for _, g := range groups {
		loggedSetCounts[g.ExerciseID] = len(g.Sets)
	}

	// Load review for this workout (if any).
	var review *models.WorkoutReview
	rev, revErr := models.GetWorkoutReviewByWorkoutID(h.DB, workoutID)
	if revErr == nil {
		review = rev
	} else if !errors.Is(revErr, models.ErrNotFound) {
		log.Printf("handlers: get review for workout %d: %v", workoutID, revErr)
		// Non-fatal — continue without review data.
	}

	return map[string]any{
		"Athlete":         athlete,
		"Workout":         workout,
		"Groups":          groups,
		"Assigned":        assigned,
		"Unassigned":      unassigned,
		"TMByExercise":    tmByExercise,
		"Prescription":    prescription,
		"LoggedSetCounts": loggedSetCounts,
		"LastSession":     lastSession,
		"Review":          review,
		"CanManage":       middleware.CanManageAthlete(user, athlete),
		"IsOwnProfile":    user.AthleteID.Valid && user.AthleteID.Int64 == int64(athlete.ID),
	}, nil
}

// UpdateNotes updates workout-level notes.
func (h *Workouts) UpdateNotes(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Verify the workout belongs to the specified athlete.
	workout, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if workout.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	if err := models.UpdateWorkoutNotes(h.DB, workoutID, r.FormValue("notes")); err != nil {
		log.Printf("handlers: update workout %d notes: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// Delete removes a workout and all its sets. Coach only.
func (h *Workouts) Delete(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	// Verify the workout belongs to the specified athlete.
	workout, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if workout.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	err = models.DeleteWorkout(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: delete workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts", http.StatusSeeOther)
}

// AddSet handles adding a set to a workout.
func (h *Workouts) AddSet(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	exerciseID, err := strconv.ParseInt(r.FormValue("exercise_id"), 10, 64)
	if err != nil || exerciseID == 0 {
		workoutRedirectWithError(w, r, athleteID, workoutID, "Exercise is required")
		return
	}

	reps, err := strconv.Atoi(r.FormValue("reps"))
	if err != nil || reps <= 0 {
		workoutRedirectWithError(w, r, athleteID, workoutID, "Reps must be a positive number")
		return
	}

	var weight float64
	if ws := r.FormValue("weight"); ws != "" {
		weight, err = strconv.ParseFloat(ws, 64)
		if err != nil {
			workoutRedirectWithError(w, r, athleteID, workoutID, "Invalid weight")
			return
		}
	}

	// Verify the workout belongs to the specified athlete.
	workoutCheck, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if workoutCheck.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	notes := r.FormValue("notes")
	repType := r.FormValue("rep_type")

	var rpe float64
	if rs := r.FormValue("rpe"); rs != "" {
		rpe, err = strconv.ParseFloat(rs, 64)
		if err != nil {
			workoutRedirectWithError(w, r, athleteID, workoutID, "Invalid RPE")
			return
		}
	}

	// Support bulk-adding identical sets (e.g. 5×5 @ 135).
	setCount := 1
	if sc := r.FormValue("sets"); sc != "" {
		setCount, err = strconv.Atoi(sc)
		if err != nil || setCount < 1 {
			workoutRedirectWithError(w, r, athleteID, workoutID, "Sets must be a positive number")
			return
		}
		if setCount > 20 {
			workoutRedirectWithError(w, r, athleteID, workoutID, "Cannot log more than 20 sets at once")
			return
		}
	}

	if setCount > 1 {
		_, err = models.AddMultipleSets(h.DB, workoutID, exerciseID, setCount, reps, weight, rpe, repType, notes)
	} else {
		_, err = models.AddSet(h.DB, workoutID, exerciseID, reps, weight, rpe, repType, notes)
	}
	if err != nil {
		log.Printf("handlers: add set(s) to workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Auto-approve when a coach/admin logs sets for an athlete.
	user := middleware.UserFromContext(r.Context())
	if user.IsCoach || user.IsAdmin {
		if err := models.AutoApproveWorkout(h.DB, workoutID, user.ID); err != nil {
			log.Printf("handlers: auto-approve workout %d: %v", workoutID, err)
		}
	}

	// Look up exercise rest time for the timer.
	restSeconds := models.DefaultRestSeconds
	if ex, exErr := models.GetExerciseByID(h.DB, exerciseID); exErr == nil {
		restSeconds = ex.EffectiveRestSeconds()
	}

	// Include exercise_id in redirect for sticky exercise selection.
	redirectURL := "/athletes/" + strconv.FormatInt(athleteID, 10) + "/workouts/" + strconv.FormatInt(workoutID, 10) +
		"?timer=" + strconv.Itoa(restSeconds) + "&exercise_id=" + strconv.FormatInt(exerciseID, 10)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// EditSetForm renders the edit set form.
func (h *Workouts) EditSetForm(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid set ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	workout, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Verify the workout belongs to the specified athlete.
	if workout.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	set, err := models.GetSetByID(h.DB, setID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get set %d: %v", setID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Verify the set belongs to the specified workout.
	if set.WorkoutID != workoutID {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
		"Workout": workout,
		"Set":     set,
	}
	if err := h.Templates.Render(w, r, "set_edit_form.html", data); err != nil {
		log.Printf("handlers: set edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// UpdateSet processes the edit set form.
func (h *Workouts) UpdateSet(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid set ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	reps, err := strconv.Atoi(r.FormValue("reps"))
	if err != nil || reps <= 0 {
		workoutRedirectWithError(w, r, athleteID, workoutID, "Reps must be a positive number")
		return
	}

	var weight float64
	if ws := r.FormValue("weight"); ws != "" {
		weight, err = strconv.ParseFloat(ws, 64)
		if err != nil {
			workoutRedirectWithError(w, r, athleteID, workoutID, "Invalid weight")
			return
		}
	}

	// Verify the workout belongs to the specified athlete.
	workoutCheck, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if workoutCheck.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	notes := r.FormValue("notes")

	var rpe float64
	if rs := r.FormValue("rpe"); rs != "" {
		rpe, err = strconv.ParseFloat(rs, 64)
		if err != nil {
			workoutRedirectWithError(w, r, athleteID, workoutID, "Invalid RPE")
			return
		}
	}

	// Verify the set belongs to the specified workout.
	setCheck, err := models.GetSetByID(h.DB, setID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get set %d for update: %v", setID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if setCheck.WorkoutID != workoutID {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}

	_, err = models.UpdateSet(h.DB, setID, reps, weight, rpe, notes)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: update set %d: %v", setID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Auto-approve when a coach/admin edits a set.
	user := middleware.UserFromContext(r.Context())
	if user.IsCoach || user.IsAdmin {
		if err := models.AutoApproveWorkout(h.DB, workoutID, user.ID); err != nil {
			log.Printf("handlers: auto-approve workout %d: %v", workoutID, err)
		}
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// DeleteSet removes a set from a workout.
func (h *Workouts) DeleteSet(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid set ID", http.StatusBadRequest)
		return
	}

	// Verify the workout belongs to the specified athlete.
	workoutCheck, err := models.GetWorkoutByID(h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if workoutCheck.AthleteID != athleteID {
		http.Error(w, "Workout not found", http.StatusNotFound)
		return
	}

	// Verify the set belongs to the specified workout.
	setCheck, err := models.GetSetByID(h.DB, setID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get set %d for delete: %v", setID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if setCheck.WorkoutID != workoutID {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}

	err = models.DeleteSet(h.DB, setID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: delete set %d: %v", setID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// workoutRedirectWithError redirects back to the workout detail page with an
// error message shown to the user. Used for form validation errors that should
// surface inline instead of as plain-text HTTP error responses.
func workoutRedirectWithError(w http.ResponseWriter, r *http.Request, athleteID, workoutID int64, msg string) {
	target := "/athletes/" + strconv.FormatInt(athleteID, 10) +
		"/workouts/" + strconv.FormatInt(workoutID, 10) +
		"?error=" + url.QueryEscape(msg)
	http.Redirect(w, r, target, http.StatusSeeOther)
}