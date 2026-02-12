package handlers

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Workouts holds dependencies for workout handlers.
type Workouts struct {
	DB        *sql.DB
	Templates *template.Template
}

// checkAthleteAccess verifies the user can access the given athlete.
// Returns false and writes an error response if access is denied.
func checkAthleteAccess(w http.ResponseWriter, r *http.Request) bool {
	user := middleware.UserFromContext(r.Context())
	athleteIDStr := r.PathValue("id")
	athleteID, err := strconv.ParseInt(athleteIDStr, 10, 64)
	if err != nil {
		return true // let the handler catch the parse error
	}
	if !middleware.CanAccessAthlete(user, athleteID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// List renders the workout history for an athlete.
func (h *Workouts) List(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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

	workouts, err := models.ListWorkouts(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: list workouts for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":  athlete,
		"Workouts": workouts,
	}
	if err := h.Templates.ExecuteTemplate(w, "workouts-list", data); err != nil {
		log.Printf("handlers: workouts list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new workout form.
func (h *Workouts) NewForm(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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
	if err := h.Templates.ExecuteTemplate(w, "workout-form", data); err != nil {
		log.Printf("handlers: workout form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new workout form submission.
func (h *Workouts) Create(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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
		h.Templates.ExecuteTemplate(w, "workout-form", data)
		return
	}
	if err != nil {
		log.Printf("handlers: create workout: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workout.ID, 10), http.StatusSeeOther)
}

// Show renders the workout detail page with logged sets and add-set form.
func (h *Workouts) Show(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

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

	groups, err := models.ListSetsByWorkout(h.DB, workoutID)
	if err != nil {
		log.Printf("handlers: list sets for workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load assigned exercises first, then all exercises for the full library.
	assigned, err := models.ListActiveAssignments(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: list assignments for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	allExercises, err := models.ListExercises(h.DB, "")
	if err != nil {
		log.Printf("handlers: list exercises: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
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

	data := map[string]any{
		"Athlete":      athlete,
		"Workout":      workout,
		"Groups":       groups,
		"Assigned":     assigned,
		"Unassigned":   unassigned,
		"IsCoach":      middleware.UserFromContext(r.Context()).IsCoach,
		"TMByExercise": tmByExercise,
	}
	if err := h.Templates.ExecuteTemplate(w, "workout-detail", data); err != nil {
		log.Printf("handlers: workout detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// UpdateNotes updates workout-level notes.
func (h *Workouts) UpdateNotes(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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

	if err := models.UpdateWorkoutNotes(h.DB, workoutID, r.FormValue("notes")); err != nil {
		log.Printf("handlers: update workout %d notes: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// Delete removes a workout and all its sets. Coach only.
func (h *Workouts) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid workout ID", http.StatusBadRequest)
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
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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
		http.Error(w, "Exercise is required", http.StatusBadRequest)
		return
	}

	reps, err := strconv.Atoi(r.FormValue("reps"))
	if err != nil || reps <= 0 {
		http.Error(w, "Reps must be a positive number", http.StatusBadRequest)
		return
	}

	var weight float64
	if ws := r.FormValue("weight"); ws != "" {
		weight, err = strconv.ParseFloat(ws, 64)
		if err != nil {
			http.Error(w, "Invalid weight", http.StatusBadRequest)
			return
		}
	}

	notes := r.FormValue("notes")

	_, err = models.AddSet(h.DB, workoutID, exerciseID, reps, weight, notes)
	if err != nil {
		log.Printf("handlers: add set to workout %d: %v", workoutID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// EditSetForm renders the edit set form.
func (h *Workouts) EditSetForm(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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

	data := map[string]any{
		"Athlete": athlete,
		"Workout": workout,
		"Set":     set,
	}
	if err := h.Templates.ExecuteTemplate(w, "set-edit-form", data); err != nil {
		log.Printf("handlers: set edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// UpdateSet processes the edit set form.
func (h *Workouts) UpdateSet(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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
		http.Error(w, "Reps must be a positive number", http.StatusBadRequest)
		return
	}

	var weight float64
	if ws := r.FormValue("weight"); ws != "" {
		weight, err = strconv.ParseFloat(ws, 64)
		if err != nil {
			http.Error(w, "Invalid weight", http.StatusBadRequest)
			return
		}
	}

	notes := r.FormValue("notes")

	_, err = models.UpdateSet(h.DB, setID, reps, weight, notes)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Set not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: update set %d: %v", setID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/workouts/"+strconv.FormatInt(workoutID, 10), http.StatusSeeOther)
}

// DeleteSet removes a set from a workout.
func (h *Workouts) DeleteSet(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
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
