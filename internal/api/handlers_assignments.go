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
func (h *Handlers) ListAssignments(w http.ResponseWriter, r *http.Request) {
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

	assignments, err := models.ListActiveAssignments(h.DB, athleteID)
	if err != nil {
		log.Printf("api: list assignments for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list assignments")
		return
	}

	result := make([]*AthleteExercise, len(assignments))
	for i, a := range assignments {
		result[i] = AthleteExerciseFromModel(a)
	}
	WriteJSON(w, http.StatusOK, result)
}

// AssignExercise assigns an exercise to an athlete. Coach only.
func (h *Handlers) AssignExercise(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		ExerciseID int64 `json:"exercise_id"`
		TargetReps int   `json:"target_reps"`
	}
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
func (h *Handlers) CheckProgramCompatibility(w http.ResponseWriter, r *http.Request) {
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
