package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// generationTimeout is the wall-clock budget for one LLM call. The HTTP
// server's WriteTimeout (60s in main.go) is no longer the bottleneck because
// the LLM runs in a detached goroutine; this timeout protects against a
// hung provider connection.
const generationTimeout = 5 * time.Minute

// --- DTOs ---

// GenerateFormResponse is the payload for GET /athletes/{id}/generate.
//
// LatestGeneration (when present) lets the SPA resume a draft after a page
// reload — if status is 'running' it polls; if 'succeeded' it jumps straight
// to the preview step.
type GenerateFormResponse struct {
	Configured        bool                `json:"configured"`
	AthleteContext    any                 `json:"athlete_context,omitempty"`
	ReferencePrograms []ProgramTemplate   `json:"reference_programs,omitempty"`
	DefaultDays       int                 `json:"default_days"`
	DefaultWeeks      int                 `json:"default_weeks"`
	LatestGeneration  *GenerationResponse `json:"latest_generation,omitempty"`
}

// GenerateSubmitRequest is the body of POST /athletes/{id}/generate.
type GenerateSubmitRequest struct {
	ProgramName     string   `json:"program_name"`
	NumDays         int      `json:"num_days"`
	NumWeeks        int      `json:"num_weeks"`
	IsLoop          bool     `json:"is_loop"`
	CoachDirections string   `json:"coach_directions"`
	FocusAreas      []string `json:"focus_areas"`
	ReferenceIDs    []int64  `json:"reference_ids"`
}

// GenerationResponse is the polling shape returned by the status endpoint
// and embedded in the form-data resume payload.
//
// On status='succeeded' the Programs and Exercises counts are populated
// by parsing the stored catalog_json so the coach sees the same preview
// metrics the old synchronous flow returned.
type GenerationResponse struct {
	ID         int64  `json:"id"`
	AthleteID  int64  `json:"athlete_id"`
	Status     string `json:"status"`
	Reasoning  string `json:"reasoning,omitempty"`
	Model      string `json:"model,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	Duration   string `json:"duration,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	Programs   int    `json:"programs,omitempty"`
	Exercises  int    `json:"exercises,omitempty"`
	Error      string `json:"error,omitempty"`
	Executed   bool   `json:"executed,omitempty"`

	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// GenerateSubmitResponse is the body of the 202 response from POST
// /athletes/{id}/generate. The SPA polls GET /generations/{id} until the
// status is terminal, then commits via POST /generations/{id}/execute.
type GenerateSubmitResponse struct {
	GenerationID int64  `json:"generation_id"`
	Status       string `json:"status"`
}

// GenerateExecuteResponse is the body of POST /generations/{id}/execute.
type GenerateExecuteResponse struct {
	ProgramsCreated  int `json:"programs_created"`
	ExercisesCreated int `json:"exercises_created"`
	PrescribedSets   int `json:"prescribed_sets"`
	ProgressionRules int `json:"progression_rules"`
}

// --- GET /athletes/{id}/generate ---

// GenerateFormData returns the AI Coach form data for an athlete.
//
//	@Summary      AI Coach form data
//	@Description  Returns the inputs the SPA needs to render the generation form: athlete context, default days/weeks from active program, list of reference programs, and the latest generation (if any) so the SPA can resume a still-running draft after page reload. `configured=false` if no LLM provider is set up.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id   path      int  true  "Athlete ID"
//	@Success      200  {object}  api.GenerateFormResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/generate [get]
func (h *Handlers) GenerateFormData(w http.ResponseWriter, r *http.Request) {
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
		WriteError(w, http.StatusForbidden, "not your athlete")
		return
	}

	// Check if LLM is configured.
	provider := models.GetSetting(h.DB, "llm.provider")
	if provider == "" {
		WriteJSON(w, http.StatusOK, GenerateFormResponse{Configured: false})
		return
	}

	// Build athlete context for the form preview.
	athleteCtx, err := llm.BuildAthleteContext(h.DB, athleteID, time.Now())
	if err != nil {
		log.Printf("api: build athlete context %d: %v", athleteID, err)
	}

	// Get defaults from active program.
	defaultDays, defaultWeeks := 3, 4
	if prog, err := models.GetActiveProgram(h.DB, athleteID); err == nil && prog != nil {
		defaultDays = prog.NumDays
		defaultWeeks = prog.NumWeeks
	}

	// Load reference programs.
	refs, _ := models.ListProgramTemplates(h.DB)
	apiRefs := make([]ProgramTemplate, len(refs))
	for i, r := range refs {
		apiRefs[i] = *ProgramTemplateFromModel(r)
	}

	// Latest generation for resume-after-reload.
	var latest *GenerationResponse
	if g, err := models.LatestGenerationForAthlete(h.DB, athleteID); err == nil {
		latest = generationToResponse(g)
	}

	WriteJSON(w, http.StatusOK, GenerateFormResponse{
		Configured:        true,
		AthleteContext:    athleteCtx,
		ReferencePrograms: apiRefs,
		DefaultDays:       defaultDays,
		DefaultWeeks:      defaultWeeks,
		LatestGeneration:  latest,
	})
}

// --- POST /athletes/{id}/generate ---

// GenerateSubmit enqueues an AI Coach generation and returns immediately.
//
//	@Summary      Enqueue an AI Coach generation
//	@Description  Validates the request, inserts a generation row in 'pending' state, and spawns a background goroutine that calls the configured LLM provider with a context detached from the HTTP request. Returns 202 with the generation_id; the SPA polls GET /generations/{id} until the status is terminal, then commits via POST /generations/{id}/execute.
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Athlete ID"
//	@Param        body  body      api.GenerateSubmitRequest  true  "Generation request"
//	@Success      202   {object}  api.GenerateSubmitResponse
//	@Failure      400   {object}  api.APIError
//	@Failure      403   {object}  api.APIError
//	@Failure      500   {object}  api.APIError  "AI Coach not configured"
//	@Router       /athletes/{id}/generate [post]
func (h *Handlers) GenerateSubmit(w http.ResponseWriter, r *http.Request) {
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
		WriteError(w, http.StatusForbidden, "not your athlete")
		return
	}

	var req GenerateSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ProgramName == "" || req.NumDays < 1 || req.NumWeeks < 1 {
		WriteError(w, http.StatusBadRequest, "program_name, num_days, and num_weeks are required")
		return
	}

	// Resolve the LLM provider up front so misconfiguration fails fast at
	// the HTTP boundary (instead of producing a 'failed' generation row).
	provider, err := h.llmProvider()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "AI Coach not configured: "+err.Error())
		return
	}

	// Snapshot the request so we can audit/re-run it later.
	reqJSON, err := json.Marshal(req)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to encode request: "+err.Error())
		return
	}

	gen, err := models.CreateGeneration(h.DB, athleteID, user.ID, string(reqJSON))
	if err != nil {
		log.Printf("api: create generation for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to enqueue generation")
		return
	}

	// Detach from the request context — the goroutine must outlive the
	// HTTP response and the browser tab. Use Background so a client
	// disconnect or page close does not cancel the LLM call (and waste
	// the tokens we have already committed to spending).
	jobCtx, cancel := context.WithTimeout(context.Background(), generationTimeout)

	h.genWG.Add(1)
	go func() {
		defer cancel()
		defer h.genWG.Done()
		h.runGeneration(jobCtx, gen.ID, provider, llm.GenerationRequest{
			AthleteID:            athleteID,
			ProgramName:          req.ProgramName,
			NumDays:              req.NumDays,
			NumWeeks:             req.NumWeeks,
			IsLoop:               req.IsLoop,
			FocusAreas:           req.FocusAreas,
			CoachDirections:      req.CoachDirections,
			ReferenceTemplateIDs: req.ReferenceIDs,
		})
	}()

	WriteJSON(w, http.StatusAccepted, GenerateSubmitResponse{
		GenerationID: gen.ID,
		Status:       gen.Status,
	})
}

// runGeneration executes one generation in the background. All errors are
// recorded on the generation row — never returned to the caller.
func (h *Handlers) runGeneration(ctx context.Context, genID int64, provider llm.Provider, req llm.GenerationRequest) {
	start := time.Now()

	// Move the row to 'running'. If this fails with ErrNotFound the row
	// was cancelled (or vanished) — log and bail without burning tokens.
	if err := models.MarkGenerationRunning(h.DB, genID); err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			log.Printf("api: mark generation %d running: %v", genID, err)
		}
		return
	}

	result, err := llm.Generate(ctx, h.DB, provider, req)
	durationMS := int(time.Since(start).Milliseconds())

	if err != nil {
		log.Printf("api: LLM generation %d failed: %v", genID, err)
		msg := err.Error()
		var apiErr *llm.APIError
		if errors.As(err, &apiErr) {
			msg = apiErr.UserMessage()
		}
		if failErr := models.FailGeneration(h.DB, genID, msg, durationMS); failErr != nil &&
			!errors.Is(failErr, models.ErrNotFound) {
			log.Printf("api: persist failure for generation %d: %v", genID, failErr)
		}
		return
	}

	if result.CatalogJSON == nil {
		_ = models.FailGeneration(h.DB, genID, "LLM returned empty output", durationMS)
		return
	}

	// Validate the catalog parses before we mark this succeeded — a
	// successful-but-unparseable response is failure from the coach's
	// perspective.
	if _, parseErr := importers.ParseCatalogJSON(bytes.NewReader(result.CatalogJSON)); parseErr != nil {
		log.Printf("api: parse generated catalog for %d: %v", genID, parseErr)
		_ = models.FailGeneration(h.DB, genID, "Failed to parse LLM output: "+parseErr.Error(), durationMS)
		return
	}

	if err := models.CompleteGeneration(h.DB, genID,
		string(result.CatalogJSON), result.Reasoning, result.Model, result.StopReason,
		result.TokensUsed, durationMS); err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			log.Printf("api: complete generation %d: %v", genID, err)
		}
		return
	}

	// Notify the coach.
	gen, err := models.GetGeneration(h.DB, genID)
	if err != nil {
		log.Printf("api: post-complete reload generation %d: %v", genID, err)
		return
	}
	athleteName := h.athleteDisplayName(gen.AthleteID)
	notify.Send(h.DB, notify.Request{
		UserID:    gen.RequestedBy,
		Type:      models.NotifyGenerationComplete,
		Title:     fmt.Sprintf("AI Coach draft ready for %s", athleteName),
		Message:   fmt.Sprintf("Review and approve the generated program for %s.", athleteName),
		Link:      fmt.Sprintf("/athletes/%d/generate", gen.AthleteID),
		AthleteID: sql.NullInt64{Int64: gen.AthleteID, Valid: true},
	})
}

// --- GET /athletes/{id}/generations/{genID} ---

// GenerationStatus returns the current state of an in-progress or completed
// AI Coach generation. The SPA polls this endpoint every ~2s after a submit.
//
//	@Summary      Get AI Coach generation status
//	@Description  Returns the current status, plus reasoning/preview metrics when succeeded or the error message when failed. The SPA polls this endpoint while the generation is pending or running.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id     path      int  true  "Athlete ID"
//	@Param        genID  path      int  true  "Generation ID"
//	@Success      200  {object}  api.GenerationResponse
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/generations/{genID} [get]
func (h *Handlers) GenerationStatus(w http.ResponseWriter, r *http.Request) {
	gen, ok := h.loadOwnedGeneration(w, r)
	if !ok {
		return
	}
	WriteJSON(w, http.StatusOK, generationToResponse(gen))
}

// --- POST /athletes/{id}/generations/{genID}/cancel ---

// GenerationCancel marks a pending or running generation as cancelled.
//
//	@Summary      Cancel an AI Coach generation
//	@Description  Idempotent cancel. If the generation has already reached a terminal state this is a no-op and the current row is returned.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id     path      int  true  "Athlete ID"
//	@Param        genID  path      int  true  "Generation ID"
//	@Success      200  {object}  api.GenerationResponse
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/generations/{genID}/cancel [post]
func (h *Handlers) GenerationCancel(w http.ResponseWriter, r *http.Request) {
	gen, ok := h.loadOwnedGeneration(w, r)
	if !ok {
		return
	}
	if !gen.IsTerminal() {
		if err := models.CancelGeneration(h.DB, gen.ID); err != nil && !errors.Is(err, models.ErrNotFound) {
			log.Printf("api: cancel generation %d: %v", gen.ID, err)
			WriteError(w, http.StatusInternalServerError, "failed to cancel")
			return
		}
		gen, _ = models.GetGeneration(h.DB, gen.ID)
	}
	WriteJSON(w, http.StatusOK, generationToResponse(gen))
}

// --- POST /athletes/{id}/generations/{genID}/execute ---

// GenerationExecute commits a succeeded generation into program_templates.
//
//	@Summary      Commit an AI Coach generation
//	@Description  Parses the stored CatalogJSON, runs ExecuteCatalogImport, and auto-assigns the new program's exercises to the athlete. The generation must be in 'succeeded' state and not already executed.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id     path      int  true  "Athlete ID"
//	@Param        genID  path      int  true  "Generation ID"
//	@Success      200  {object}  api.GenerateExecuteResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/generations/{genID}/execute [post]
func (h *Handlers) GenerationExecute(w http.ResponseWriter, r *http.Request) {
	gen, ok := h.loadOwnedGeneration(w, r)
	if !ok {
		return
	}
	if gen.Status != models.GenerationSucceeded {
		WriteError(w, http.StatusBadRequest, "generation is not ready to execute (status: "+gen.Status+")")
		return
	}
	if gen.ExecutedAt.Valid {
		WriteError(w, http.StatusBadRequest, "generation has already been executed")
		return
	}
	if !gen.CatalogJSON.Valid {
		WriteError(w, http.StatusInternalServerError, "generation has no catalog to execute")
		return
	}

	parsed, err := importers.ParseCatalogJSON(bytes.NewReader([]byte(gen.CatalogJSON.String)))
	if err != nil {
		log.Printf("api: parse stored catalog for generation %d: %v", gen.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to parse stored catalog")
		return
	}

	// Build mappings against the *current* exercise/equipment catalog so
	// any catalog changes made between generation and approval are picked
	// up by the importer's de-dup logic.
	existing, _ := models.ListExercises(h.DB, "")
	entities := exercisesToEntities(existing)
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, entities),
		Parsed:    parsed,
	}
	if parsed.Equipment != nil {
		equip, _ := models.ListEquipment(h.DB)
		eqEntities := make([]importers.ExistingEntity, len(equip))
		for i, e := range equip {
			eqEntities[i] = importers.ExistingEntity{ID: e.ID, Name: e.Name}
		}
		ms.Equipment = importers.BuildEquipmentMappings(parsed.Equipment, eqEntities)
	}
	if parsed.Programs != nil {
		ms.Programs = importers.BuildProgramMappings(parsed.Programs, nil)
	}

	result, err := models.ExecuteCatalogImport(h.DB, ms, &gen.AthleteID)
	if err != nil {
		log.Printf("api: execute generation %d for athlete %d: %v", gen.ID, gen.AthleteID, err)
		WriteError(w, http.StatusInternalServerError, "Failed to save program: "+err.Error())
		return
	}

	// Auto-assign exercises for created programs.
	for _, templateID := range result.CreatedTemplateIDs {
		if _, err := models.AssignProgramExercises(h.DB, gen.AthleteID, templateID); err != nil {
			log.Printf("api: auto-assign exercises for template %d: %v", templateID, err)
		}
	}

	if err := models.MarkGenerationExecuted(h.DB, gen.ID); err != nil && !errors.Is(err, models.ErrNotFound) {
		log.Printf("api: mark generation %d executed: %v", gen.ID, err)
	}

	WriteJSON(w, http.StatusOK, GenerateExecuteResponse{
		ProgramsCreated:  result.ProgramsCreated,
		ExercisesCreated: result.ExercisesCreated,
		PrescribedSets:   result.PrescribedSets,
		ProgressionRules: result.ProgressionRules,
	})
}

// --- internals ---

// loadOwnedGeneration parses {id} and {genID} from the URL, enforces the
// coach role + athlete ownership, and verifies the generation belongs to
// the same athlete. Returns (nil, false) after writing an error response
// when any check fails.
func (h *Handlers) loadOwnedGeneration(w http.ResponseWriter, r *http.Request) (*models.Generation, bool) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return nil, false
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return nil, false
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "not your athlete")
		return nil, false
	}

	genID, err := strconv.ParseInt(r.PathValue("genID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid generation ID")
		return nil, false
	}

	gen, err := models.GetGeneration(h.DB, genID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "generation not found")
		return nil, false
	}
	if err != nil {
		log.Printf("api: get generation %d: %v", genID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load generation")
		return nil, false
	}
	if gen.AthleteID != athleteID {
		// Prevent cross-athlete leakage even though the caller already
		// passed CanAccessAthlete for the URL's {id}.
		WriteError(w, http.StatusNotFound, "generation not found")
		return nil, false
	}
	return gen, true
}

// generationToResponse maps the model to the wire shape. Reasoning, model,
// tokens, and counts are only populated when the generation has reached a
// useful state, so the polling SPA can rely on simple presence checks.
func generationToResponse(g *models.Generation) *GenerationResponse {
	resp := &GenerationResponse{
		ID:        g.ID,
		AthleteID: g.AthleteID,
		Status:    g.Status,
		CreatedAt: g.CreatedAt,
		Executed:  g.ExecutedAt.Valid,
	}
	if g.StartedAt.Valid {
		t := g.StartedAt.Time
		resp.StartedAt = &t
	}
	if g.CompletedAt.Valid {
		t := g.CompletedAt.Time
		resp.CompletedAt = &t
	}
	if g.Reasoning.Valid {
		resp.Reasoning = g.Reasoning.String
	}
	if g.Model.Valid {
		resp.Model = g.Model.String
	}
	resp.TokensUsed = g.TokensUsed
	if g.DurationMS > 0 {
		resp.Duration = (time.Duration(g.DurationMS) * time.Millisecond).Round(time.Millisecond).String()
	}
	if g.StopReason.Valid {
		resp.Truncated = g.StopReason.String == "max_tokens" || g.StopReason.String == "length"
	}
	if g.Error.Valid {
		resp.Error = g.Error.String
	}

	// Best-effort preview counts — only matter on success.
	if g.Status == models.GenerationSucceeded && g.CatalogJSON.Valid {
		if parsed, err := importers.ParseCatalogJSON(bytes.NewReader([]byte(g.CatalogJSON.String))); err == nil {
			resp.Programs = len(parsed.Programs)
			resp.Exercises = len(parsed.Exercises)
		}
	}
	return resp
}
