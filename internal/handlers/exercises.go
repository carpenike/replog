package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Exercises holds dependencies for exercise handlers.
type Exercises struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders the exercise list page.
func (h *Exercises) List(w http.ResponseWriter, r *http.Request) {
	tierFilter := r.URL.Query().Get("tier")

	exercises, err := models.ListExercises(h.DB, tierFilter)
	if err != nil {
		log.Printf("handlers: list exercises: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Exercises":  exercises,
		"TierFilter": tierFilter,
		"Tiers":      tierFilterOptions(),
	}
	if err := h.Templates.Render(w, r, "exercises_list.html", data); err != nil {
		log.Printf("handlers: exercises list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new exercise form. Coach only.
func (h *Exercises) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := map[string]any{
		"Tiers": tierOptions(),
	}
	if err := h.Templates.Render(w, r, "exercise_form.html", data); err != nil {
		log.Printf("handlers: exercise new form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new exercise form submission. Coach only.
func (h *Exercises) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		data := map[string]any{
			"Error": "Name is required",
			"Tiers": tierOptions(),
			"Form":  r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "exercise_form.html", data)
		return
	}

	targetReps, _ := strconv.Atoi(r.FormValue("target_reps"))
	restSeconds, _ := strconv.Atoi(r.FormValue("rest_seconds"))

	exercise, err := models.CreateExercise(h.DB, name, r.FormValue("tier"), targetReps, r.FormValue("form_notes"), r.FormValue("demo_url"), restSeconds)
	if errors.Is(err, models.ErrDuplicateExerciseName) {
		data := map[string]any{
			"Error": "An exercise with that name already exists",
			"Tiers": tierOptions(),
			"Form":  r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "exercise_form.html", data)
		return
	}
	if err != nil {
		log.Printf("handlers: create exercise: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/exercises/"+strconv.FormatInt(exercise.ID, 10), http.StatusSeeOther)
}

// Show renders the exercise detail page.
func (h *Exercises) Show(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	exercise, err := models.GetExerciseByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get exercise %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	assignedAthletes, err := models.ListAssignedAthletes(h.DB, id)
	if err != nil {
		log.Printf("handlers: list assigned athletes for exercise %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	recentSets, err := models.ListRecentSetsForExercise(h.DB, id)
	if err != nil {
		log.Printf("handlers: list recent sets for exercise %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Non-coaches should only see their own athlete data, not other athletes'.
	if !user.IsCoach {
		if user.AthleteID.Valid {
			ownID := user.AthleteID.Int64
			var filteredAthletes []*models.AssignedAthlete
			for _, a := range assignedAthletes {
				if a.AthleteID == ownID {
					filteredAthletes = append(filteredAthletes, a)
				}
			}
			assignedAthletes = filteredAthletes

			var filteredSets []*models.RecentExerciseSet
			for _, s := range recentSets {
				if s.AthleteID == ownID {
					filteredSets = append(filteredSets, s)
				}
			}
			recentSets = filteredSets
		} else {
			assignedAthletes = nil
			recentSets = nil
		}
	}

	data := map[string]any{
		"Exercise":         exercise,
		"AssignedAthletes": assignedAthletes,
		"RecentSets":       recentSets,
	}
	if err := h.Templates.Render(w, r, "exercise_detail.html", data); err != nil {
		log.Printf("handlers: exercise detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// EditForm renders the edit exercise form. Coach only.
func (h *Exercises) EditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	exercise, err := models.GetExerciseByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get exercise %d for edit: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Exercise": exercise,
		"Tiers":    tierOptions(),
	}
	if err := h.Templates.Render(w, r, "exercise_form.html", data); err != nil {
		log.Printf("handlers: exercise edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update processes the edit exercise form submission. Coach only.
func (h *Exercises) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		exercise, _ := models.GetExerciseByID(h.DB, id)
		data := map[string]any{
			"Error":    "Name is required",
			"Exercise": exercise,
			"Tiers":    tierOptions(),
			"Form":     r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "exercise_form.html", data)
		return
	}

	targetReps, _ := strconv.Atoi(r.FormValue("target_reps"))
	restSeconds, _ := strconv.Atoi(r.FormValue("rest_seconds"))

	_, err = models.UpdateExercise(h.DB, id, name, r.FormValue("tier"), targetReps, r.FormValue("form_notes"), r.FormValue("demo_url"), restSeconds)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, models.ErrDuplicateExerciseName) {
		exercise, _ := models.GetExerciseByID(h.DB, id)
		data := map[string]any{
			"Error":    "An exercise with that name already exists",
			"Exercise": exercise,
			"Tiers":    tierOptions(),
			"Form":     r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "exercise_form.html", data)
		return
	}
	if err != nil {
		log.Printf("handlers: update exercise %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/exercises/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// Delete removes an exercise. Coach only.
func (h *Exercises) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteExercise(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, models.ErrExerciseInUse) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		if err := h.Templates.RenderErrorFragment(w, "Cannot delete — exercise has been logged in workouts."); err != nil {
			log.Printf("handlers: delete exercise error template: %v", err)
		}
		return
	}
	if err != nil {
		log.Printf("handlers: delete exercise %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/exercises", http.StatusSeeOther)
}

func tierFilterOptions() []struct{ Value, Label string } {
	return []struct{ Value, Label string }{
		{"", "All Tiers"},
		{"foundational", "Foundational"},
		{"intermediate", "Intermediate"},
		{"sport_performance", "Sport Performance"},
		{"none", "No Tier"},
	}
}

// ExerciseHistory renders the exercise history for a specific athlete+exercise.
func (h *Exercises) ExerciseHistory(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for exercise history: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	exercise, err := models.GetExerciseByID(h.DB, exerciseID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get exercise %d for exercise history: %v", exerciseID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	page, err := models.ListExerciseHistory(h.DB, athleteID, exerciseID, offset)
	if err != nil {
		log.Printf("handlers: list exercise history for athlete %d exercise %d: %v", athleteID, exerciseID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load volume bar chart data.
	volumeChart, chartErr := models.ExerciseVolumeChart(h.DB, athleteID, exerciseID, 20)
	if chartErr != nil {
		log.Printf("handlers: exercise volume chart for athlete %d exercise %d: %v", athleteID, exerciseID, chartErr)
	}

	data := map[string]any{
		"Athlete":     athlete,
		"Exercise":    exercise,
		"Days":        page.Days,
		"HasMore":     page.HasMore,
		"NextOffset":  offset + models.ExerciseHistoryPageSize,
		"VolumeChart": volumeChart,
	}
	if err := h.Templates.Render(w, r, "exercise_history.html", data); err != nil {
		log.Printf("handlers: exercise history template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
