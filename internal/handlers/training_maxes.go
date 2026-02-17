package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// TrainingMaxes holds dependencies for training max handlers.
type TrainingMaxes struct {
	DB        *sql.DB
	Templates TemplateCache
}

// NewForm renders the form to set a new training max. Coach/admin only.
func (h *TrainingMaxes) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		h.Templates.Forbidden(w, r)
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
		log.Printf("handlers: get athlete %d for TM form: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	exercise, err := models.GetExerciseByID(h.DB, exerciseID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get exercise %d for TM form: %v", exerciseID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":  athlete,
		"Exercise": exercise,
		"Today":    time.Now().Format("2006-01-02"),
	}
	if err := h.Templates.Render(w, r, "training_max_form.html", data); err != nil {
		log.Printf("handlers: training max form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new training max form submission. Coach/admin only.
func (h *TrainingMaxes) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		h.Templates.Forbidden(w, r)
		return
	}

	exerciseID, err := strconv.ParseInt(r.PathValue("exerciseID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid exercise ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	weightStr := r.FormValue("weight")
	if weightStr == "" {
		h.renderFormWithError(w, r, athleteID, exerciseID, "Weight is required")
		return
	}

	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil || weight <= 0 {
		h.renderFormWithError(w, r, athleteID, exerciseID, "Weight must be a positive number")
		return
	}

	effectiveDate := r.FormValue("effective_date")
	if effectiveDate == "" {
		effectiveDate = time.Now().Format("2006-01-02")
	}

	notes := r.FormValue("notes")

	_, err = models.SetTrainingMax(h.DB, athleteID, exerciseID, weight, effectiveDate, notes)
	if err != nil {
		log.Printf("handlers: set training max: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10), http.StatusSeeOther)
}

// History renders training max history for an athlete+exercise.
func (h *TrainingMaxes) History(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		h.Templates.Forbidden(w, r)
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
		log.Printf("handlers: get athlete %d for TM history: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	exercise, err := models.GetExerciseByID(h.DB, exerciseID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Exercise not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get exercise %d for TM history: %v", exerciseID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	history, err := models.ListTrainingMaxHistory(h.DB, athleteID, exerciseID)
	if err != nil {
		log.Printf("handlers: list training max history: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load chart data for TM progression.
	prefs := middleware.PrefsFromContext(r.Context())
	unit := models.DefaultWeightUnit
	if prefs != nil {
		unit = prefs.WeightUnit
	}
	chartData, chartErr := models.TrainingMaxChartData(h.DB, athleteID, exerciseID, unit)
	if chartErr != nil {
		log.Printf("handlers: TM chart for athlete %d exercise %d: %v", athleteID, exerciseID, chartErr)
	}

	data := map[string]any{
		"Athlete":  athlete,
		"Exercise": exercise,
		"History":  history,
		"Chart":    chartData,
	}
	if err := h.Templates.Render(w, r, "training_max_history.html", data); err != nil {
		log.Printf("handlers: training max history template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// renderFormWithError re-renders the TM form with an error message.
func (h *TrainingMaxes) renderFormWithError(w http.ResponseWriter, r *http.Request, athleteID, exerciseID int64, errMsg string) {
	athlete, _ := models.GetAthleteByID(h.DB, athleteID)
	exercise, _ := models.GetExerciseByID(h.DB, exerciseID)
	data := map[string]any{
		"Athlete":  athlete,
		"Exercise": exercise,
		"Today":    time.Now().Format("2006-01-02"),
		"Error":    errMsg,
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	h.Templates.Render(w, r, "training_max_form.html", data)
}
