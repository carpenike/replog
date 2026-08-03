package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Review Submission ---

// SubmitReview creates or updates a workout review. Coach only.
//
//	@Summary      Submit workout review
//	@Tags         Reviews
//	@Accept       json
//	@Produce      json
//	@Param        id         path      int                 true  "Athlete ID"
//	@Param        workoutID  path      int                 true  "Workout ID"
//	@Param        body       body      api.ReviewRequest   true  "Review"
//	@Success      200  {object}  api.WorkoutReview
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/review [post]
func (h *Handlers) SubmitReview(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	// The workout ID is global; confirm it belongs to the path athlete so a
	// coach cannot attach a review to a workout of an athlete they can access
	// in the path but which actually belongs to someone else.
	workout, err := models.GetWorkoutByID(r.Context(), h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) || (err == nil && workout.AthleteID != athleteID) {
		WriteError(w, http.StatusNotFound, "workout not found")
		return
	}
	if err != nil {
		log.Printf("api: get workout %d for review: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to submit review")
		return
	}

	var req ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status != "approved" && req.Status != "needs_work" {
		WriteError(w, http.StatusBadRequest, "status must be 'approved' or 'needs_work'")
		return
	}

	review, err := models.CreateOrUpdateWorkoutReview(r.Context(), h.DB, workoutID, user.ID, req.Status, req.Notes)
	if err != nil {
		log.Printf("api: submit review for workout %d: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to submit review")
		return
	}

	// Notify the athlete's linked user that a review landed (ADR 008).
	var title string
	if req.Status == "approved" {
		title = "Workout approved"
	} else {
		title = "Workout needs work"
	}
	h.notifyAthlete(r.Context(), athleteID, models.NotifyReviewSubmitted, title, req.Notes,
		fmt.Sprintf("/athletes/%d/workouts/%d", athleteID, workoutID))

	WriteJSON(w, http.StatusOK, WorkoutReviewFromModel(review))
}

// --- Program Assignment ---

// AssignProgramToAthlete assigns a program template to an athlete. Coach only.
//
//	@Summary      Assign program to athlete
//	@Description  Auto-deactivates any existing active program in the same role (per ADR 010).
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                       true  "Athlete ID"
//	@Param        body  body      api.AssignProgramRequest  true  "Assignment"
//	@Success      201  {object}  api.AthleteProgram
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/programs [post]
func (h *Handlers) AssignProgramToAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req AssignProgramRequest
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

	ap, err := models.ReplaceProgram(r.Context(), h.DB, athleteID, req.TemplateID, req.StartDate, req.Notes, req.Goal, req.Role, req.Schedule)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrInvalidSchedule):
			WriteValidationError(w, "schedule", err.Error())
		case errors.Is(err, models.ErrInvalidProgramRole):
			WriteValidationError(w, "role", err.Error())
		case errors.Is(err, models.ErrScheduleConflict):
			WriteValidationError(w, "schedule", "conflicts with an existing active program")
		case errors.Is(err, models.ErrProgramAlreadyActive):
			WriteError(w, http.StatusConflict, "another active program already exists")
		default:
			log.Printf("api: assign program to athlete %d: %v", athleteID, err)
			WriteError(w, http.StatusInternalServerError, "failed to assign program")
		}
		return
	}

	// Notify the athlete that a new program was assigned (ADR 008). Look up
	// the template name for the title; fall back to a generic message if
	// the lookup fails (the notify is best-effort).
	programName := "a new program"
	if tpl, terr := models.GetProgramTemplateByID(r.Context(), h.DB, req.TemplateID); terr == nil && tpl != nil {
		programName = tpl.Name
	}
	h.notifyAthlete(r.Context(), athleteID, models.NotifyProgramAssigned,
		"New program assigned",
		fmt.Sprintf("%s — starting %s", programName, req.StartDate),
		fmt.Sprintf("/athletes/%d/programs", athleteID))

	WriteJSON(w, http.StatusCreated, AthleteProgramFromModel(ap))
}

// DeactivateAthleteProgram deactivates an athlete's program. Coach only.
//
//	@Summary      Deactivate athlete program
//	@Tags         Programs
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        programID  path      int  true  "Program assignment ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/programs/{programID}/deactivate [post]
func (h *Handlers) DeactivateAthleteProgram(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	programID, err := strconv.ParseInt(r.PathValue("programID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	// The program assignment ID is global; verify it belongs to the path
	// athlete so it cannot be deactivated cross-athlete.
	if prog, perr := models.GetAthleteProgramByID(r.Context(), h.DB, programID); errors.Is(perr, models.ErrNotFound) || (perr == nil && prog.AthleteID != athleteID) {
		WriteError(w, http.StatusNotFound, "program assignment not found")
		return
	} else if perr != nil {
		log.Printf("api: get program %d for deactivate: %v", programID, perr)
		WriteError(w, http.StatusInternalServerError, "failed to deactivate program")
		return
	}

	if err := models.DeactivateProgram(r.Context(), h.DB, programID); err != nil {
		log.Printf("api: deactivate program %d: %v", programID, err)
		WriteError(w, http.StatusInternalServerError, "failed to deactivate program")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReactivateAthleteProgram reactivates a deactivated program assignment.
// POST /api/athletes/{id}/programs/{programID}/reactivate
//
//	@Summary      Reactivate athlete program
//	@Description  Auto-deactivates any other active program in the same role.
//	@Tags         Programs
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        programID  path      int  true  "Program assignment ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/programs/{programID}/reactivate [post]
func (h *Handlers) ReactivateAthleteProgram(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	programID, err := strconv.ParseInt(r.PathValue("programID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	// Get the program to check its role and auto-deactivate conflicts. Also
	// enforce that it belongs to the path athlete (cross-athlete guard).
	program, err := models.GetAthleteProgramByID(r.Context(), h.DB, programID)
	if err != nil || program.AthleteID != athleteID {
		WriteError(w, http.StatusNotFound, "program assignment not found")
		return
	}

	// Deactivate any existing active program in the same role.
	existingPrograms, _ := models.ListAthletePrograms(r.Context(), h.DB, athleteID)
	for _, p := range existingPrograms {
		if p.Active && p.Role == program.Role && p.ID != programID {
			_ = models.DeactivateProgram(r.Context(), h.DB, p.ID)
		}
	}

	if err := models.ReactivateProgram(r.Context(), h.DB, programID); err != nil {
		log.Printf("api: reactivate program %d: %v", programID, err)
		WriteError(w, http.StatusInternalServerError, "failed to reactivate program")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteAthleteProgram removes a program assignment entirely.
// DELETE /api/athletes/{id}/programs/{programID}
//
//	@Summary      Delete athlete program assignment
//	@Tags         Programs
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        programID  path      int  true  "Program assignment ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/programs/{programID} [delete]
func (h *Handlers) DeleteAthleteProgram(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	programID, err := strconv.ParseInt(r.PathValue("programID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	// Verify the assignment belongs to the path athlete (cross-athlete guard).
	if prog, perr := models.GetAthleteProgramByID(r.Context(), h.DB, programID); errors.Is(perr, models.ErrNotFound) || (perr == nil && prog.AthleteID != athleteID) {
		WriteError(w, http.StatusNotFound, "program assignment not found")
		return
	} else if perr != nil {
		log.Printf("api: get program %d for delete: %v", programID, perr)
		WriteError(w, http.StatusInternalServerError, "failed to delete program assignment")
		return
	}

	if err := models.DeleteAthleteProgram(r.Context(), h.DB, programID); err != nil {
		log.Printf("api: delete athlete program %d: %v", programID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete program assignment")
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
// ListAccessoryPlans returns all accessory plans for an athlete.
//
//	@Summary      List accessory plans
//	@Tags         Athletes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {array}   api.AccessoryPlan
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/accessories [get]
func (h *Handlers) ListAccessoryPlans(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	plans, err := models.ListAllAccessoryPlans(r.Context(), h.DB, athleteID)
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
// CreateAccessoryPlan creates a new accessory plan for an athlete.
//
//	@Summary      Create accessory plan
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                       true  "Athlete ID"
//	@Param        body  body      api.AccessoryPlanRequest  true  "Plan"
//	@Success      201  {object}  api.AccessoryPlan
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/accessories [post]
func (h *Handlers) CreateAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req AccessoryPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id is required")
		return
	}

	plan, err := models.CreateAccessoryPlan(r.Context(), h.DB, athleteID, req.Day, req.ExerciseID, req.TargetSets, req.TargetRepMin, req.TargetRepMax, req.TargetWeight, req.Notes, req.SortOrder)
	if err != nil {
		log.Printf("api: create accessory plan for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create accessory plan")
		return
	}

	WriteJSON(w, http.StatusCreated, AccessoryPlanFromModel(plan))
}

// DeleteAccessoryPlan deletes an accessory plan. Coach only.
// DeleteAccessoryPlan permanently removes an accessory plan.
//
//	@Summary      Delete accessory plan
//	@Tags         Athletes
//	@Produce      json
//	@Param        id      path      int  true  "Athlete ID"
//	@Param        planID  path      int  true  "Plan ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/accessories/{planID} [delete]
func (h *Handlers) DeleteAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	if err := models.DeleteAccessoryPlan(r.Context(), h.DB, planID, athleteID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "accessory plan not found")
			return
		}
		log.Printf("api: delete accessory plan %d: %v", planID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete accessory plan")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateAccessoryPlan updates an accessory plan.
// UpdateAccessoryPlan updates an accessory plan.
//
//	@Summary      Update accessory plan
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id      path      int                              true  "Athlete ID"
//	@Param        planID  path      int                              true  "Plan ID"
//	@Param        body    body      api.AccessoryPlanUpdateRequest   true  "Plan"
//	@Success      200  {object}  api.AccessoryPlan
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/accessories/{planID} [put]
func (h *Handlers) UpdateAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	var req AccessoryPlanUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := models.UpdateAccessoryPlan(r.Context(), h.DB, planID, athleteID, req.TargetSets, req.TargetRepMin, req.TargetRepMax, req.TargetWeight, req.Notes, req.SortOrder); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "accessory plan not found")
			return
		}
		log.Printf("api: update accessory plan %d: %v", planID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update plan")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeactivateAccessoryPlan deactivates an accessory plan.
// DeactivateAccessoryPlan marks an accessory plan as inactive.
//
//	@Summary      Deactivate accessory plan
//	@Tags         Athletes
//	@Produce      json
//	@Param        id      path      int  true  "Athlete ID"
//	@Param        planID  path      int  true  "Plan ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/accessories/{planID}/deactivate [post]
func (h *Handlers) DeactivateAccessoryPlan(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	planID, err := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid plan ID")
		return
	}

	if err := models.DeactivateAccessoryPlan(r.Context(), h.DB, planID, athleteID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "accessory plan not found")
			return
		}
		log.Printf("api: deactivate accessory plan %d: %v", planID, err)
		WriteError(w, http.StatusInternalServerError, "failed to deactivate plan")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteReview deletes a workout review.
//
//	@Summary      Delete workout review
//	@Tags         Reviews
//	@Produce      json
//	@Param        id         path      int  true  "Athlete ID"
//	@Param        workoutID  path      int  true  "Workout ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/review [delete]
func (h *Handlers) DeleteReview(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return
	}

	// Ensure the workout (and thus its review) belongs to the path athlete —
	// the workout ID is global and must not be reachable cross-athlete.
	workout, err := models.GetWorkoutByID(r.Context(), h.DB, workoutID)
	if errors.Is(err, models.ErrNotFound) || (err == nil && workout.AthleteID != athleteID) {
		WriteError(w, http.StatusNotFound, "review not found")
		return
	}
	if err != nil {
		log.Printf("api: get workout %d for delete review: %v", workoutID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete review")
		return
	}

	review, err := models.GetWorkoutReviewByWorkoutID(r.Context(), h.DB, workoutID)
	if err != nil {
		log.Printf("api: get review for workout %d: %v", workoutID, err)
		WriteError(w, http.StatusNotFound, "review not found")
		return
	}

	if err := models.DeleteWorkoutReview(r.Context(), h.DB, review.ID); err != nil {
		log.Printf("api: delete review %d: %v", review.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete review")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
