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

// Accessories holds dependencies for accessory plan handlers.
type Accessories struct {
	DB        *sql.DB
	Templates TemplateCache
}

// List renders the accessory plan management page for an athlete.
func (h *Accessories) List(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	plans, err := models.ListAllAccessoryPlans(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: list accessory plans for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	maxDay, err := models.MaxAccessoryDay(h.DB, athleteID)
	if err != nil {
		log.Printf("handlers: max accessory day for athlete %d: %v", athleteID, err)
	}

	exercises, err := models.ListExercises(h.DB, "")
	if err != nil {
		log.Printf("handlers: list exercises: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Group plans by day for template rendering.
	dayMap := make(map[int][]*models.AccessoryPlan)
	for _, p := range plans {
		dayMap[p.Day] = append(dayMap[p.Day], p)
	}

	user := middleware.UserFromContext(r.Context())

	data := map[string]any{
		"Athlete":    athlete,
		"Plans":      plans,
		"DayMap":     dayMap,
		"MaxDay":     maxDay,
		"Exercises":  exercises,
		"CanManage":  middleware.CanManageAthlete(user, athlete),
		"Error":      r.URL.Query().Get("error"),
		"Success":    r.URL.Query().Get("success"),
	}
	if err := h.Templates.Render(w, r, "accessory_plans.html", data); err != nil {
		log.Printf("handlers: accessory plans template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create adds a new accessory plan entry.
func (h *Accessories) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
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
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
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

	exerciseID, err := strconv.ParseInt(r.FormValue("exercise_id"), 10, 64)
	if err != nil || exerciseID < 1 {
		accessoryRedirectWithError(w, r, athleteID, "Exercise is required")
		return
	}

	day, err := strconv.Atoi(r.FormValue("day"))
	if err != nil || day < 1 {
		accessoryRedirectWithError(w, r, athleteID, "Day must be a positive number")
		return
	}

	targetSets, _ := strconv.Atoi(r.FormValue("target_sets"))
	targetRepMin, _ := strconv.Atoi(r.FormValue("target_rep_min"))
	targetRepMax, _ := strconv.Atoi(r.FormValue("target_rep_max"))
	targetWeight, _ := strconv.ParseFloat(r.FormValue("target_weight"), 64)
	notes := r.FormValue("notes")
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))

	_, err = models.CreateAccessoryPlan(h.DB, athleteID, day, exerciseID, targetSets, targetRepMin, targetRepMax, targetWeight, notes, sortOrder)
	if err != nil {
		log.Printf("handlers: create accessory plan: %v", err)
		accessoryRedirectWithError(w, r, athleteID, "Could not add accessory — it may already exist for that day")
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/accessories?success=Accessory+added", http.StatusSeeOther)
}

// Update modifies an existing accessory plan entry.
func (h *Accessories) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
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
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid plan ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	targetSets, _ := strconv.Atoi(r.FormValue("target_sets"))
	targetRepMin, _ := strconv.Atoi(r.FormValue("target_rep_min"))
	targetRepMax, _ := strconv.Atoi(r.FormValue("target_rep_max"))
	targetWeight, _ := strconv.ParseFloat(r.FormValue("target_weight"), 64)
	notes := r.FormValue("notes")
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))

	err = models.UpdateAccessoryPlan(h.DB, planID, targetSets, targetRepMin, targetRepMax, targetWeight, notes, sortOrder)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: update accessory plan %d: %v", planID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/accessories?success=Accessory+updated", http.StatusSeeOther)
}

// Deactivate sets an accessory plan to inactive.
func (h *Accessories) Deactivate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
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
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid plan ID", http.StatusBadRequest)
		return
	}

	err = models.DeactivateAccessoryPlan(h.DB, planID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: deactivate accessory plan %d: %v", planID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/accessories?success=Accessory+deactivated", http.StatusSeeOther)
}

// Delete permanently removes an accessory plan entry.
func (h *Accessories) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
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
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid plan ID", http.StatusBadRequest)
		return
	}

	err = models.DeleteAccessoryPlan(h.DB, planID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: delete accessory plan %d: %v", planID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/accessories?success=Accessory+removed", http.StatusSeeOther)
}

func accessoryRedirectWithError(w http.ResponseWriter, r *http.Request, athleteID int64, msg string) {
	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/accessories?error="+msg, http.StatusSeeOther)
}
