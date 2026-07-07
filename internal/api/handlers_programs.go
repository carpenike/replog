package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Program Template CRUD ---

// CreateProgramTemplate creates a new program template. Coach only.
//
//	@Summary      Create program template
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.ProgramTemplateRequest  true  "Template"
//	@Success      201  {object}  api.ProgramTemplate
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs [post]
func (h *Handlers) CreateProgramTemplate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	var req ProgramTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.NumWeeks < 1 || req.NumDays < 1 {
		WriteError(w, http.StatusBadRequest, "name, num_weeks, and num_days are required")
		return
	}

	program, err := models.CreateProgramTemplate(r.Context(), h.DB, req.AthleteID, req.Name, req.Description, req.NumWeeks, req.NumDays, req.IsLoop, req.Audience)
	if err != nil {
		log.Printf("api: create program template: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create program")
		return
	}

	WriteJSON(w, http.StatusCreated, ProgramTemplateFromModel(program))
}

// UpdateProgramTemplate updates a program template. Coach only.
//
//	@Summary      Update program template
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                                true  "Program ID"
//	@Param        body  body      api.ProgramTemplateUpdateRequest   true  "Template"
//	@Success      200  {object}  api.ProgramTemplate
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /programs/{id} [put]
func (h *Handlers) UpdateProgramTemplate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	var req ProgramTemplateUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	program, err := models.UpdateProgramTemplate(r.Context(), h.DB, id, req.Name, req.Description, req.NumWeeks, req.NumDays, req.IsLoop)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "program not found")
		return
	}
	if err != nil {
		log.Printf("api: update program template %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to update program")
		return
	}

	WriteJSON(w, http.StatusOK, ProgramTemplateFromModel(program))
}

// DeleteProgramTemplate deletes a program template. Coach only.
//
//	@Summary      Delete program template
//	@Tags         Programs
//	@Produce      json
//	@Param        id   path      int  true  "Program ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs/{id} [delete]
func (h *Handlers) DeleteProgramTemplate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	if err := models.DeleteProgramTemplate(r.Context(), h.DB, id); err != nil {
		log.Printf("api: delete program template %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete program")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Prescribed Sets CRUD ---

// AddPrescribedSet adds a prescribed set to a program template.
//
//	@Summary      Add prescribed set
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                       true  "Program ID"
//	@Param        body  body      api.PrescribedSetRequest  true  "Set"
//	@Success      201  {object}  api.PrescribedSet
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs/{id}/sets [post]
func (h *Handlers) AddPrescribedSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	var req PrescribedSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id is required")
		return
	}
	if req.RepType == "" {
		req.RepType = "reps"
	}

	set, err := models.CreatePrescribedSet(r.Context(), h.DB, templateID, req.ExerciseID, req.Week, req.Day, req.SetNumber, req.Reps, req.Percentage, req.AbsoluteWeight, req.SortOrder, req.RepType, req.Notes)
	if err != nil {
		log.Printf("api: add prescribed set to template %d: %v", templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to add set")
		return
	}

	WriteJSON(w, http.StatusCreated, PrescribedSetFromModel(set))
}

// UpdatePrescribedSet updates a prescribed set.
//
//	@Summary      Update prescribed set
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        id     path      int                              true  "Program ID"
//	@Param        setID  path      int                              true  "Set ID"
//	@Param        body   body      api.PrescribedSetUpdateRequest   true  "Set"
//	@Success      200  {object}  api.PrescribedSet
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /programs/{id}/sets/{setID} [put]
func (h *Handlers) UpdatePrescribedSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	var req PrescribedSetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RepType == "" {
		req.RepType = "reps"
	}

	set, err := models.UpdatePrescribedSet(r.Context(), h.DB, setID, req.ExerciseID, req.SetNumber, req.Reps, req.Percentage, req.AbsoluteWeight, req.SortOrder, req.RepType, req.Notes)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "set not found")
		return
	}
	if err != nil {
		log.Printf("api: update prescribed set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update set")
		return
	}

	WriteJSON(w, http.StatusOK, PrescribedSetFromModel(set))
}

// DeletePrescribedSet deletes a prescribed set.
//
//	@Summary      Delete prescribed set
//	@Tags         Programs
//	@Produce      json
//	@Param        id     path      int  true  "Program ID"
//	@Param        setID  path      int  true  "Set ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs/{id}/sets/{setID} [delete]
func (h *Handlers) DeletePrescribedSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	setID, err := strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	if err := models.DeletePrescribedSet(r.Context(), h.DB, setID); err != nil {
		log.Printf("api: delete prescribed set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete set")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Progression Rules ---

// ProgressionRuleResponse is the JSON representation.
type ProgressionRuleResponse struct {
	ID           int64   `json:"id"`
	TemplateID   int64   `json:"template_id"`
	ExerciseID   int64   `json:"exercise_id"`
	Increment    float64 `json:"increment"`
	ExerciseName string  `json:"exercise_name,omitempty"`
}

// ListProgressionRules returns progression rules for a template.
//
//	@Summary      List progression rules
//	@Tags         Programs
//	@Produce      json
//	@Param        id   path      int  true  "Program ID"
//	@Success      200  {array}   api.ProgressionRuleResponse
//	@Failure      400  {object}  api.APIError
//	@Router       /programs/{id}/rules [get]
func (h *Handlers) ListProgressionRules(w http.ResponseWriter, r *http.Request) {
	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	rules, err := models.ListProgressionRules(r.Context(), h.DB, templateID)
	if err != nil {
		log.Printf("api: list progression rules for template %d: %v", templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	result := make([]*ProgressionRuleResponse, len(rules))
	for i, r := range rules {
		result[i] = &ProgressionRuleResponse{
			ID:           r.ID,
			TemplateID:   r.TemplateID,
			ExerciseID:   r.ExerciseID,
			Increment:    r.Increment,
			ExerciseName: r.ExerciseName,
		}
	}
	WriteJSON(w, http.StatusOK, result)
}

// SetProgressionRule sets a progression rule for an exercise in a template.
//
//	@Summary      Set progression rule
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                          true  "Program ID"
//	@Param        body  body      api.ProgressionRuleRequest   true  "Rule"
//	@Success      201  {object}  api.ProgressionRuleResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs/{id}/rules [post]
func (h *Handlers) SetProgressionRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	var req ProgressionRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExerciseID == 0 || req.Increment <= 0 {
		WriteError(w, http.StatusBadRequest, "exercise_id and increment are required")
		return
	}

	rule, err := models.SetProgressionRule(r.Context(), h.DB, templateID, req.ExerciseID, req.Increment)
	if err != nil {
		log.Printf("api: set progression rule for template %d: %v", templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to set rule")
		return
	}

	WriteJSON(w, http.StatusCreated, ProgressionRuleResponse{
		ID:         rule.ID,
		TemplateID: rule.TemplateID,
		ExerciseID: rule.ExerciseID,
		Increment:  rule.Increment,
	})
}

// DeleteProgressionRule deletes a progression rule.
//
//	@Summary      Delete progression rule
//	@Tags         Programs
//	@Produce      json
//	@Param        id      path      int  true  "Program ID"
//	@Param        ruleID  path      int  true  "Rule ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs/{id}/rules/{ruleID} [delete]
func (h *Handlers) DeleteProgressionRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	ruleID, err := strconv.ParseInt(r.PathValue("ruleID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}

	if err := models.DeleteProgressionRule(r.Context(), h.DB, ruleID); err != nil {
		log.Printf("api: delete progression rule %d: %v", ruleID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Athlete Promotion ---

// PromoteAthlete promotes an athlete to the next tier. Coach only.
//
//	@Summary      Promote athlete to next tier
//	@Tags         Athletes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.Athlete
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/promote [post]
func (h *Handlers) PromoteAthlete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	athlete, err := models.GetAthleteByID(r.Context(), h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: get athlete %d for promotion: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get athlete")
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	promoted, err := models.PromoteAthlete(r.Context(), h.DB, id)
	if err != nil {
		log.Printf("api: promote athlete %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to promote athlete")
		return
	}

	WriteJSON(w, http.StatusOK, AthleteFromModel(promoted))
}

// ListProgramTemplates returns program templates. With no query params it
// returns every template. When an athlete_id query param is supplied it
// returns the templates assignable to that athlete: global templates plus
// templates scoped to that athlete (athlete-specific sorted first).
//
//	@Summary      List program templates
//	@Tags         Programs
//	@Produce      json
//	@Param        athlete_id  query  int  false  "Limit to templates assignable to this athlete (global + athlete-scoped)"
//	@Success      200  {array}   api.ProgramTemplate
//	@Router       /programs [get]
func (h *Handlers) ListProgramTemplates(w http.ResponseWriter, r *http.Request) {
	var (
		programs []*models.ProgramTemplate
		err      error
	)
	if raw := r.URL.Query().Get("athlete_id"); raw != "" {
		athleteID, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			WriteError(w, http.StatusBadRequest, "invalid athlete_id")
			return
		}
		// Athlete-scoped templates can embed that athlete's context (e.g.
		// AI-generated programs), so gate on access before returning them.
		user := middleware.UserFromContext(r.Context())
		if !middleware.CanAccessAthlete(r.Context(), h.DB, user, athleteID) {
			WriteError(w, http.StatusForbidden, "access denied")
			return
		}
		programs, err = models.ListProgramTemplatesForAthlete(r.Context(), h.DB, athleteID)
	} else {
		programs, err = models.ListProgramTemplates(r.Context(), h.DB)
	}
	if err != nil {
		log.Printf("api: list program templates: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list programs")
		return
	}

	result := make([]*ProgramTemplate, len(programs))
	for i, p := range programs {
		result[i] = ProgramTemplateFromModel(p)
	}
	WriteJSON(w, http.StatusOK, result)
}

// GetProgramTemplate returns a single program template.
//
//	@Summary      Get program template
//	@Description  Returns the template metadata plus its prescribed sets.
//	@Tags         Programs
//	@Produce      json
//	@Param        id   path      int  true  "Program ID"
//	@Success      200  {object}  map[string]interface{}
//	@Failure      400  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /programs/{id} [get]
func (h *Handlers) GetProgramTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	program, err := models.GetProgramTemplateByID(r.Context(), h.DB, id)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "program not found")
		return
	}
	if err != nil {
		log.Printf("api: get program template %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get program")
		return
	}
	// Athlete-scoped templates embed that athlete's context; a global template
	// (AthleteID == nil) is readable by any authenticated user, but a scoped one
	// requires access to its athlete. Treat a denied scoped template as 404 so
	// its existence is not disclosed.
	if program.AthleteID != nil {
		user := middleware.UserFromContext(r.Context())
		if !middleware.CanAccessAthlete(r.Context(), h.DB, user, *program.AthleteID) {
			WriteError(w, http.StatusNotFound, "program not found")
			return
		}
	}

	sets, err := models.ListPrescribedSets(r.Context(), h.DB, id)
	if err != nil {
		log.Printf("api: list prescribed sets for program %d: %v", id, err)
		WriteError(w, http.StatusInternalServerError, "failed to get program sets")
		return
	}

	apiSets := make([]*PrescribedSet, len(sets))
	for i, s := range sets {
		apiSets[i] = PrescribedSetFromModel(s)
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"program": ProgramTemplateFromModel(program),
		"sets":    apiSets,
	})
}

// ListAthletePrograms returns programs assigned to an athlete.//
//
//	@Summary      List athlete's program assignments
//	@Tags         Programs
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {array}   api.AthleteProgram
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/programs [get]
func (h *Handlers) ListAthletePrograms(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	programs, err := models.ListAthletePrograms(r.Context(), h.DB, athleteID)
	if err != nil {
		log.Printf("api: list athlete programs for %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list programs")
		return
	}

	result := make([]*AthleteProgram, len(programs))
	for i, p := range programs {
		result[i] = AthleteProgramFromModel(p)
	}
	WriteJSON(w, http.StatusOK, result)
}

// CopyWeek copies prescribed sets from one week to another.
// CopyWeek copies all prescribed sets from one week to another within a template.
//
//	@Summary      Copy week of prescribed sets
//	@Tags         Programs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                  true  "Program ID"
//	@Param        body  body      api.CopyWeekRequest  true  "Source/target weeks"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /programs/{id}/copy-week [post]
func (h *Handlers) CopyWeek(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	templateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid program ID")
		return
	}

	var req CopyWeekRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourceWeek < 1 || req.TargetWeek < 1 {
		WriteError(w, http.StatusBadRequest, "source_week and target_week must be positive")
		return
	}

	inserted, err := models.CopyWeek(r.Context(), h.DB, templateID, req.SourceWeek, req.TargetWeek)
	if err != nil {
		log.Printf("api: copy week for template %d: %v", templateID, err)
		WriteError(w, http.StatusInternalServerError, "failed to copy week")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int{"sets_copied": int(inserted)})
}
