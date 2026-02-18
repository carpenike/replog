package handlers

import (
	"database/sql"
	"errors"
	"fmt"
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

// List renders the athlete list page. Coaches see their own athletes;
// admins see all athletes; non-coaches are redirected to their own athlete profile.
func (h *Athletes) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		if user.AthleteID.Valid {
			http.Redirect(w, r, "/athletes/"+strconv.FormatInt(user.AthleteID.Int64, 10), http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		return
	}

	coachFilter := middleware.CoachAthleteFilter(user)
	athletes, err := models.ListAthleteCards(h.DB, coachFilter)
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

// NewForm renders the new athlete form. Coach or admin only.
func (h *Athletes) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

// Create processes the new athlete form submission. Coach or admin only.
func (h *Athletes) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		h.Templates.Forbidden(w, r)
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

	trackBW := r.FormValue("track_body_weight") != "0"
	athlete, err := models.CreateAthlete(h.DB, name, r.FormValue("tier"), r.FormValue("notes"), r.FormValue("goal"), sql.NullInt64{Int64: user.ID, Valid: true}, trackBW)
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

	// Check access: admins see all, coaches see their own athletes, non-coaches see own profile.
	if !user.IsAdmin {
		if user.IsCoach {
			// Coach can view own linked athlete profile or athletes they coach.
			ownProfile := user.AthleteID.Valid && user.AthleteID.Int64 == id
			ownsAthlete := athlete.CoachID.Valid && athlete.CoachID.Int64 == user.ID
			if !ownProfile && !ownsAthlete {
				h.Templates.Forbidden(w, r)
				return
			}
		} else if !user.AthleteID.Valid || user.AthleteID.Int64 != id {
			h.Templates.Forbidden(w, r)
			return
		}
	}

	data, err := h.loadAthleteShowData(user, athlete)
	if err != nil {
		log.Printf("handlers: load athlete show data %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.Templates.Render(w, r, "athlete_detail.html", data); err != nil {
		log.Printf("handlers: athlete detail template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// loadAthleteShowData fetches all data needed for the athlete detail page.
// Fatal queries return errors; non-fatal queries log and continue with nil/zero values.
func (h *Athletes) loadAthleteShowData(user *models.User, athlete *models.Athlete) (map[string]any, error) {
	id := athlete.ID

	assignments, err := models.ListActiveAssignments(h.DB, id)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}

	currentTMs, err := models.ListCurrentTrainingMaxes(h.DB, id)
	if err != nil {
		return nil, fmt.Errorf("list training maxes: %w", err)
	}

	// Build a map from exerciseID → current training max for easy lookup.
	tmByExercise := make(map[int64]*models.TrainingMax)
	for _, tm := range currentTMs {
		tmByExercise[tm.ExerciseID] = tm
	}

	// Load recent workouts for the athlete detail page.
	recentPage, err := models.ListWorkouts(h.DB, id, 0)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}
	recentWorkouts := recentPage.Workouts
	// Limit to 5 most recent for the summary view.
	if len(recentWorkouts) > 5 {
		recentWorkouts = recentWorkouts[:5]
	}

	// Load deactivated assignments for reactivation UI (coach/admin only for managed athletes).
	var deactivated []*models.AthleteExercise
	if middleware.CanManageAthlete(user, athlete) {
		deactivated, err = models.ListDeactivatedAssignments(h.DB, id)
		if err != nil {
			log.Printf("handlers: list deactivated assignments for athlete %d: %v", id, err)
			// Non-fatal — continue without deactivated list.
		}
	}

	// Load latest body weight for the summary card (only if tracking is enabled).
	var latestWeight *models.BodyWeight
	if athlete.TrackBodyWeight {
		latestWeight, err = models.LatestBodyWeight(h.DB, id)
		if err != nil {
			log.Printf("handlers: latest body weight for athlete %d: %v", id, err)
			// Non-fatal — continue without weight data.
		}
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

	// Check for missing TMs and equipment gaps in the active program.
	var missingTMs []*models.MissingProgramTM
	var missingEquip []models.EquipmentCompatibility
	if activeProgram != nil && middleware.CanManageAthlete(user, athlete) {
		missingTMs, err = models.ListMissingProgramTMs(h.DB, activeProgram.TemplateID, id)
		if err != nil {
			log.Printf("handlers: missing program TMs for athlete %d: %v", id, err)
		}
		equipCompat, err := models.CheckProgramCompatibility(h.DB, id, activeProgram.TemplateID)
		if err != nil {
			log.Printf("handlers: program equipment compat for athlete %d: %v", id, err)
		} else if equipCompat != nil && !equipCompat.Ready {
			for _, ex := range equipCompat.Exercises {
				if !ex.HasRequired {
					missingEquip = append(missingEquip, ex)
				}
			}
		}
	}

	// Load available program templates for assignment (coach/admin only for managed athletes).
	var programTemplates []*models.ProgramTemplate
	if middleware.CanManageAthlete(user, athlete) {
		programTemplates, err = models.ListProgramTemplates(h.DB)
		if err != nil {
			log.Printf("handlers: list program templates for athlete %d: %v", id, err)
		}
	}

	// Load featured lifts (current TM, personal best, estimated 1RM).
	featuredLifts, err := models.ListFeaturedLifts(h.DB, id)
	if err != nil {
		log.Printf("handlers: featured lifts for athlete %d: %v", id, err)
		// Non-fatal — continue without featured data.
	}

	return map[string]any{
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
		"FeaturedLifts":    featuredLifts,
		"MissingTMs":       missingTMs,
		"MissingEquip":     missingEquip,
		"CanManage":        middleware.CanManageAthlete(user, athlete),
		"TodayDate":        time.Now().Format("2006-01-02"),
	}, nil
}

// EditForm renders the edit athlete form. Coach (owns athlete) or admin only.
func (h *Athletes) EditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

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

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
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

// Update processes the edit athlete form submission. Coach (owns athlete) or admin only.
func (h *Athletes) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

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
		log.Printf("handlers: get athlete %d for update: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		athlete, err := models.GetAthleteByID(h.DB, id)
		if err != nil {
			log.Printf("handlers: get athlete %d for edit form: %v", id, err)
		}
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

	_, err = models.UpdateAthlete(h.DB, id, name, r.FormValue("tier"), r.FormValue("notes"), r.FormValue("goal"), athlete.CoachID, r.FormValue("track_body_weight") == "1")
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

// Delete removes an athlete. Coach (owns athlete) or admin only.
func (h *Athletes) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

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
		log.Printf("handlers: get athlete %d for delete: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
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

// Promote advances an athlete to the next tier. Coach (owns athlete) or admin only.
func (h *Athletes) Promote(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

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
		log.Printf("handlers: get athlete %d for promote: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
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
