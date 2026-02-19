// Package llm provides LLM-assisted program generation for RepLog.
//
// The package implements a three-layer pipeline:
//  1. Context Assembly — gather per-athlete data into a structured document
//  2. LLM Generation — send context + prompt to a provider, get CatalogJSON back
//  3. Coach Review — preview, edit, approve/reject (handled by existing import UI)
package llm

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// AthleteContext is the structured data package sent to the LLM.
// Every field is specific to one athlete — the same function called
// for two different athletes produces completely different contexts.
type AthleteContext struct {
	Athlete         AthleteProfile   `json:"athlete"`
	Equipment       []string         `json:"available_equipment"`
	CurrentProgram  *ProgramSummary  `json:"current_program"`
	Performance     PerformanceData  `json:"performance"`
	CoachNotes      []NoteEntry      `json:"coach_notes"`
	Goals           GoalContext      `json:"goals"`
	ExerciseCatalog []ExerciseEntry  `json:"exercise_catalog"`
	RecentWorkouts  []WorkoutSummary `json:"recent_workouts"`
	PriorTemplates  []TemplateSummary `json:"prior_templates"`
}

// AthleteProfile contains the athlete's identity and summary stats.
type AthleteProfile struct {
	Name           string  `json:"name"`
	Tier           *string `json:"tier"`
	Goal           *string `json:"goal"`
	Notes          *string `json:"notes"`
	TrainingMonths int     `json:"training_months"`
	TotalWorkouts  int     `json:"total_workouts"`
	LatestBW       *float64 `json:"latest_body_weight,omitempty"`
}

// ProgramSummary describes the athlete's current or prior program.
type ProgramSummary struct {
	Name      string `json:"name"`
	NumWeeks  int    `json:"num_weeks"`
	NumDays   int    `json:"num_days"`
	IsLoop    bool   `json:"is_loop"`
	StartDate string `json:"start_date"`
	Active    bool   `json:"active"`
}

// PerformanceData holds training maxes and body weight history.
type PerformanceData struct {
	TrainingMaxes []TMEntry         `json:"training_maxes"`
	BodyWeights   []BodyWeightEntry `json:"body_weights"`
}

// TMEntry is a single training max snapshot.
type TMEntry struct {
	Exercise      string  `json:"exercise"`
	Weight        float64 `json:"weight"`
	EffectiveDate string  `json:"effective_date"`
}

// BodyWeightEntry is a single body weight reading.
type BodyWeightEntry struct {
	Date   string  `json:"date"`
	Weight float64 `json:"weight"`
}

// NoteEntry is a coach note, workout review, or journal entry.
type NoteEntry struct {
	Date    string `json:"date"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Author  string `json:"author,omitempty"`
	Pinned  bool   `json:"pinned,omitempty"`
}

// GoalContext holds the athlete's current goal and history.
type GoalContext struct {
	Current string   `json:"current"`
	History []string `json:"history,omitempty"`
}

// ExerciseEntry describes an available exercise for the LLM.
type ExerciseEntry struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Tier        *string `json:"tier"`
	FormNotes   *string `json:"form_notes,omitempty"`
	RestSeconds int     `json:"rest_seconds,omitempty"`
	Compatible  bool    `json:"compatible"`
}

// WorkoutSummary describes a recent workout with its sets.
type WorkoutSummary struct {
	Date  string       `json:"date"`
	Notes *string      `json:"notes,omitempty"`
	Sets  []SetSummary `json:"sets"`
}

// SetSummary is a single logged set.
type SetSummary struct {
	Exercise  string   `json:"exercise"`
	SetNumber int      `json:"set_number"`
	Reps      int      `json:"reps"`
	Weight    *float64 `json:"weight,omitempty"`
	RPE       *float64 `json:"rpe,omitempty"`
	RepType   string   `json:"rep_type"`
}

// TemplateSummary describes an existing program template.
type TemplateSummary struct {
	Name     string `json:"name"`
	NumWeeks int    `json:"num_weeks"`
	NumDays  int    `json:"num_days"`
	IsLoop   bool   `json:"is_loop"`
}

// BuildAthleteContext gathers all relevant data for one athlete into the
// structured context document that the LLM receives. This is the per-athlete
// "briefing packet" — pure server-side queries, no LLM involved.
func BuildAthleteContext(db *sql.DB, athleteID int64, now time.Time) (*AthleteContext, error) {
	ctx := &AthleteContext{}

	// Athlete profile.
	profile, err := buildProfile(db, athleteID, now)
	if err != nil {
		return nil, fmt.Errorf("llm: build profile: %w", err)
	}
	ctx.Athlete = *profile

	// Equipment.
	equip, err := buildEquipmentList(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build equipment: %w", err)
	}
	ctx.Equipment = equip

	// Current program.
	prog, err := buildCurrentProgram(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build program: %w", err)
	}
	ctx.CurrentProgram = prog

	// Training maxes.
	tms, err := buildTrainingMaxes(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build training maxes: %w", err)
	}
	ctx.Performance.TrainingMaxes = tms

	// Body weights.
	bws, err := buildBodyWeights(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build body weights: %w", err)
	}
	ctx.Performance.BodyWeights = bws

	// Coach notes (from athlete_notes + journal entries).
	notes, err := buildCoachNotes(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build coach notes: %w", err)
	}
	ctx.CoachNotes = notes

	// Goals.
	ctx.Goals = buildGoals(profile)

	// Exercise catalog (filtered by equipment compatibility).
	exercises, err := buildExerciseCatalog(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build exercise catalog: %w", err)
	}
	ctx.ExerciseCatalog = exercises

	// Recent workouts with sets.
	workouts, err := buildRecentWorkouts(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build recent workouts: %w", err)
	}
	ctx.RecentWorkouts = workouts

	// Prior program templates.
	templates, err := buildPriorTemplates(db)
	if err != nil {
		return nil, fmt.Errorf("llm: build prior templates: %w", err)
	}
	ctx.PriorTemplates = templates

	return ctx, nil
}

// buildProfile constructs the athlete profile with computed summary stats.
func buildProfile(db *sql.DB, athleteID int64, now time.Time) (*AthleteProfile, error) {
	athlete, err := models.GetAthleteByID(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("get athlete %d: %w", athleteID, err)
	}

	profile := &AthleteProfile{
		Name: athlete.Name,
	}
	if athlete.Tier.Valid {
		profile.Tier = &athlete.Tier.String
	}
	if athlete.Goal.Valid {
		profile.Goal = &athlete.Goal.String
	}
	if athlete.Notes.Valid {
		profile.Notes = &athlete.Notes.String
	}

	// Compute training months from earliest workout.
	page, err := models.ListWorkouts(db, athleteID, 0)
	if err != nil {
		return nil, fmt.Errorf("list workouts for profile: %w", err)
	}
	profile.TotalWorkouts = len(page.Workouts)

	// Walk pages to get total count and earliest date.
	allWorkouts := page.Workouts
	offset := 0
	for page.HasMore {
		offset += len(page.Workouts)
		page, err = models.ListWorkouts(db, athleteID, offset)
		if err != nil {
			return nil, fmt.Errorf("list workouts page: %w", err)
		}
		allWorkouts = append(allWorkouts, page.Workouts...)
	}
	profile.TotalWorkouts = len(allWorkouts)

	if len(allWorkouts) > 0 {
		// Earliest workout is the last in the list (sorted DESC).
		earliest := allWorkouts[len(allWorkouts)-1]
		if t, err := parseDate(earliest.Date); err == nil {
			months := int(now.Sub(t).Hours() / 24 / 30)
			profile.TrainingMonths = months
		}
	}

	// Latest body weight.
	bw, err := models.LatestBodyWeight(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("latest body weight: %w", err)
	}
	if bw != nil {
		profile.LatestBW = &bw.Weight
	}

	return profile, nil
}

// buildEquipmentList returns the names of equipment the athlete has access to.
func buildEquipmentList(db *sql.DB, athleteID int64) ([]string, error) {
	items, err := models.ListAthleteEquipment(db, athleteID)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.EquipmentName
	}
	return names, nil
}

// buildCurrentProgram returns the athlete's active program, or nil if none.
func buildCurrentProgram(db *sql.DB, athleteID int64) (*ProgramSummary, error) {
	prog, err := models.GetActiveProgram(db, athleteID)
	if err != nil {
		return nil, err
	}
	if prog == nil {
		return nil, nil
	}
	return &ProgramSummary{
		Name:      prog.TemplateName,
		NumWeeks:  prog.NumWeeks,
		NumDays:   prog.NumDays,
		IsLoop:    prog.IsLoop,
		StartDate: prog.StartDate,
		Active:    prog.Active,
	}, nil
}

// buildTrainingMaxes returns the athlete's current training maxes.
func buildTrainingMaxes(db *sql.DB, athleteID int64) ([]TMEntry, error) {
	tms, err := models.ListCurrentTrainingMaxes(db, athleteID)
	if err != nil {
		return nil, err
	}
	entries := make([]TMEntry, len(tms))
	for i, tm := range tms {
		entries[i] = TMEntry{
			Exercise:      tm.ExerciseName,
			Weight:        tm.Weight,
			EffectiveDate: tm.EffectiveDate,
		}
	}
	return entries, nil
}

// buildBodyWeights returns the athlete's recent body weight entries (up to 30).
func buildBodyWeights(db *sql.DB, athleteID int64) ([]BodyWeightEntry, error) {
	page, err := models.ListBodyWeights(db, athleteID, 0)
	if err != nil {
		return nil, err
	}
	entries := make([]BodyWeightEntry, len(page.Entries))
	for i, bw := range page.Entries {
		entries[i] = BodyWeightEntry{
			Date:   bw.Date,
			Weight: bw.Weight,
		}
	}
	return entries, nil
}

// buildCoachNotes returns a combined view of coach notes and relevant journal entries.
func buildCoachNotes(db *sql.DB, athleteID int64) ([]NoteEntry, error) {
	// Athlete notes (coach observations, pinned items).
	notes, err := models.ListAthleteNotes(db, athleteID, true)
	if err != nil {
		return nil, fmt.Errorf("list athlete notes: %w", err)
	}

	var entries []NoteEntry
	for _, n := range notes {
		entries = append(entries, NoteEntry{
			Date:    n.Date,
			Type:    "note",
			Content: n.Content,
			Author:  n.AuthorName,
			Pinned:  n.Pinned,
		})
	}

	// Journal entries (workout reviews, goal changes, etc.) — limit to 50 most recent.
	journal, err := models.ListJournalEntries(db, athleteID, true, 50)
	if err != nil {
		return nil, fmt.Errorf("list journal entries: %w", err)
	}
	for _, j := range journal {
		// Include reviews, goal changes, tier changes — skip workout and body_weight
		// entries since those are already covered by RecentWorkouts and BodyWeights.
		switch j.Type {
		case "review", "goal_change", "tier_change", "program_start", "note":
			entries = append(entries, NoteEntry{
				Date:    j.Date,
				Type:    j.Type,
				Content: j.Summary,
				Author:  j.Author,
			})
		}
	}

	return entries, nil
}

// buildGoals constructs the goal context from the athlete profile.
func buildGoals(profile *AthleteProfile) GoalContext {
	gc := GoalContext{}
	if profile.Goal != nil {
		gc.Current = *profile.Goal
	}
	return gc
}

// buildExerciseCatalog returns all exercises annotated with equipment compatibility.
func buildExerciseCatalog(db *sql.DB, athleteID int64) ([]ExerciseEntry, error) {
	exercises, err := models.ListExercises(db, "")
	if err != nil {
		return nil, err
	}

	// Get athlete's equipment IDs for compatibility checks.
	equipIDs, err := models.AthleteEquipmentIDs(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("athlete equipment IDs: %w", err)
	}

	entries := make([]ExerciseEntry, 0, len(exercises))
	for _, ex := range exercises {
		entry := ExerciseEntry{
			ID:          ex.ID,
			Name:        ex.Name,
			RestSeconds: ex.EffectiveRestSeconds(),
		}
		if ex.Tier.Valid {
			entry.Tier = &ex.Tier.String
		}
		if ex.FormNotes.Valid {
			entry.FormNotes = &ex.FormNotes.String
		}

		// Check equipment compatibility.
		compat, err := models.CheckExerciseCompatibility(db, athleteID, ex.ID)
		if err != nil {
			return nil, fmt.Errorf("check compatibility for exercise %d: %w", ex.ID, err)
		}
		entry.Compatible = compat.HasRequired

		// If the athlete has no equipment configured at all, mark everything
		// as compatible (no equipment constraints means no filtering).
		if len(equipIDs) == 0 {
			entry.Compatible = true
		}

		entries = append(entries, entry)
	}
	return entries, nil
}

// buildRecentWorkouts returns the athlete's most recent workouts with their sets.
// Returns up to 20 workouts.
func buildRecentWorkouts(db *sql.DB, athleteID int64) ([]WorkoutSummary, error) {
	page, err := models.ListWorkouts(db, athleteID, 0)
	if err != nil {
		return nil, err
	}

	// Take up to 20 most recent.
	workouts := page.Workouts
	if len(workouts) > 20 {
		workouts = workouts[:20]
	}

	summaries := make([]WorkoutSummary, 0, len(workouts))
	for _, w := range workouts {
		ws := WorkoutSummary{
			Date: normalizeDate(w.Date),
		}
		if w.Notes.Valid {
			ws.Notes = &w.Notes.String
		}

		// Get sets grouped by exercise.
		groups, err := models.ListSetsByWorkout(db, w.ID)
		if err != nil {
			return nil, fmt.Errorf("list sets for workout %d: %w", w.ID, err)
		}
		for _, g := range groups {
			for _, s := range g.Sets {
				ss := SetSummary{
					Exercise:  g.ExerciseName,
					SetNumber: s.SetNumber,
					Reps:      s.Reps,
					RepType:   s.RepType,
				}
				if s.Weight.Valid {
					ss.Weight = &s.Weight.Float64
				}
				if s.RPE.Valid {
					ss.RPE = &s.RPE.Float64
				}
				ws.Sets = append(ws.Sets, ss)
			}
		}

		summaries = append(summaries, ws)
	}
	return summaries, nil
}

// buildPriorTemplates returns all existing program templates as context.
func buildPriorTemplates(db *sql.DB) ([]TemplateSummary, error) {
	templates, err := models.ListProgramTemplates(db)
	if err != nil {
		return nil, err
	}
	summaries := make([]TemplateSummary, len(templates))
	for i, t := range templates {
		summaries[i] = TemplateSummary{
			Name:     t.Name,
			NumWeeks: t.NumWeeks,
			NumDays:  t.NumDays,
			IsLoop:   t.IsLoop,
		}
	}
	return summaries, nil
}

// parseDate parses a date string in either "2006-01-02" or "2006-01-02T15:04:05Z" format.
func parseDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02T15:04:05Z", s)
}

// normalizeDate extracts the YYYY-MM-DD portion from a date string.
func normalizeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
