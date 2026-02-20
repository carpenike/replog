package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/models"
)

func init() {
	gob.Register(&llm.GenerationResult{})
}

// Generate handles AI Coach program generation.
type Generate struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	Templates TemplateCache
}

// Form renders the program generation form for an athlete.
func (h *Generate) Form(w http.ResponseWriter, r *http.Request) {
	athlete, athleteID, ok := h.loadAthlete(w, r)
	if !ok {
		return
	}

	configured := models.IsAICoachConfigured(h.DB)

	// Get current active program for pre-filling defaults.
	active, _ := models.GetActiveProgram(h.DB, athleteID)

	suggestedName := "New Program"
	numDays := 3
	numWeeks := 4
	isLoop := false
	if active != nil {
		suggestedName = suggestNextProgramName(active.TemplateName)
		numDays = active.NumDays
		numWeeks = active.NumWeeks
		isLoop = active.IsLoop
	}

	// Build athlete context for the LLM preview panel.
	athleteCtx, ctxErr := llm.BuildAthleteContext(h.DB, athleteID, time.Now())
	if ctxErr != nil {
		log.Printf("handlers: build context preview for athlete %d: %v", athleteID, ctxErr)
	}

	data := map[string]any{
		"Athlete":       athlete,
		"Configured":    configured,
		"SuggestedName": suggestedName,
		"NumDays":       numDays,
		"NumWeeks":      numWeeks,
		"IsLoop":        isLoop,
		"Context":       athleteCtx,
	}
	if err := h.Templates.Render(w, r, "generate_form.html", data); err != nil {
		log.Printf("handlers: render generate form: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// Submit handles the generation form POST: assembles context, calls LLM,
// parses CatalogJSON into import mappings, and redirects to preview.
func (h *Generate) Submit(w http.ResponseWriter, r *http.Request) {
	athlete, athleteID, ok := h.loadAthlete(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Build generation request from form values.
	programName := strings.TrimSpace(r.FormValue("program_name"))
	if programName == "" {
		programName = "Generated Program"
	}

	numDays, _ := strconv.Atoi(r.FormValue("num_days"))
	if numDays < 1 || numDays > 7 {
		numDays = 3
	}

	numWeeks, _ := strconv.Atoi(r.FormValue("num_weeks"))
	if numWeeks < 1 || numWeeks > 52 {
		numWeeks = 4
	}

	isLoop := r.FormValue("is_loop") == "1"
	if isLoop {
		numWeeks = 1
	}

	coachDirections := strings.TrimSpace(r.FormValue("coach_directions"))
	focusAreas := r.Form["focus_areas"]

	req := llm.GenerationRequest{
		AthleteID:       athleteID,
		ProgramName:     programName,
		NumDays:         numDays,
		NumWeeks:        numWeeks,
		IsLoop:          isLoop,
		FocusAreas:      focusAreas,
		CoachDirections: coachDirections,
	}

	// Create provider from settings.
	provider, err := llm.NewProviderFromSettings(h.DB)
	if err != nil {
		log.Printf("handlers: create LLM provider: %v", err)
		h.renderFormError(w, r, athlete, req,
			"AI Coach is not configured. Please ask an administrator to configure it in Settings.")
		return
	}

	// Call the LLM — large prompts with 16k max tokens can take 2–4 minutes.
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()

	log.Printf("handlers: starting LLM generation for athlete %d (%s, %d days, %d weeks)",
		athleteID, programName, numDays, numWeeks)
	result, err := llm.Generate(ctx, h.DB, provider, req)
	if err != nil {
		log.Printf("handlers: generate program for athlete %d: %v", athleteID, err)
		var apiErr *llm.APIError
		switch {
		case errors.As(err, &apiErr):
			h.renderFormError(w, r, athlete, req, apiErr.UserMessage())
		case errors.Is(err, context.DeadlineExceeded):
			h.renderFormError(w, r, athlete, req,
				"Generation timed out. The AI provider took too long to respond. Please try again or simplify your directions.")
		case errors.Is(err, context.Canceled):
			h.renderFormError(w, r, athlete, req,
				"Generation was canceled. Please try again.")
		default:
			h.renderFormError(w, r, athlete, req,
				fmt.Sprintf("Generation failed: %v", err))
		}
		return
	}

	log.Printf("handlers: LLM generation complete for athlete %d: model=%s tokens=%d duration=%s catalog_bytes=%d stop_reason=%s",
		athleteID, result.Model, result.TokensUsed, result.Duration, len(result.CatalogJSON), result.StopReason)

	// Check for output truncation — the model ran out of tokens before completing JSON.
	isTruncated := result.StopReason == "max_tokens" || result.StopReason == "length"

	// Parse CatalogJSON into import structures.
	if len(result.CatalogJSON) == 0 {
		log.Printf("handlers: empty CatalogJSON from LLM for athlete %d, raw response length=%d stop_reason=%s",
			athleteID, len(result.RawResponse), result.StopReason)
		if len(result.RawResponse) > 0 {
			// Log a truncated preview to help debug extraction failures.
			preview := result.RawResponse
			if len(preview) > 2000 {
				preview = preview[:2000] + "... [truncated]"
			}
			log.Printf("handlers: raw LLM response preview: %s", preview)
		}

		var errMsg string
		switch {
		case isTruncated:
			errMsg = "The AI Coach ran out of output tokens before completing the program JSON. " +
				"Try reducing the number of training days or weeks, or simplifying your directions."
			if result.Reasoning != "" {
				errMsg += fmt.Sprintf("\n\nThe AI's reasoning before truncation: %s", result.Reasoning)
			}
		case result.Reasoning != "":
			errMsg = fmt.Sprintf("The AI Coach provided reasoning but no valid program JSON. Its reasoning: %s", result.Reasoning)
		default:
			errMsg = "The AI Coach did not return valid program data. Please try again with different directions."
		}
		h.renderFormError(w, r, athlete, req, errMsg)
		return
	}

	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(result.CatalogJSON))
	if err != nil {
		log.Printf("handlers: parse LLM CatalogJSON: %v", err)
		h.renderFormError(w, r, athlete, req,
			fmt.Sprintf("The AI Coach returned invalid data: %v. Please try again.", err))
		return
	}

	// Build entity mappings against existing catalog.
	existingExercises, _ := listExistingExercises(h.DB)
	existingEquipment, _ := listExistingEquipment(h.DB)
	existingPrograms, _ := listExistingProgramsForAthlete(h.DB, athleteID)

	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Parsed:    parsed,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, existingExercises),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, existingEquipment),
		Programs:  importers.BuildProgramMappings(parsed.Programs, existingPrograms),
	}

	// Exercises referenced in prescribed_sets/progression_rules may not appear
	// in the catalog's "exercises" array (they're existing DB exercises the AI
	// used without re-declaring). Merge those into the exercise mappings so the
	// import can resolve their IDs.
	progExNames := importers.CollectProgramExerciseNames(parsed.Programs)
	if len(progExNames) > 0 {
		progExParsed := make([]importers.ParsedExercise, len(progExNames))
		for i, name := range progExNames {
			progExParsed[i] = importers.ParsedExercise{Name: name}
		}
		progExMappings := importers.BuildExerciseMappings(progExParsed, existingExercises)
		ms.Exercises = importers.MergeExerciseMappings(ms.Exercises, progExMappings)
	}

	// Store results in session for preview/execute.
	h.Sessions.Put(r.Context(), "generate_result", result)
	h.Sessions.Put(r.Context(), "generate_mapping", ms)

	http.Redirect(w, r, fmt.Sprintf("/athletes/%d/programs/generate/preview", athleteID), http.StatusSeeOther)
}

// Preview shows the LLM reasoning and catalog import summary.
func (h *Generate) Preview(w http.ResponseWriter, r *http.Request) {
	athlete, athleteID, ok := h.loadAthlete(w, r)
	if !ok {
		return
	}

	result, ok := h.Sessions.Get(r.Context(), "generate_result").(*llm.GenerationResult)
	if !ok || result == nil {
		http.Redirect(w, r, fmt.Sprintf("/athletes/%d/programs/generate", athleteID), http.StatusSeeOther)
		return
	}

	ms, ok := h.Sessions.Get(r.Context(), "generate_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, fmt.Sprintf("/athletes/%d/programs/generate", athleteID), http.StatusSeeOther)
		return
	}

	preview := models.BuildCatalogImportPreview(ms)

	// Build structured program view for the detail display.
	var programViews []programDayView
	if ms.Parsed != nil {
		for _, p := range ms.Parsed.Programs {
			programViews = append(programViews, buildProgramDays(p.Template)...)
		}
	}

	data := map[string]any{
		"Athlete":     athlete,
		"Result":      result,
		"Preview":     preview,
		"Mapping":     ms,
		"ProgramDays": programViews,
	}
	if err := h.Templates.Render(w, r, "generate_preview.html", data); err != nil {
		log.Printf("handlers: render generate preview: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// Execute approves the generated program and imports it.
func (h *Generate) Execute(w http.ResponseWriter, r *http.Request) {
	athlete, athleteID, ok := h.loadAthlete(w, r)
	if !ok {
		return
	}

	ms, ok := h.Sessions.Get(r.Context(), "generate_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, fmt.Sprintf("/athletes/%d/programs/generate", athleteID), http.StatusSeeOther)
		return
	}

	importResult, err := models.ExecuteCatalogImport(h.DB, ms, &athleteID)
	if err != nil {
		log.Printf("handlers: execute generated import for athlete %d: %v", athleteID, err)
		data := map[string]any{
			"Athlete": athlete,
			"Error":   fmt.Sprintf("Import failed: %v", err),
		}
		h.Templates.Render(w, r, "generate_form.html", data)
		return
	}

	// Get generation result for display metadata.
	genResult, _ := h.Sessions.Get(r.Context(), "generate_result").(*llm.GenerationResult)

	// Clear session data.
	h.Sessions.Remove(r.Context(), "generate_result")
	h.Sessions.Remove(r.Context(), "generate_mapping")

	data := map[string]any{
		"Athlete":      athlete,
		"ImportResult": importResult,
		"GenResult":    genResult,
	}
	if err := h.Templates.Render(w, r, "generate_result.html", data); err != nil {
		log.Printf("handlers: render generate result: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// --- Helpers ---

// loadAthlete retrieves the athlete and checks access. Returns the athlete,
// its ID, and whether the caller should continue.
func (h *Generate) loadAthlete(w http.ResponseWriter, r *http.Request) (*models.Athlete, int64, bool) {
	athleteID, ok := checkAthleteAccess(h.DB, h.Templates, w, r)
	if !ok {
		return nil, 0, false
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return nil, 0, false
	}
	if err != nil {
		log.Printf("handlers: get athlete %d: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return nil, 0, false
	}
	return athlete, athleteID, true
}

// renderFormError re-renders the generate form with an error message,
// preserving the user\u2019s input values.
func (h *Generate) renderFormError(w http.ResponseWriter, r *http.Request, athlete *models.Athlete, req llm.GenerationRequest, errMsg string) {
	data := map[string]any{
		"Athlete":       athlete,
		"Error":         errMsg,
		"SuggestedName": req.ProgramName,
		"NumDays":       req.NumDays,
		"NumWeeks":      req.NumWeeks,
		"IsLoop":        req.IsLoop,
		"Configured":    true,
	}
	if err := h.Templates.Render(w, r, "generate_form.html", data); err != nil {
		log.Printf("handlers: render generate form with error: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// ContextJSON returns the full athlete context as JSON. This lets coaches
// copy the context for use with external LLMs or for debugging.
// GET /athletes/{id}/context.json
func (h *Generate) ContextJSON(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	ctx, err := llm.BuildAthleteContext(h.DB, id, time.Now())
	if err != nil {
		log.Printf("handlers: build athlete context for %d: %v", id, err)
		http.Error(w, "Failed to build context", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="athlete-%d-context.json"`, id))
	if err := json.NewEncoder(w).Encode(ctx); err != nil {
		log.Printf("handlers: encode athlete context JSON for %d: %v", id, err)
	}
}

// suggestNextProgramName auto-increments a trailing number in the program name.
// "Sport Performance Month 3" -> "Sport Performance Month 4"
func suggestNextProgramName(current string) string {
	parts := strings.Fields(current)
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if n, err := strconv.Atoi(last); err == nil {
			parts[len(parts)-1] = strconv.Itoa(n + 1)
			return strings.Join(parts, " ")
		}
	}
	return current + " 2"
}

// --- Program preview structures ---

// programDayView groups exercises and sets for one training day in one program.
type programDayView struct {
	ProgramName string
	Description string
	NumWeeks    int
	NumDays     int
	IsLoop      bool
	Week        int
	Day         int
	Exercises   []programExerciseView
}

// programExerciseView groups sets for one exercise within a day.
type programExerciseView struct {
	Name       string
	SortOrder  int
	Sets       []programSetView
	SetsReps   string // e.g. "3×5", "2×5 + 1×AMRAP"
	WeightStr  string // consolidated weight (from first set)
	FirstNotes string // notes from first set
}

// programSetView is a single prescribed set for template display.
type programSetView struct {
	SetNumber int
	RepsStr   string  // formatted reps string, e.g. "5", "AMRAP", "30s", "8 each"
	WeightStr string  // formatted weight, e.g. "BW", "25 lbs", "75%"
	Notes     string
}

// buildProgramDays converts a ParsedProgramTemplate into a slice of day views,
// one per unique (week, day) combination, with exercises sorted by sort_order.
func buildProgramDays(tmpl importers.ParsedProgramTemplate) []programDayView {
	// Group sets by (week, day) → exercise name → sets.
	type dayKey struct{ Week, Day int }
	type exerciseGroup struct {
		sortOrder int
		sets      []programSetView
		name      string
	}

	dayMap := make(map[dayKey]map[string]*exerciseGroup)

	for _, s := range tmpl.PrescribedSets {
		dk := dayKey{s.Week, s.Day}
		if dayMap[dk] == nil {
			dayMap[dk] = make(map[string]*exerciseGroup)
		}
		eg, ok := dayMap[dk][s.Exercise]
		if !ok {
			eg = &exerciseGroup{name: s.Exercise, sortOrder: s.SortOrder}
			dayMap[dk][s.Exercise] = eg
		}

		sv := programSetView{
			SetNumber: s.SetNumber,
			RepsStr:   formatSetReps(s.Reps, s.RepType),
			WeightStr: formatSetWeight(s.Percentage, s.AbsoluteWeight),
		}
		if s.Notes != nil {
			sv.Notes = *s.Notes
		}
		eg.sets = append(eg.sets, sv)
	}

	// Collect and sort day keys.
	var keys []dayKey
	for k := range dayMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Week != keys[j].Week {
			return keys[i].Week < keys[j].Week
		}
		return keys[i].Day < keys[j].Day
	})

	var days []programDayView
	for _, dk := range keys {
		// Sort exercises by sort_order.
		var exercises []programExerciseView
		for _, eg := range dayMap[dk] {
			ev := programExerciseView{
				Name:      eg.name,
				SortOrder: eg.sortOrder,
				Sets:      eg.sets,
				SetsReps:  summarizeSetsReps(eg.sets),
			}
			if len(eg.sets) > 0 {
				ev.WeightStr = eg.sets[0].WeightStr
				ev.FirstNotes = eg.sets[0].Notes
			}
			exercises = append(exercises, ev)
		}
		sort.Slice(exercises, func(i, j int) bool {
			return exercises[i].SortOrder < exercises[j].SortOrder
		})

		desc := ""
		if tmpl.Description != nil {
			desc = *tmpl.Description
		}

		days = append(days, programDayView{
			ProgramName: tmpl.Name,
			Description: desc,
			NumWeeks:    tmpl.NumWeeks,
			NumDays:     tmpl.NumDays,
			IsLoop:      tmpl.IsLoop,
			Week:        dk.Week,
			Day:         dk.Day,
			Exercises:   exercises,
		})
	}
	return days
}

// formatSetReps formats reps for display.
func formatSetReps(reps *int, repType string) string {
	if reps == nil {
		return "AMRAP"
	}
	r := *reps
	switch repType {
	case "seconds":
		return fmt.Sprintf("%ds", r)
	case "each_side":
		return fmt.Sprintf("%d each", r)
	case "distance":
		return fmt.Sprintf("%dm", r)
	default:
		return fmt.Sprintf("%d", r)
	}
}

// summarizeSetsReps produces a compact string like "3×5" or "2×5 + 1×AMRAP".
func summarizeSetsReps(sets []programSetView) string {
	if len(sets) == 0 {
		return ""
	}

	// Group consecutive sets by reps string.
	type group struct {
		reps  string
		count int
	}
	var groups []group
	for _, s := range sets {
		if len(groups) > 0 && groups[len(groups)-1].reps == s.RepsStr {
			groups[len(groups)-1].count++
		} else {
			groups = append(groups, group{reps: s.RepsStr, count: 1})
		}
	}

	// If all sets are the same, just "3×5".
	if len(groups) == 1 {
		return fmt.Sprintf("%d×%s", groups[0].count, groups[0].reps)
	}

	// Otherwise "2×5 + 1×AMRAP".
	var parts []string
	for _, g := range groups {
		parts = append(parts, fmt.Sprintf("%d×%s", g.count, g.reps))
	}
	return strings.Join(parts, " + ")
}

// formatSetWeight formats the weight/loading for display.
func formatSetWeight(percentage *float64, absoluteWeight *float64) string {
	if percentage != nil && *percentage > 0 {
		return fmt.Sprintf("%.0f%%", *percentage*100)
	}
	if absoluteWeight != nil {
		if *absoluteWeight == 0 {
			return "BW"
		}
		if *absoluteWeight == float64(int(*absoluteWeight)) {
			return fmt.Sprintf("%.0f lbs", *absoluteWeight)
		}
		return fmt.Sprintf("%.1f lbs", *absoluteWeight)
	}
	return "BW"
}
