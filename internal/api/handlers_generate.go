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
	"sort"
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
// metrics the old synchronous flow returned. Preview is also populated
// with the per-day prescribed-set projection so the coach can review the
// actual program before approving it (HOF-001 #13).
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

	// Preview is the set-level projection of catalog_json — present only
	// on succeeded generations. Lets the SPA render the actual prescribed
	// program (per week/day) before the coach approves.
	Preview *GenerationPreview `json:"preview,omitempty"`

	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// GenerationPreview is the human-reviewable projection of a generated
// program built from the stored catalog_json — what the coach sees on
// the preview step before clicking "approve as draft".
type GenerationPreview struct {
	Programs         []ProgramPreview         `json:"programs"`
	ProgressionRules []ProgressionRulePreview `json:"progression_rules,omitempty"`
}

// ProgramPreview is one program template within a generation preview.
type ProgramPreview struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	NumWeeks    int           `json:"num_weeks"`
	NumDays     int           `json:"num_days"`
	IsLoop      bool          `json:"is_loop"`
	Weeks       []WeekPreview `json:"weeks"`
}

// WeekPreview groups prescribed sets by training week.
type WeekPreview struct {
	Week int          `json:"week"`
	Days []DayPreview `json:"days"`
}

// DayPreview groups prescribed sets by training day within a week.
type DayPreview struct {
	Day  int                    `json:"day"`
	Sets []PrescribedSetPreview `json:"sets"`
}

// PrescribedSetPreview is one prescribed set in the preview projection.
type PrescribedSetPreview struct {
	Exercise       string   `json:"exercise"`
	SetNumber      int      `json:"set_number"`
	Reps           *int     `json:"reps,omitempty"`
	RepType        string   `json:"rep_type,omitempty"`
	Percentage     *float64 `json:"percentage,omitempty"`
	AbsoluteWeight *float64 `json:"absolute_weight,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

// ProgressionRulePreview is one TM-progression rule attached to a program.
type ProgressionRulePreview struct {
	Program   string  `json:"program"`
	Exercise  string  `json:"exercise"`
	Increment float64 `json:"increment"`
}

// GenerateSubmitResponse is the body of the 202 response from POST
// /athletes/{id}/generate. The SPA polls GET /generations/{id} until the
// status is terminal, then commits via POST /generations/{id}/execute.
type GenerateSubmitResponse struct {
	GenerationID int64  `json:"generation_id"`
	Status       string `json:"status"`
}

// GenerateExecuteResponse is the body of POST /generations/{id}/execute.
//
// CreatedTemplateIDs lets the SPA navigate the coach straight to the new
// (athlete-scoped, unassigned) template's edit page after approving the
// draft. The coach is expected to edit then explicitly assign via
// POST /athletes/{id}/programs (HOF-001 #13, ADR 007).
type GenerateExecuteResponse struct {
	ProgramsCreated    int     `json:"programs_created"`
	ExercisesCreated   int     `json:"exercises_created"`
	PrescribedSets     int     `json:"prescribed_sets"`
	ProgressionRules   int     `json:"progression_rules"`
	CreatedTemplateIDs []int64 `json:"created_template_ids"`
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
//	@Failure      409   {object}  api.APIError  "A draft is already in flight for this athlete"
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

	// Duplicate-submit guard: reject if this athlete already has a draft
	// in flight. The unique-per-athlete pending/running invariant keeps
	// the SPA's resume-on-reload logic deterministic and prevents two
	// goroutines burning tokens for the same athlete in parallel.
	if existing, lookupErr := models.PendingOrRunningGenerationForAthlete(h.DB, athleteID); lookupErr != nil {
		log.Printf("api: check in-flight generation for athlete %d: %v", athleteID, lookupErr)
		WriteError(w, http.StatusInternalServerError, "failed to check in-flight generations")
		return
	} else if existing != nil {
		WriteError(w, http.StatusConflict, fmt.Sprintf("a draft is already in flight for this athlete (generation %d, status %s)", existing.ID, existing.Status))
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
// recorded on the generation row — never returned to the caller. Every
// terminal-failure path also fires a NotifyGenerationFailed so the SPA's
// "safe to close this tab — a notification will arrive" promise holds.
func (h *Handlers) runGeneration(ctx context.Context, genID int64, provider llm.Provider, req llm.GenerationRequest) {
	start := time.Now()

	// Move the row to 'running'. If this fails with ErrNotFound the row
	// was cancelled (or vanished) — log and bail without burning tokens
	// or notifying (the coach saw the cancel they themselves clicked).
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
		h.failAndNotify(genID, msg, durationMS)
		return
	}

	if result.CatalogJSON == nil {
		msg := "LLM returned empty output"
		if result.StopReason == "max_tokens" || result.StopReason == "length" {
			msg = "Output was truncated — increase max_tokens in AI Coach settings and try again. (No CatalogJSON found in the response.)"
		}
		h.failAndNotify(genID, msg, durationMS)
		return
	}

	// Validate the catalog parses before we mark this succeeded — a
	// successful-but-unparseable response is failure from the coach's
	// perspective. If we know the response was truncated, fold an
	// actionable hint into the error message — FailGeneration doesn't
	// persist stop_reason, so the SPA only sees the error string.
	if _, parseErr := importers.ParseCatalogJSON(bytes.NewReader(result.CatalogJSON)); parseErr != nil {
		log.Printf("api: parse generated catalog for %d: %v", genID, parseErr)
		msg := "Failed to parse LLM output: " + parseErr.Error()
		if result.StopReason == "max_tokens" || result.StopReason == "length" {
			msg = "Output was truncated — increase max_tokens in AI Coach settings and try again. (Parse error: " + parseErr.Error() + ")"
		}
		h.failAndNotify(genID, msg, durationMS)
		return
	}

	if err := models.CompleteGeneration(h.DB, genID,
		string(result.CatalogJSON), result.Reasoning, result.Model, result.StopReason,
		result.TokensUsed, durationMS,
		string(result.ContextJSON), result.Prompt); err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			log.Printf("api: complete generation %d: %v", genID, err)
		}
		return
	}

	// Notify the coach the draft is ready for review.
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

// failAndNotify persists a generation failure and notifies the requester.
// Safe to call on a row that vanished between MarkGenerationRunning and
// here (cancelled) — FailGeneration is no-op on non-running rows and we
// suppress the notification in that case.
func (h *Handlers) failAndNotify(genID int64, msg string, durationMS int) {
	if err := models.FailGeneration(h.DB, genID, msg, durationMS); err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			log.Printf("api: persist failure for generation %d: %v", genID, err)
		}
		// Row was cancelled or missing — don't notify (the coach who
		// cancelled doesn't need to hear about it as a failure).
		return
	}
	gen, err := models.GetGeneration(h.DB, genID)
	if err != nil {
		log.Printf("api: post-failure reload generation %d: %v", genID, err)
		return
	}
	athleteName := h.athleteDisplayName(gen.AthleteID)
	notify.Send(h.DB, notify.Request{
		UserID:    gen.RequestedBy,
		Type:      models.NotifyGenerationFailed,
		Title:     fmt.Sprintf("AI Coach draft failed for %s", athleteName),
		Message:   msg,
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

	result, err := models.ExecuteCatalogImport(h.DB, ms, &gen.AthleteID, false)
	if err != nil {
		log.Printf("api: execute generation %d for athlete %d: %v", gen.ID, gen.AthleteID, err)
		WriteError(w, http.StatusInternalServerError, "Failed to save program: "+err.Error())
		return
	}

	// IMPORTANT: do NOT auto-assign exercises or activate the program.
	// Approving the draft creates an athlete-scoped but UNASSIGNED template;
	// the coach edits via PUT /programs/{id} + sets/rules and then explicitly
	// assigns via POST /athletes/{id}/programs (HOF-001 #13, ADR 007).

	if err := models.MarkGenerationExecuted(h.DB, gen.ID); err != nil && !errors.Is(err, models.ErrNotFound) {
		log.Printf("api: mark generation %d executed: %v", gen.ID, err)
	}

	WriteJSON(w, http.StatusOK, GenerateExecuteResponse{
		ProgramsCreated:    result.ProgramsCreated,
		ExercisesCreated:   result.ExercisesCreated,
		PrescribedSets:     result.PrescribedSets,
		ProgressionRules:   result.ProgressionRules,
		CreatedTemplateIDs: result.CreatedTemplateIDs,
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

	// Best-effort preview projection — only on success. Counts mirror the
	// old synchronous flow's metrics; preview is the new per-day set-level
	// projection the coach reviews before approving (HOF-001 #13).
	if g.Status == models.GenerationSucceeded && g.CatalogJSON.Valid {
		if parsed, err := importers.ParseCatalogJSON(bytes.NewReader([]byte(g.CatalogJSON.String))); err == nil {
			resp.Programs = len(parsed.Programs)
			resp.Exercises = len(parsed.Exercises)
			resp.Preview = buildGenerationPreview(parsed)
		}
	}
	return resp
}

// buildGenerationPreview projects a parsed catalog into the per-week,
// per-day shape the SPA renders. Empty programs are skipped; sets within
// a day are sorted by (sort_order, set_number) so the coach sees them in
// the intended order regardless of how the LLM emitted them.
func buildGenerationPreview(parsed *importers.ParsedFile) *GenerationPreview {
	if parsed == nil || len(parsed.Programs) == 0 {
		return nil
	}
	preview := &GenerationPreview{}
	for _, prog := range parsed.Programs {
		pt := prog.Template
		pp := ProgramPreview{
			Name:     pt.Name,
			NumWeeks: pt.NumWeeks,
			NumDays:  pt.NumDays,
			IsLoop:   pt.IsLoop,
		}
		if pt.Description != nil {
			pp.Description = *pt.Description
		}

		// Group: week → day → ordered sets. We use slices instead of maps
		// so the JSON keeps a stable order.
		type dayKey struct{ week, day int }
		byDay := make(map[dayKey][]importers.ParsedPrescribedSet)
		for _, ps := range pt.PrescribedSets {
			k := dayKey{ps.Week, ps.Day}
			byDay[k] = append(byDay[k], ps)
		}
		for w := 1; w <= pt.NumWeeks; w++ {
			wk := WeekPreview{Week: w}
			anyDay := false
			for d := 1; d <= pt.NumDays; d++ {
				sets := byDay[dayKey{w, d}]
				if len(sets) == 0 {
					continue
				}
				anyDay = true
				sort.SliceStable(sets, func(i, j int) bool {
					if sets[i].SortOrder != sets[j].SortOrder {
						return sets[i].SortOrder < sets[j].SortOrder
					}
					return sets[i].SetNumber < sets[j].SetNumber
				})
				dp := DayPreview{Day: d}
				for _, ps := range sets {
					sp := PrescribedSetPreview{
						Exercise:       ps.Exercise,
						SetNumber:      ps.SetNumber,
						Reps:           ps.Reps,
						RepType:        ps.RepType,
						Percentage:     ps.Percentage,
						AbsoluteWeight: ps.AbsoluteWeight,
					}
					if ps.Notes != nil {
						sp.Notes = *ps.Notes
					}
					dp.Sets = append(dp.Sets, sp)
				}
				wk.Days = append(wk.Days, dp)
			}
			if anyDay {
				pp.Weeks = append(pp.Weeks, wk)
			}
		}
		for _, rule := range pt.ProgressionRules {
			preview.ProgressionRules = append(preview.ProgressionRules, ProgressionRulePreview{
				Program:   pt.Name,
				Exercise:  rule.Exercise,
				Increment: rule.Increment,
			})
		}
		preview.Programs = append(preview.Programs, pp)
	}
	return preview
}
