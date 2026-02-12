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

// Assignments holds dependencies for assignment handlers.
type Assignments struct {
	DB        *sql.DB
	Templates *template.Template
}

// Assign creates a new active assignment for an athlete.
func (h *Assignments) Assign(w http.ResponseWriter, r *http.Request) {
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	exerciseIDStr := r.FormValue("exercise_id")
	if exerciseIDStr == "" {
		http.Error(w, "Exercise is required", http.StatusBadRequest)
		return
	}

	exerciseID, err := strconv.ParseInt(exerciseIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	_, err = models.AssignExercise(h.DB, athleteID, exerciseID)
	if errors.Is(err, models.ErrAlreadyAssigned) {
		http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10), http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Printf("handlers: assign exercise %d to athlete %d: %v", exerciseID, athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10), http.StatusSeeOther)
}

// Deactivate removes an active assignment.
func (h *Assignments) Deactivate(w http.ResponseWriter, r *http.Request) {
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	assignmentID, err := strconv.ParseInt(r.PathValue("assignmentID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid assignment ID", http.StatusBadRequest)
		return
	}

	err = models.DeactivateAssignment(h.DB, assignmentID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Assignment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: deactivate assignment %d: %v", assignmentID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10), http.StatusSeeOther)
}

// AssignForm renders the form to assign an exercise to an athlete.
func (h *Assignments) AssignForm(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("handlers: get athlete %d for assign form: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	exercises, err := models.ListUnassignedExercises(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: list unassigned exercises for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":   athlete,
		"Exercises": exercises,
	}
	if err := h.Templates.ExecuteTemplate(w, "assign-exercise-form", data); err != nil {
		log.Printf("handlers: assign form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
