package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// GenerateFormResponse returns data needed to render the generation form.
type GenerateFormResponse struct {
	Configured        bool     `json:"configured"`
	AthleteContext    any      `json:"athlete_context,omitempty"`
	ReferencePrograms []ProgramTemplate `json:"reference_programs,omitempty"`
	DefaultDays       int      `json:"default_days"`
	DefaultWeeks      int      `json:"default_weeks"`
}

// GenerateFormData returns the AI Coach form data for an athlete.
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

	WriteJSON(w, http.StatusOK, GenerateFormResponse{
		Configured:        true,
		AthleteContext:    athleteCtx,
		ReferencePrograms: apiRefs,
		DefaultDays:       defaultDays,
		DefaultWeeks:      defaultWeeks,
	})
}

// GenerateSubmitRequest is the request to submit an LLM generation.
type GenerateSubmitRequest struct {
	ProgramName     string  `json:"program_name"`
	NumDays         int     `json:"num_days"`
	NumWeeks        int     `json:"num_weeks"`
	IsLoop          bool    `json:"is_loop"`
	CoachDirections string  `json:"coach_directions"`
	FocusAreas      []string `json:"focus_areas"`
	ReferenceIDs    []int64 `json:"reference_ids"`
}

// GenerateSubmitResponse is returned after LLM generation completes.
type GenerateSubmitResponse struct {
	Reasoning  string `json:"reasoning"`
	Model      string `json:"model"`
	TokensUsed int    `json:"tokens_used"`
	Duration   string `json:"duration"`
	Truncated  bool   `json:"truncated"`
	Programs   int    `json:"programs"`
	Exercises  int    `json:"exercises"`
}

// GenerateSubmit submits an LLM generation request and returns results.
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

	var req GenerateSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ProgramName == "" || req.NumDays < 1 || req.NumWeeks < 1 {
		WriteError(w, http.StatusBadRequest, "program_name, num_days, and num_weeks are required")
		return
	}

	// Create LLM provider.
	provider, err := llm.NewProviderFromSettings(h.DB)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "AI Coach not configured: "+err.Error())
		return
	}

	// Build generation request.
	genReq := llm.GenerationRequest{
		AthleteID:            athleteID,
		ProgramName:          req.ProgramName,
		NumDays:              req.NumDays,
		NumWeeks:             req.NumWeeks,
		IsLoop:               req.IsLoop,
		FocusAreas:           req.FocusAreas,
		CoachDirections:      req.CoachDirections,
		ReferenceTemplateIDs: req.ReferenceIDs,
	}

	// Call LLM (with timeout).
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	result, err := llm.Generate(ctx, h.DB, provider, genReq)
	if err != nil {
		log.Printf("api: LLM generation failed for athlete %d: %v", athleteID, err)
		msg := "Generation failed: " + err.Error()
		var apiErr *llm.APIError
		if errors.As(err, &apiErr) {
			msg = apiErr.UserMessage()
		}
		WriteError(w, http.StatusInternalServerError, msg)
		return
	}

	// Parse the catalog JSON to build mappings.
	var parsed *importers.ParsedFile
	if result.CatalogJSON != nil {
		var parseErr error
		parsed, parseErr = importers.ParseCatalogJSON(bytes.NewReader(result.CatalogJSON))
		if parseErr != nil {
			log.Printf("api: parse generated catalog: %v", parseErr)
			WriteError(w, http.StatusInternalServerError, "Failed to parse LLM output")
			return
		}
	} else {
		WriteError(w, http.StatusInternalServerError, "LLM returned empty output")
		return
	}

	// Build mappings.
	existing, _ := models.ListExercises(h.DB, "")
	entities := exercisesToEntities(existing)
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, entities),
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

	// Store in memory for execute step (avoids gob encoding issues with session store).
	h.generateCache.Store(athleteID, ms)

	truncated := result.StopReason == "max_tokens" || result.StopReason == "length"

	WriteJSON(w, http.StatusOK, GenerateSubmitResponse{
		Reasoning:  result.Reasoning,
		Model:      result.Model,
		TokensUsed: result.TokensUsed,
		Duration:   result.Duration.Round(time.Millisecond).String(),
		Truncated:  truncated,
		Programs:   len(ms.Programs),
		Exercises:  len(ms.Exercises),
	})
}

// GenerateExecute commits the generated program to the database.
func (h *Handlers) GenerateExecute(w http.ResponseWriter, r *http.Request) {
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

	val, ok := h.generateCache.Load(athleteID)
	if !ok {
		WriteError(w, http.StatusBadRequest, "no generation in progress — submit first")
		return
	}
	ms := val.(*importers.MappingState)

	result, err := models.ExecuteCatalogImport(h.DB, ms, &athleteID)
	if err != nil {
		log.Printf("api: execute generated program for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "Failed to save program: "+err.Error())
		return
	}

	// Auto-assign exercises for created programs.
	for _, templateID := range result.CreatedTemplateIDs {
		if _, err := models.AssignProgramExercises(h.DB, athleteID, templateID); err != nil {
			log.Printf("api: auto-assign exercises for template %d: %v", templateID, err)
		}
	}

	h.generateCache.Delete(athleteID)

	WriteJSON(w, http.StatusOK, map[string]any{
		"programs_created":   result.ProgramsCreated,
		"exercises_created":  result.ExercisesCreated,
		"prescribed_sets":    result.PrescribedSets,
		"progression_rules":  result.ProgressionRules,
	})
}
