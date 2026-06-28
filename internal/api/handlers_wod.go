package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// wodMethodologyKey is the seeded adult Sarge-circuit methodology a WOD
// always resolves to (HOF-015). It carries the circuit style exemplars
// (Circuit A/B/C) and the equipment/exercise allow-lists that scope the
// catalog handed to the LLM.
const wodMethodologyKey = "sarge-circuit"

// wodCoachDirections is the one-session framing preset prepended to any
// coach-supplied directions. The adult Sarge block in buildSystemPrompt
// already encodes the circuit shape; this constrains the engine to emit a
// single day to vary from the A/B/C exemplars rather than a multi-day block.
const wodCoachDirections = "Generate exactly ONE training day as a single Sarge Athletics circuit-style WOD. " +
	"Treat the Circuit A/B/C reference programs as stylistic exemplars to vary from — do not copy them verbatim. " +
	"Produce one cohesive session: an EMOM opener followed by paired-exercise circuit rows, absolute-weight loading (no percentages). " +
	"Keep it to a single day's work the athlete can complete in one session."

// --- DTOs ---

// WODSubmitRequest is the body of POST /athletes/{id}/wod. All fields are
// optional — a bare WOD request uses the athlete's configured equipment and
// the default Sarge framing. CoachDirections, when present, is appended to
// the built-in WOD framing; FocusAreas bias the session emphasis.
type WODSubmitRequest struct {
	CoachDirections string   `json:"coach_directions"`
	FocusAreas      []string `json:"focus_areas"`
}

// WODLogRequest is the body of POST /athletes/{id}/wod/{genID}/log.
//
// Date defaults to today (the athlete's timezone). Replace controls the
// same-day collision behavior: when a resistance workout already exists for
// the date, replace=false returns a 409 collision (the SPA prompts
// replace-or-cancel); replace=true supersedes the existing resistance
// workout (HOF-015 same-day collision decision).
type WODLogRequest struct {
	Date    string `json:"date"`
	Replace bool   `json:"replace"`
}

// WODLogResponse summarizes what the log path committed.
type WODLogResponse struct {
	WorkoutID   int64 `json:"workout_id"`
	SetsCreated int   `json:"sets_created"`
	Replaced    bool  `json:"replaced"`
}

// WODCollisionResponse is returned with 409 when a same-day resistance
// workout exists and the caller did not request a replace. The SPA reads
// `collision: true` to show a replace-or-cancel prompt instead of a raw
// error toast.
type WODCollisionResponse struct {
	Error     string `json:"error"`
	Collision bool   `json:"collision"`
}

// --- POST /athletes/{id}/wod ---

// WODSubmit enqueues an ad-hoc Sarge-circuit WOD generation for an adult
// athlete (HOF-015).
//
//	@Summary      Generate an ad-hoc WOD
//	@Description  Enqueues a single-session (NumDays=1, NumWeeks=1) Sarge-circuit "workout of the day" generation scoped to the athlete's configured equipment. Reuses the async generation pipeline; poll status via GET /athletes/{id}/generations/{genID}. Adult-only (resolves the sarge-circuit methodology). Returns 202 with the generation_id.
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                  true  "Athlete ID"
//	@Param        body  body      api.WODSubmitRequest  false  "Optional coach directions / focus areas"
//	@Success      202  {object}  api.GenerateSubmitResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      409  {object}  api.APIError
//	@Failure      500  {object}  api.APIError
//	@Router       /athletes/{id}/wod [post]
func (h *Handlers) WODSubmit(w http.ResponseWriter, r *http.Request) {
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

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "athlete not found")
		return
	}
	if err != nil {
		log.Printf("api: get athlete %d for WOD: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load athlete")
		return
	}
	// Adult-only for v1 (the sarge-circuit methodology is adult-audience and
	// youth-safety floors are a separate conversation). Youth athletes carry
	// a tier; adults have none.
	if athlete.Tier.Valid && strings.TrimSpace(athlete.Tier.String) != "" {
		WriteError(w, http.StatusBadRequest, "ad-hoc WOD generation is adult-only")
		return
	}

	// Body is optional.
	var req WODSubmitRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	provider, err := h.llmProvider()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "AI Coach not configured: "+err.Error())
		return
	}

	// Resolve the Sarge-circuit methodology by key — adults have no
	// tier-default, so the WOD path binds it explicitly.
	methodology, err := models.GetMethodologyByKey(h.DB, wodMethodologyKey)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusInternalServerError, "sarge-circuit methodology is not seeded")
		return
	}
	if err != nil {
		log.Printf("api: resolve sarge-circuit methodology: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to resolve WOD methodology")
		return
	}
	methodologyID := methodology.ID

	// Duplicate-submit guard, scoped to WOD kind so a WOD in flight does not
	// contend with a normal program draft for the same athlete.
	if existing, lookupErr := models.PendingOrRunningGenerationForAthlete(h.DB, athleteID, models.GenerationKindWOD); lookupErr != nil {
		log.Printf("api: check in-flight WOD for athlete %d: %v", athleteID, lookupErr)
		WriteError(w, http.StatusInternalServerError, "failed to check in-flight WODs")
		return
	} else if existing != nil {
		WriteError(w, http.StatusConflict, "a WOD is already in flight for this athlete")
		return
	}

	directions := wodCoachDirections
	if d := strings.TrimSpace(req.CoachDirections); d != "" {
		directions = wodCoachDirections + "\n\nAdditional coach directions: " + d
	}

	genReq := llm.GenerationRequest{
		AthleteID:       athleteID,
		ProgramName:     "Ad-hoc WOD",
		NumDays:         1,
		NumWeeks:        1,
		IsLoop:          false,
		FocusAreas:      req.FocusAreas,
		CoachDirections: directions,
		MethodologyID:   &methodologyID,
	}

	reqJSON, err := json.Marshal(genReq)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to encode request: "+err.Error())
		return
	}

	gen, err := models.CreateGenerationWithKind(h.DB, athleteID, user.ID, string(reqJSON), models.GenerationKindWOD)
	if err != nil {
		log.Printf("api: create WOD generation for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to enqueue WOD")
		return
	}

	// Detach from the request context so the LLM call outlives the browser
	// tab (same rationale as GenerateSubmit).
	jobCtx, cancel := context.WithTimeout(context.Background(), generationTimeout)
	h.genWG.Add(1)
	go func() {
		defer cancel()
		defer h.genWG.Done()
		h.runGeneration(jobCtx, gen.ID, provider, genReq)
	}()

	WriteJSON(w, http.StatusAccepted, GenerateSubmitResponse{
		GenerationID: gen.ID,
		Status:       gen.Status,
	})
}

// --- POST /athletes/{id}/wod/{genID}/log ---

// WODLog commits a succeeded WOD generation as an ad-hoc resistance workout
// (HOF-015). "Discard" is the absence of this call — an unlogged WOD leaves
// the generation row unexecuted and writes nothing to the workout log.
//
//	@Summary      Log an ad-hoc WOD
//	@Description  Parses the generation's CatalogJSON and seeds a discipline='resistance', assignment_id NULL ad-hoc workout with the generated sets (exercise names resolved to IDs), for the athlete to confirm/edit. If a resistance workout already exists for the date, returns 409 with collision=true unless replace=true is sent (replace supersedes the existing workout). The generation must be kind='wod', in 'succeeded' state, and not already logged.
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id     path      int               true  "Athlete ID"
//	@Param        genID  path      int               true  "Generation ID"
//	@Param        body   body      api.WODLogRequest  false  "Date + replace flag"
//	@Success      201  {object}  api.WODLogResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      409  {object}  api.WODCollisionResponse
//	@Failure      500  {object}  api.APIError
//	@Router       /athletes/{id}/wod/{genID}/log [post]
func (h *Handlers) WODLog(w http.ResponseWriter, r *http.Request) {
	gen, ok := h.loadOwnedGeneration(w, r)
	if !ok {
		return
	}
	if gen.Kind != models.GenerationKindWOD {
		WriteError(w, http.StatusBadRequest, "generation is not a WOD")
		return
	}
	if gen.Status != models.GenerationSucceeded {
		WriteError(w, http.StatusBadRequest, "WOD is not ready to log (status: "+gen.Status+")")
		return
	}
	if gen.ExecutedAt.Valid {
		WriteError(w, http.StatusBadRequest, "WOD has already been logged")
		return
	}
	if !gen.CatalogJSON.Valid {
		WriteError(w, http.StatusInternalServerError, "WOD has no catalog to log")
		return
	}

	var req WODLogRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	date := strings.TrimSpace(req.Date)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	parsed, err := importers.ParseCatalogJSON(bytes.NewReader([]byte(gen.CatalogJSON.String)))
	if err != nil {
		log.Printf("api: parse stored catalog for WOD %d: %v", gen.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to parse stored WOD")
		return
	}

	result, err := models.LogWODFromCatalog(h.DB, gen.AthleteID, date, parsed, req.Replace)
	if errors.Is(err, models.ErrWODCollision) {
		WriteJSON(w, http.StatusConflict, WODCollisionResponse{
			Error:     "a resistance workout already exists for this date — replace it or cancel",
			Collision: true,
		})
		return
	}
	if err != nil {
		log.Printf("api: log WOD %d for athlete %d: %v", gen.ID, gen.AthleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to log WOD: "+err.Error())
		return
	}

	if err := models.MarkGenerationExecuted(h.DB, gen.ID); err != nil && !errors.Is(err, models.ErrNotFound) {
		log.Printf("api: mark WOD %d logged: %v", gen.ID, err)
	}

	WriteJSON(w, http.StatusCreated, WODLogResponse{
		WorkoutID:   result.WorkoutID,
		SetsCreated: result.SetsCreated,
		Replaced:    result.Replaced,
	})
}
