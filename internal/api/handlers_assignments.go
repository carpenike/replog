package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Exercise Assignments ---

// ListAssignments returns active exercise assignments for an athlete.
// ListAssignments returns exercise assignments for an athlete. By default
// returns only active assignments; pass `?include_inactive=true` to also
// include previously deactivated rows (latest per exercise — used by the
// SPA to offer reactivation).
//
//	@Summary      List exercise assignments
//	@Tags         Athletes
//	@Produce      json
//	@Param        id                path      int     true   "Athlete ID"
//	@Param        include_inactive  query     bool    false  "Include previously deactivated assignments"
//	@Success      200  {array}   api.AthleteExercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/assignments [get]
func (h *Handlers) ListAssignments(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	assignments, err := models.ListActiveAssignments(h.DB, athleteID)
	if err != nil {
		log.Printf("api: list assignments for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list assignments")
		return
	}

	if r.URL.Query().Get("include_inactive") == "true" {
		inactive, err := models.ListDeactivatedAssignments(h.DB, athleteID)
		if err != nil {
			log.Printf("api: list deactivated assignments for athlete %d: %v", athleteID, err)
			WriteError(w, http.StatusInternalServerError, "failed to list assignments")
			return
		}
		assignments = append(assignments, inactive...)
	}

	result := make([]*AthleteExercise, len(assignments))
	for i, a := range assignments {
		result[i] = AthleteExerciseFromModel(a)
	}
	WriteJSON(w, http.StatusOK, result)
}

// AssignExercise assigns an exercise to an athlete. Coach only.
//
//	@Summary      Assign exercise to athlete
//	@Description  Schema enforces UNIQUE WHERE active=1 — a duplicate active assignment for the same exercise is rejected.
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Athlete ID"
//	@Param        body  body      api.AssignExerciseRequest  true  "Assignment"
//	@Success      201  {object}  api.AthleteExercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/assignments [post]
func (h *Handlers) AssignExercise(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req AssignExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id is required")
		return
	}

	assignment, err := models.AssignExercise(h.DB, athleteID, req.ExerciseID, req.TargetReps)
	if err != nil {
		log.Printf("api: assign exercise to athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to assign exercise")
		return
	}

	WriteJSON(w, http.StatusCreated, AthleteExerciseFromModel(assignment))
}

// DeactivateAssignment deactivates an exercise assignment. Coach only.
//
//	@Summary      Deactivate exercise assignment
//	@Tags         Athletes
//	@Produce      json
//	@Param        id            path      int  true  "Athlete ID"
//	@Param        assignmentID  path      int  true  "Assignment ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/assignments/{assignmentID}/deactivate [post]
func (h *Handlers) DeactivateAssignment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	assignmentID, err := strconv.ParseInt(r.PathValue("assignmentID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid assignment ID")
		return
	}

	if err := models.DeactivateAssignment(h.DB, assignmentID); err != nil {
		log.Printf("api: deactivate assignment %d: %v", assignmentID, err)
		WriteError(w, http.StatusInternalServerError, "failed to deactivate assignment")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Program Compatibility ---

// CompatibilityResponse shows equipment compatibility for a program.
type CompatibilityResponse struct {
	TemplateID   int64                         `json:"template_id"`
	TemplateName string                        `json:"template_name"`
	Ready        bool                          `json:"ready"`
	ReadyCount   int                           `json:"ready_count"`
	TotalCount   int                           `json:"total_count"`
	Exercises    []ExerciseCompatibilityResponse `json:"exercises"`
}

// ExerciseCompatibilityResponse shows one exercise's equipment status.
type ExerciseCompatibilityResponse struct {
	ExerciseID   int64  `json:"exercise_id"`
	ExerciseName string `json:"exercise_name"`
	HasRequired  bool   `json:"has_required"`
}

// CheckProgramCompatibility checks equipment compatibility for a program assignment.
//
//	@Summary      Check program equipment compatibility for an athlete
//	@Tags         Programs
//	@Produce      json
//	@Param        id           path      int  true  "Athlete ID"
//	@Param        template_id  query     int  true  "Program template ID"
//	@Success      200  {object}  api.CompatibilityResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/program-compatibility [get]
func (h *Handlers) CheckProgramCompatibility(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	templateID, err := strconv.ParseInt(r.URL.Query().Get("template_id"), 10, 64)
	if err != nil || templateID == 0 {
		WriteError(w, http.StatusBadRequest, "template_id query parameter required")
		return
	}

	compat, err := models.CheckProgramCompatibility(h.DB, athleteID, templateID)
	if err != nil {
		log.Printf("api: check compatibility athlete %d template %d: %v", athleteID, templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to check compatibility")
		return
	}

	exercises := make([]ExerciseCompatibilityResponse, len(compat.Exercises))
	for i, e := range compat.Exercises {
		exercises[i] = ExerciseCompatibilityResponse{
			ExerciseID:   e.ExerciseID,
			ExerciseName: e.ExerciseName,
			HasRequired:  e.HasRequired,
		}
	}

	WriteJSON(w, http.StatusOK, CompatibilityResponse{
		TemplateID:   compat.TemplateID,
		TemplateName: compat.TemplateName,
		Ready:        compat.Ready,
		ReadyCount:   compat.ReadyCount,
		TotalCount:   compat.TotalCount,
		Exercises:    exercises,
	})
}

// ReactivateAssignment reactivates a deactivated exercise assignment.
//
//	@Summary      Reactivate exercise assignment
//	@Description  Creates a fresh active row for the athlete+exercise pair (the previous one stays as history).
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Athlete ID"
//	@Param        body  body      api.AssignExerciseRequest  true  "Assignment"
//	@Success      200  {object}  api.AthleteExercise
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/assignments/reactivate [post]
func (h *Handlers) ReactivateAssignment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	var req AssignExerciseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id is required")
		return
	}

	assignment, err := models.ReactivateAssignment(h.DB, athleteID, req.ExerciseID, req.TargetReps)
	if err != nil {
		log.Printf("api: reactivate assignment athlete %d exercise %d: %v", athleteID, req.ExerciseID, err)
		WriteError(w, http.StatusInternalServerError, "failed to reactivate assignment")
		return
	}

	WriteJSON(w, http.StatusOK, AthleteExerciseFromModel(assignment))
}
