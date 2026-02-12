package handlers

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/models"
)

// Exercises holds dependencies for exercise handlers.
type Exercises struct {
	DB        *sql.DB
	Templates *template.Template
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
	if err := h.Templates.ExecuteTemplate(w, "exercises-list", data); err != nil {
		log.Printf("handlers: exercises list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new exercise form.
func (h *Exercises) NewForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Tiers": tierOptions(),
	}
	if err := h.Templates.ExecuteTemplate(w, "exercise-form", data); err != nil {
		log.Printf("handlers: exercise new form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new exercise form submission.
func (h *Exercises) Create(w http.ResponseWriter, r *http.Request) {
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
		h.Templates.ExecuteTemplate(w, "exercise-form", data)
		return
	}

	targetReps, _ := strconv.Atoi(r.FormValue("target_reps"))

	exercise, err := models.CreateExercise(h.DB, name, r.FormValue("tier"), targetReps, r.FormValue("form_notes"))
	if errors.Is(err, models.ErrDuplicateUsername) {
		data := map[string]any{
			"Error": "An exercise with that name already exists",
			"Tiers": tierOptions(),
			"Form":  r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.ExecuteTemplate(w, "exercise-form", data)
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

	data := map[string]any{
		"Exercise": exercise,
	}
	if err := h.Templates.ExecuteTemplate(w, "exercise-detail", data); err != nil {
		log.Printf("handlers: exercise detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// EditForm renders the edit exercise form.
func (h *Exercises) EditForm(w http.ResponseWriter, r *http.Request) {
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
	if err := h.Templates.ExecuteTemplate(w, "exercise-form", data); err != nil {
		log.Printf("handlers: exercise edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update processes the edit exercise form submission.
func (h *Exercises) Update(w http.ResponseWriter, r *http.Request) {
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
		h.Templates.ExecuteTemplate(w, "exercise-form", data)
		return
	}

	targetReps, _ := strconv.Atoi(r.FormValue("target_reps"))

	_, err = models.UpdateExercise(h.DB, id, name, r.FormValue("tier"), targetReps, r.FormValue("form_notes"))
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, models.ErrDuplicateUsername) {
		exercise, _ := models.GetExerciseByID(h.DB, id)
		data := map[string]any{
			"Error":    "An exercise with that name already exists",
			"Exercise": exercise,
			"Tiers":    tierOptions(),
			"Form":     r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.ExecuteTemplate(w, "exercise-form", data)
		return
	}
	if err != nil {
		log.Printf("handlers: update exercise %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/exercises/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// Delete removes an exercise.
func (h *Exercises) Delete(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Cannot delete — exercise has been logged in workouts", http.StatusConflict)
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
