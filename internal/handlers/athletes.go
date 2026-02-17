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

// Athletes holds dependencies for athlete handlers.
type Athletes struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders the athlete list page. Coaches see all athletes; non-coaches
// are redirected to their own athlete profile.
func (h *Athletes) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		if user.AthleteID.Valid {
			http.Redirect(w, r, "/athletes/"+strconv.FormatInt(user.AthleteID.Int64, 10), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		return
	}

	athletes, err := models.ListAthleteCards(h.DB)
	if err != nil {
		log.Printf("handlers: list athlete cards: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athletes": athletes,
	}
	if err := h.Templates.Render(w, r, "athletes_list.html", data); err != nil {
		log.Printf("handlers: athletes list template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// NewForm renders the new athlete form. Coach only.
func (h *Athletes) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := map[string]any{
		"Tiers": tierOptions(),
	}
	if err := h.Templates.Render(w, r, "athlete_form.html", data); err != nil {
		log.Printf("handlers: athlete new form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create processes the new athlete form submission. Coach only.
func (h *Athletes) Create(w http.ResponseWriter, r *http.Request) {
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
		h.Templates.Render(w, r, "athlete_form.html", data)
		return
	}

	athlete, err := models.CreateAthlete(h.DB, name, r.FormValue("tier"), r.FormValue("notes"))
	if err != nil {
		log.Printf("handlers: create athlete: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athlete.ID, 10), http.StatusSeeOther)
}

// Show renders the athlete detail page.
func (h *Athletes) Show(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(user, id) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	assignments, err := models.ListActiveAssignments(h.DB, id)
	if err != nil {
		log.Printf("handlers: list assignments for athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	currentTMs, err := models.ListCurrentTrainingMaxes(h.DB, id)
	if err != nil {
		log.Printf("handlers: list training maxes for athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build a map from exerciseID → current training max for easy lookup.
	tmByExercise := make(map[int64]*models.TrainingMax)
	for _, tm := range currentTMs {
		tmByExercise[tm.ExerciseID] = tm
	}

	// Load recent workouts for the athlete detail page.
	recentPage, err := models.ListWorkouts(h.DB, id, 0)
	if err != nil {
		log.Printf("handlers: list workouts for athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	recentWorkouts := recentPage.Workouts
	// Limit to 5 most recent for the summary view.
	if len(recentWorkouts) > 5 {
		recentWorkouts = recentWorkouts[:5]
	}

	// Load deactivated assignments for reactivation UI (coach only).
	var deactivated []*models.AthleteExercise
	if user.IsCoach {
		deactivated, err = models.ListDeactivatedAssignments(h.DB, id)
		if err != nil {
			log.Printf("handlers: list deactivated assignments for athlete %d: %v", id, err)
			// Non-fatal — continue without deactivated list.
		}
	}

	// Load latest body weight for the summary card.
	latestWeight, err := models.LatestBodyWeight(h.DB, id)
	if err != nil {
		log.Printf("handlers: latest body weight for athlete %d: %v", id, err)
		// Non-fatal — continue without weight data.
	}

	// Load weekly completion streaks (last 8 weeks).
	streaks, err := models.WeeklyStreaks(h.DB, id, 8)
	if err != nil {
		log.Printf("handlers: weekly streaks for athlete %d: %v", id, err)
		// Non-fatal — continue without streak data.
	}

	// Load workout frequency heatmap (last 52 weeks).
	heatmap, err := models.WorkoutHeatmap(h.DB, id)
	if err != nil {
		log.Printf("handlers: workout heatmap for athlete %d: %v", id, err)
		// Non-fatal — continue without heatmap data.
	}

	// Load active program and today's prescription.
	activeProgram, err := models.GetActiveProgram(h.DB, id)
	if err != nil {
		log.Printf("handlers: active program for athlete %d: %v", id, err)
		// Non-fatal — continue without program data.
	}

	var prescription *models.Prescription
	if activeProgram != nil {
		prescription, err = models.GetPrescription(h.DB, id, time.Now())
		if err != nil {
			log.Printf("handlers: prescription for athlete %d: %v", id, err)
			// Non-fatal — continue without prescription data.
		}
	}

	// Load available program templates for assignment (coach only).
	var programTemplates []*models.ProgramTemplate
	if user.IsCoach {
		programTemplates, err = models.ListProgramTemplates(h.DB)
		if err != nil {
			log.Printf("handlers: list program templates for athlete %d: %v", id, err)
		}
	}

	data := map[string]any{
		"Athlete":          athlete,
		"Assignments":      assignments,
		"TMByExercise":     tmByExercise,
		"RecentWorkouts":   recentWorkouts,
		"Deactivated":      deactivated,
		"LatestWeight":     latestWeight,
		"Streaks":          streaks,
		"Heatmap":          heatmap,
		"ActiveProgram":    activeProgram,
		"Prescription":     prescription,
		"ProgramTemplates": programTemplates,
		"TodayDate":        time.Now().Format("2006-01-02"),
	}
	if err := h.Templates.Render(w, r, "athlete_detail.html", data); err != nil {
		log.Printf("handlers: athlete detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// EditForm renders the edit athlete form. Coach only.
func (h *Athletes) EditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for edit: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
		"Tiers":   tierOptions(),
	}
	if err := h.Templates.Render(w, r, "athlete_form.html", data); err != nil {
		log.Printf("handlers: athlete edit form template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update processes the edit athlete form submission. Coach only.
func (h *Athletes) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		athlete, _ := models.GetAthleteByID(h.DB, id)
		data := map[string]any{
			"Error":   "Name is required",
			"Athlete": athlete,
			"Tiers":   tierOptions(),
			"Form":    r.Form,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.Templates.Render(w, r, "athlete_form.html", data)
		return
	}

	_, err = models.UpdateAthlete(h.DB, id, name, r.FormValue("tier"), r.FormValue("notes"))
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: update athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// Delete removes an athlete. Coach only.
func (h *Athletes) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteAthlete(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: delete athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes", http.StatusSeeOther)
}

// Promote advances an athlete to the next tier. Coach only.
func (h *Athletes) Promote(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	_, err = models.PromoteAthlete(h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, models.ErrInvalidInput) {
		log.Printf("handlers: promote athlete %d: %v", id, err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		log.Printf("handlers: promote athlete %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func tierOptions() []struct{ Value, Label string } {
	return []struct{ Value, Label string }{
		{"", "— None —"},
		{"foundational", "Foundational"},
		{"intermediate", "Intermediate"},
		{"sport_performance", "Sport Performance"},
	}
}
