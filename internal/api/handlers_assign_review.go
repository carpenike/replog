package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Review Submission ---

// SubmitReview creates or updates a workout review. Coach only.
func (h *Handlers) SubmitReview(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status != "approved" && req.Status != "needs_work" {
		WriteError(w, http.StatusBadRequest, "status must be 'approved' or 'needs_work'")
		return
	}

	review, err := models.CreateOrUpdateWorkoutReview(h.DB, workoutID, user.ID, req.Status, req.Notes)
	if err != nil {
		log.Printf("api: submit review for workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to submit review")
		return
	}

	WriteJSON(w, http.StatusOK, WorkoutReviewFromModel(review))
}

// --- Program Assignment ---

// AssignProgramToAthlete assigns a program template to an athlete. Coach only.
func (h *Handlers) AssignProgramToAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req struct {
		TemplateID int64  `json:"template_id"`
		StartDate  string `json:"start_date"`
		Notes      string `json:"notes"`
		Goal       string `json:"goal"`
		Role       string `json:"role"`
		Schedule   string `json:"schedule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TemplateID == 0 || req.StartDate == "" {
		WriteError(w, http.StatusBadRequest, "template_id and start_date are required")
		return
	}
	if req.Role == "" {
		req.Role = "primary"
	}

	ap, err := models.AssignProgram(h.DB, athleteID, req.TemplateID, req.StartDate, req.Notes, req.Goal, req.Role, req.Schedule)
	if err != nil {
		log.Printf("api: assign program to athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to assign program")
		return
	}

	WriteJSON(w, http.StatusCreated, AthleteProgramFromModel(ap))
}

// DeactivateAthleteProgram deactivates an athlete's program. Coach only.
func (h *Handlers) DeactivateAthleteProgram(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	programID, err := strconv.ParseInt(r.PathValue("programID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	if err := models.DeactivateProgram(h.DB, programID); err != nil {
		log.Printf("api: deactivate program %d: %v", programID, err)
		WriteError(w, http.StatusInternalServerError, "failed to deactivate program")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Accessory Plans ---

// AccessoryPlanFromModel converts a models.AccessoryPlan to an API AccessoryPlan.
func AccessoryPlanFromModel(m *models.AccessoryPlan) *AccessoryPlan {
	return &AccessoryPlan{
		ID:           m.ID,
		AthleteID:    m.AthleteID,
		Day:          m.Day,
		ExerciseID:   m.ExerciseID,
		TargetSets:   nullInt(m.TargetSets),
		TargetRepMin: nullInt(m.TargetRepMin),
		TargetRepMax: nullInt(m.TargetRepMax),
		TargetWeight: nullFloat(m.TargetWeight),
		Notes:        nullStr(m.Notes),
		SortOrder:    m.SortOrder,
		Active:       m.Active,
		CreatedAt:    fmtTime(m.CreatedAt),
		UpdatedAt:    fmtTime(m.UpdatedAt),
		ExerciseName: m.ExerciseName,
	}
}

// ListAccessoryPlans returns all accessory plans for an athlete.
func (h *Handlers) ListAccessoryPlans(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	plans, err := models.ListAllAccessoryPlans(h.DB, athleteID)
	if err != nil {
		log.Printf("api: list accessory plans for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list accessory plans")
		return
	}

	result := make([]*AccessoryPlan, len(plans))
	for i, p := range plans {
		result[i] = AccessoryPlanFromModel(p)
	}
	WriteJSON(w, http.StatusOK, result)
}

// CreateAccessoryPlan creates a new accessory plan. Coach only.
func (h *Handlers) CreateAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	var req struct {
		Day          int     `json:"day"`
		ExerciseID   int64   `json:"exercise_id"`
		TargetSets   int     `json:"target_sets"`
		TargetRepMin int     `json:"target_rep_min"`
		TargetRepMax int     `json:"target_rep_max"`
		TargetWeight float64 `json:"target_weight"`
		Notes        string  `json:"notes"`
		SortOrder    int     `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id is required")
		return
	}

	plan, err := models.CreateAccessoryPlan(h.DB, athleteID, req.Day, req.ExerciseID, req.TargetSets, req.TargetRepMin, req.TargetRepMax, req.TargetWeight, req.Notes, req.SortOrder)
	if err != nil {
		log.Printf("api: create accessory plan for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create accessory plan")
		return
	}

	WriteJSON(w, http.StatusCreated, AccessoryPlanFromModel(plan))
}

// DeleteAccessoryPlan deletes an accessory plan. Coach only.
func (h *Handlers) DeleteAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	if err := models.DeleteAccessoryPlan(h.DB, planID); err != nil {
		log.Printf("api: delete accessory plan %d: %v", planID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete accessory plan")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
