// Package llm provides LLM-assisted program generation for RepLog.
//
// The package implements a three-layer pipeline:
//  1. Context Assembly — gather per-athlete data into a structured document
//  2. LLM Generation — send context + prompt to a provider, get CatalogJSON back
//  3. Coach Review — preview, edit, approve/reject (handled by existing import UI)
package llm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// AthleteContext is the structured data package sent to the LLM.
// Every field is specific to one athlete — the same function called
// for two different athletes produces completely different contexts.
type AthleteContext struct {
	Athlete           AthleteProfile            `json:"athlete"`
	Equipment         []string                  `json:"available_equipment"`
	CurrentPrograms   []ProgramSummary          `json:"current_programs"`
	ProgramHistory    []ProgramHistoryEntry     `json:"program_history"`
	Performance       PerformanceData           `json:"performance"`
	CoachNotes        []NoteEntry               `json:"coach_notes"`
	Goals             GoalContext               `json:"goals"`
	ExerciseCatalog   []ExerciseEntry           `json:"exercise_catalog"`
	RecentWorkouts    []WorkoutSummary          `json:"recent_workouts"`
	RecentActivity    RecentActivity            `json:"recent_activity"`
	ReferencePrograms []ReferenceProgramSummary `json:"reference_programs"`
	PriorTemplates    []TemplateSummary         `json:"prior_templates"`

	// Methodology is a lean projection of the selected program-design
	// methodology (ADR 016 Phase 2). When non-nil, the LLM was given a
	// methodology-specific per-tier prompt block sourced from
	// methodology.definition and an exercise catalog scoped to the
	// methodology's allow-lists.
	//
	// This intentionally OMITS the full `definition` text — that text
	// is already persisted on the generation row's `prompt` column;
	// duplicating it into `context_json` would grow the audit table
	// linearly with definition copy edits. The full
	// *models.MethodologyWithLinks is held in the unexported
	// `methodology` field below for in-process use by buildSystemPrompt,
	// buildUserPrompt, and buildExerciseCatalog.
	Methodology *MethodologyProjection `json:"methodology,omitempty"`

	// methodology is the full struct used by the prompt + scoping helpers
	// within this package. Unexported so it never leaks into context_json
	// or out to the SPA, and intentionally kept off the marshalled
	// AthleteContext above. Same package can access via ctx.methodology.
	methodology *models.MethodologyWithLinks
}

// MethodologyProjection is the lean view of a methodology that ships in
// the marshalled AthleteContext (ADR 016 Phase 2 — keep audit rows small).
// The full `definition` lives in the prompt column on the generation row.
type MethodologyProjection struct {
	ID                 int64    `json:"id"`
	Key                string   `json:"key"`
	Name               string   `json:"name"`
	Audience           string   `json:"audience,omitempty"`
	ApplicableTiers    string   `json:"applicable_tiers,omitempty"`
	Philosophy         string   `json:"philosophy,omitempty"`
	AllowedPatterns    []string `json:"allowed_patterns,omitempty"`
	AllowedEquipmentN  int      `json:"allowed_equipment_count"`
	AllowedExercisesN  int      `json:"allowed_exercises_count"`
	ReferenceProgramsN int      `json:"reference_programs_count"`
}

// AthleteProfile contains the athlete's identity and summary stats.
type AthleteProfile struct {
	Name           string   `json:"name"`
	Tier           *string  `json:"tier"`
	Goal           *string  `json:"goal"`
	Notes          *string  `json:"notes"`
	Age            *int     `json:"age,omitempty"`
	Grade          *string  `json:"grade,omitempty"`
	Gender         *string  `json:"gender,omitempty"`
	TrainingMonths int      `json:"training_months"`
	TotalWorkouts  int      `json:"total_workouts"`
	LatestBW       *float64 `json:"latest_body_weight,omitempty"`
	// WeightUnit is the unit ("lbs" or "kg") that every weight in this context
	// — and every weight the LLM emits — is expressed in. Sourced from the app
	// default weight-unit setting so the model never has to guess.
	WeightUnit string `json:"weight_unit"`
}

// RecentActivity is a compact rollup of the ADR-018 cross-discipline signals
// (season phase, recovery, throwing/conditioning volume, training-load
// advisories) that bear on programming decisions — especially youth safety.
type RecentActivity struct {
	SeasonPhase     *string           `json:"season_phase,omitempty"`
	LatestRecovery  *RecoverySnapshot `json:"latest_recovery,omitempty"`
	Throwing14d     *DisciplineVolume `json:"throwing_14d,omitempty"`
	Conditioning14d *DisciplineVolume `json:"conditioning_14d,omitempty"`
	// LoadAdvisory carries short human-readable flags derived from the ACWR
	// load summary (e.g. "throwing ACWR 1.6 — elevated"). Empty when nothing
	// is noteworthy.
	LoadAdvisory []string `json:"load_advisory,omitempty"`
}

// RecoverySnapshot is the most recent recovery check-in in compact form.
type RecoverySnapshot struct {
	Date       string   `json:"date"`
	SleepHours *float64 `json:"sleep_hours,omitempty"`
	Soreness   *int64   `json:"soreness,omitempty"` // 1-10
	Energy     *int64   `json:"energy,omitempty"`   // 1-10
	Notes      string   `json:"notes,omitempty"`
}

// DisciplineVolume summarizes recent session count/volume for one discipline.
type DisciplineVolume struct {
	Sessions int    `json:"sessions"`
	Detail   string `json:"detail,omitempty"` // e.g. "142 throws", "3200s"
}

// ProgramSummary describes one of the athlete's currently active programs.
type ProgramSummary struct {
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	Schedule  *string `json:"schedule,omitempty"`
	NumWeeks  int     `json:"num_weeks"`
	NumDays   int     `json:"num_days"`
	IsLoop    bool    `json:"is_loop"`
	StartDate string  `json:"start_date"`
	Active    bool    `json:"active"`
}

// ProgramHistoryEntry describes one program assignment (active or past).
type ProgramHistoryEntry struct {
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	Schedule  *string `json:"schedule,omitempty"`
	NumWeeks  int     `json:"num_weeks"`
	NumDays   int     `json:"num_days"`
	IsLoop    bool    `json:"is_loop"`
	StartDate string  `json:"start_date"`
	Active    bool    `json:"active"`
	Notes     *string `json:"notes,omitempty"`
	Goal      *string `json:"goal,omitempty"`
}

// PerformanceData holds training maxes and body weight history.
type PerformanceData struct {
	TrainingMaxes []TMEntry             `json:"training_maxes"`
	BodyWeights   []BodyWeightEntry     `json:"body_weights"`
	Trends        []ExercisePerformance `json:"trends,omitempty"`
}

// ExercisePerformance holds computed performance trends for a single exercise
// from the athlete's recent workout history.
type ExercisePerformance struct {
	Exercise  string   `json:"exercise"`
	AvgRPE    *float64 `json:"avg_rpe,omitempty"`
	MaxWeight *float64 `json:"max_weight,omitempty"`
	TotalSets int      `json:"total_sets"`
	TotalReps int      `json:"total_reps"`
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

// TemplateSummary describes an existing program template (athlete-scoped, metadata only).
type TemplateSummary struct {
	Name     string `json:"name"`
	NumWeeks int    `json:"num_weeks"`
	NumDays  int    `json:"num_days"`
	IsLoop   bool   `json:"is_loop"`
}

// ReferenceProgramSummary is a global seed/reference program with full prescribed sets.
// Included so the LLM can see concrete structural examples for the athlete's audience.
type ReferenceProgramSummary struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	NumWeeks    int    `json:"num_weeks"`
	NumDays     int    `json:"num_days"`
	IsLoop      bool   `json:"is_loop"`
	Audience    string `json:"audience,omitempty"`
	// Phase labels a youth reference program with the tier it represents
	// ("foundational", "intermediate", "sport_performance") so the LLM can
	// tell which of the youth references matches the athlete's current tier.
	// Empty for adult programs and any youth program whose name we don't
	// recognise (defensive — adding a new seeded youth program is not a
	// silent labeling failure, it's an unlabeled program).
	Phase          string                 `json:"phase,omitempty"`
	PrescribedSets []PrescribedSetSummary `json:"prescribed_sets"`
}

// PrescribedSetSummary is a single prescribed set within a reference program.
type PrescribedSetSummary struct {
	Exercise       string   `json:"exercise"`
	Week           int      `json:"week"`
	Day            int      `json:"day"`
	SetNumber      int      `json:"set_number"`
	Reps           *int     `json:"reps,omitempty"`
	RepType        string   `json:"rep_type"`
	Percentage     *float64 `json:"percentage,omitempty"`
	AbsoluteWeight *float64 `json:"absolute_weight,omitempty"`
	RestSeconds    *int     `json:"rest_seconds,omitempty"`
	SortOrder      int      `json:"sort_order"`
	Notes          string   `json:"notes,omitempty"`
}

// BuildAthleteContext gathers all relevant data for one athlete into the
// structured context document that the LLM receives. This is the per-athlete
// "briefing packet" — pure server-side queries, no LLM involved.
//
// If referenceTemplateIDs is non-empty, only those specific templates are
// included as reference programs. Otherwise all audience-matching global
// templates are included (audience inferred from the athlete's tier).
// BuildContextOptions controls how BuildAthleteContext resolves the data
// package for a generation. All fields are optional; the zero value
// preserves pre-ADR-016-Phase-2 behavior (no methodology resolution, no
// catalog scoping, audience-filtered references).
type BuildContextOptions struct {
	// ReferenceTemplateIDs, when non-empty, overrides BOTH the
	// methodology's default exemplars and the audience-filtered fallback.
	// This is the coach's explicit "use exactly these reference programs"
	// path.
	ReferenceTemplateIDs []int64

	// MethodologyID, when set, names the methodology to resolve. When nil
	// AND RequireMethodology is true AND the athlete is youth, the methodology
	// is resolved by tier (foundational → yessis-1x20, intermediate →
	// yessis-1x15, sport_performance → yessis-sport-performance). When nil
	// for adults, no methodology is bound and the LLM falls back to the
	// in-code generic adult block.
	MethodologyID *int64

	// RequireMethodology gates the youth tier-default resolution AND the
	// "no methodology configured for this athlete" error. Generation paths
	// set this to true; form-preview / audit-replay paths leave it false
	// because they don't need a fully-resolved methodology.
	RequireMethodology bool
}

// BuildAthleteContext gathers all relevant data for one athlete into the
// LLM-facing AthleteContext. When opts.MethodologyID is set OR
// opts.RequireMethodology is true for a youth athlete, the resolved
// methodology drives:
//
//   - the per-tier prompt block (definition flows through ctx.methodology)
//   - the exercise catalog scope (allow-list filters in buildExerciseCatalog)
//   - the default reference programs (methodology's exemplars; empty-exemplar
//     fallback to today's audience-filtered behavior when both override list
//     AND exemplars are empty).
//
// Coach-supplied ReferenceTemplateIDs always override the methodology's
// default exemplars.
func BuildAthleteContext(ctx context.Context, db *sql.DB, athleteID int64, now time.Time, opts BuildContextOptions) (*AthleteContext, error) {
	ac := &AthleteContext{}

	// Athlete profile.
	profile, err := buildProfile(ctx, db, athleteID, now)
	if err != nil {
		return nil, fmt.Errorf("llm: build profile: %w", err)
	}
	ac.Athlete = *profile

	// Equipment.
	equip, err := buildEquipmentList(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build equipment: %w", err)
	}
	ac.Equipment = equip

	// Current programs (primary + supplementals).
	currentProgs, err := buildCurrentPrograms(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build programs: %w", err)
	}
	ac.CurrentPrograms = currentProgs

	// Program history (all assignments, active + past).
	history, err := buildProgramHistory(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build program history: %w", err)
	}
	ac.ProgramHistory = history

	// Training maxes.
	tms, err := buildTrainingMaxes(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build training maxes: %w", err)
	}
	ac.Performance.TrainingMaxes = tms

	// Body weights.
	bws, err := buildBodyWeights(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build body weights: %w", err)
	}
	ac.Performance.BodyWeights = bws

	// Coach notes (from athlete_notes + journal entries).
	notes, err := buildCoachNotes(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build coach notes: %w", err)
	}
	ac.CoachNotes = notes

	// Goals (with history from goal_history table).
	ac.Goals = buildGoals(ctx, db, profile, athleteID)

	// Resolve methodology (ADR 016 Phase 2). MUST run before
	// buildExerciseCatalog because the catalog is scoped to the
	// methodology's allow-lists when one is bound.
	methodology, err := resolveMethodology(ctx, db, profile, opts)
	if err != nil {
		return nil, err
	}
	ac.methodology = methodology
	if methodology != nil {
		ac.Methodology = projectMethodology(methodology)
	}

	// Exercise catalog (filtered by equipment compatibility + methodology
	// allow-lists when a methodology is bound).
	exercises, err := buildExerciseCatalog(ctx, db, athleteID, methodology)
	if err != nil {
		return nil, fmt.Errorf("llm: build exercise catalog: %w", err)
	}
	ac.ExerciseCatalog = exercises

	// Recent workouts with sets.
	workouts, err := buildRecentWorkouts(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build recent workouts: %w", err)
	}
	ac.RecentWorkouts = workouts

	// Exercise performance trends (computed from recent workouts).
	ac.Performance.Trends = buildPerformanceTrends(workouts)

	// Cross-discipline recent-activity rollup (ADR 018): season phase,
	// recovery, throwing/conditioning volume, and load advisories. Best-effort
	// — a query failure here degrades the context but must not fail generation,
	// since none of it is load-bearing for the JSON schema.
	ac.RecentActivity = buildRecentActivity(ctx, db, athleteID, now)

	// Prior program templates (athlete-scoped only — previously generated for this athlete).
	templates, err := buildPriorTemplates(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("llm: build prior templates: %w", err)
	}
	ac.PriorTemplates = templates

	// Reference programs:
	//   1. coach-supplied ReferenceTemplateIDs override everything;
	//   2. else if methodology has exemplars, use those;
	//   3. else fall back to today's audience-filtered behavior (so a
	//      methodology like int-youth-gpp with no seeded exemplars
	//      doesn't ship zero references — see ADR 016 D4).
	var refProgs []ReferenceProgramSummary
	switch {
	case len(opts.ReferenceTemplateIDs) > 0:
		refProgs, err = buildReferenceProgramsByIDs(ctx, db, opts.ReferenceTemplateIDs)
	case methodology != nil && len(methodology.ReferenceProgramIDs) > 0:
		refProgs, err = buildReferenceProgramsByIDs(ctx, db, methodology.ReferenceProgramIDs)
	default:
		audience := "adult"
		if profile.Tier != nil {
			audience = "youth"
		}
		refProgs, err = buildReferencePrograms(ctx, db, audience)
	}
	if err != nil {
		return nil, fmt.Errorf("llm: build reference programs: %w", err)
	}
	// Move the on-tier youth reference (if any) to the front so the LLM
	// treats it as the primary structural exemplar. Off-tier references
	// stay visible — see HOF-002 (emphasize-but-show-all): hard-filtering
	// the intermediate tier would leave the model with a sample size of 1.
	if profile.Tier != nil {
		refProgs = sortReferencesByTier(refProgs, *profile.Tier)
	}
	ac.ReferencePrograms = refProgs

	return ac, nil
}

// tierMethodologyKey maps a youth tier to its default methodology key.
// Returns "" for tiers with no default mapping (adult / unmapped values).
func tierMethodologyKey(tier string) string {
	switch tier {
	case "foundational":
		return "yessis-1x20"
	case "intermediate":
		return "yessis-1x15"
	case "sport_performance":
		return "yessis-sport-performance"
	}
	return ""
}

// resolveMethodology returns the methodology (with links) bound to this
// generation, or nil if none is bound. Encodes the ADR 016 D1/D2 rules:
//
//   - explicit MethodologyID always wins (returns ErrNotFound if it
//     doesn't resolve);
//   - else if youth AND RequireMethodology: look up by tier-mapped key,
//     fail if not found — youth never generates rules-less;
//   - else (adult, or non-RequireMethodology path): return nil and let
//     the caller fall back to in-code defaults.
func resolveMethodology(ctx context.Context, db *sql.DB, profile *AthleteProfile, opts BuildContextOptions) (*models.MethodologyWithLinks, error) {
	if opts.MethodologyID != nil {
		m, err := models.LoadMethodologyWithLinks(ctx, db, *opts.MethodologyID)
		if err != nil {
			return nil, fmt.Errorf("llm: load methodology %d: %w", *opts.MethodologyID, err)
		}
		return m, nil
	}
	if !opts.RequireMethodology {
		return nil, nil
	}
	if profile.Tier == nil {
		// Adult — adult fallback to the in-code generic block is
		// intentional pre-Phase-3 (no UI selector for adults yet).
		return nil, nil
	}
	key := tierMethodologyKey(*profile.Tier)
	if key == "" {
		return nil, fmt.Errorf("llm: no methodology mapped for youth tier %q — configure a methodology for this athlete or fix tier mapping", *profile.Tier)
	}
	m, err := models.GetMethodologyByKey(ctx, db, key)
	if err != nil {
		return nil, fmt.Errorf("llm: load tier-default methodology %q for tier %q: %w", key, *profile.Tier, err)
	}
	return models.LoadMethodologyWithLinks(ctx, db, m.ID)
}

// projectMethodology returns the lean view of a methodology that ships in
// the marshalled AthleteContext. The full `definition` is intentionally
// omitted — it already lives on the generation row's `prompt` column.
func projectMethodology(m *models.MethodologyWithLinks) *MethodologyProjection {
	p := &MethodologyProjection{
		ID:                 m.ID,
		Key:                m.Key,
		Name:               m.Name,
		AllowedPatterns:    append([]string(nil), m.AllowedPatterns...),
		AllowedEquipmentN:  len(m.AllowedEquipmentIDs),
		AllowedExercisesN:  len(m.AllowedExerciseIDs),
		ReferenceProgramsN: len(m.ReferenceProgramIDs),
	}
	if m.Audience.Valid {
		p.Audience = m.Audience.String
	}
	if m.ApplicableTiers.Valid {
		p.ApplicableTiers = m.ApplicableTiers.String
	}
	if m.Philosophy.Valid {
		p.Philosophy = m.Philosophy.String
	}
	return p
}

// buildProfile constructs the athlete profile with computed summary stats.
func buildProfile(ctx context.Context, db *sql.DB, athleteID int64, now time.Time) (*AthleteProfile, error) {
	athlete, err := models.GetAthleteByID(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("get athlete %d: %w", athleteID, err)
	}

	profile := &AthleteProfile{
		Name:       athlete.Name,
		WeightUnit: models.GetDefaultWeightUnit(ctx, db),
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
	if athlete.DateOfBirth.Valid {
		if dob, err := parseDate(athlete.DateOfBirth.String); err == nil {
			age := int(now.Sub(dob).Hours() / 24 / 365)
			profile.Age = &age
		}
	}
	if athlete.Grade.Valid {
		profile.Grade = &athlete.Grade.String
	}
	if athlete.Gender.Valid {
		profile.Gender = &athlete.Gender.String
	}

	// Compute training months from earliest workout (single query, no paging).
	count, earliest, err := models.WorkoutStats(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("workout stats for profile: %w", err)
	}
	profile.TotalWorkouts = count

	if earliest != "" {
		if t, err := parseDate(earliest); err == nil {
			months := int(now.Sub(t).Hours() / 24 / 30)
			profile.TrainingMonths = months
		}
	}

	// Latest body weight.
	bw, err := models.LatestBodyWeight(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("latest body weight: %w", err)
	}
	if bw != nil {
		profile.LatestBW = &bw.Weight
	}

	return profile, nil
}

// buildEquipmentList returns the names of equipment the athlete has access to.
func buildEquipmentList(ctx context.Context, db *sql.DB, athleteID int64) ([]string, error) {
	items, err := models.ListAthleteEquipment(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.EquipmentName
	}
	return names, nil
}

// buildCurrentPrograms returns all of the athlete's active programs (primary + supplementals).
func buildCurrentPrograms(ctx context.Context, db *sql.DB, athleteID int64) ([]ProgramSummary, error) {
	programs, err := models.ListActiveProgramAssignments(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	if len(programs) == 0 {
		return nil, nil
	}
	result := make([]ProgramSummary, len(programs))
	for i, p := range programs {
		result[i] = ProgramSummary{
			Name:      p.TemplateName,
			Role:      p.Role,
			NumWeeks:  p.NumWeeks,
			NumDays:   p.NumDays,
			IsLoop:    p.IsLoop,
			StartDate: p.StartDate,
			Active:    p.Active,
		}
		if p.Schedule.Valid {
			result[i].Schedule = &p.Schedule.String
		}
	}
	return result, nil
}

// buildTrainingMaxes returns the athlete's current training maxes.
func buildTrainingMaxes(ctx context.Context, db *sql.DB, athleteID int64) ([]TMEntry, error) {
	tms, err := models.ListCurrentTrainingMaxes(ctx, db, athleteID)
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
func buildBodyWeights(ctx context.Context, db *sql.DB, athleteID int64) ([]BodyWeightEntry, error) {
	page, err := models.ListBodyWeights(ctx, db, athleteID, 0)
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

// buildPerformanceTrends computes per-exercise aggregate stats from recent workouts.
// This gives the LLM a quick view of volume and intensity trends without
// needing to parse every individual set.
func buildPerformanceTrends(workouts []WorkoutSummary) []ExercisePerformance {
	type accumulator struct {
		totalRPE  float64
		rpeCount  int
		maxWeight float64
		totalSets int
		totalReps int
	}
	byExercise := make(map[string]*accumulator)

	for _, w := range workouts {
		for _, s := range w.Sets {
			acc, exists := byExercise[s.Exercise]
			if !exists {
				acc = &accumulator{}
				byExercise[s.Exercise] = acc
			}
			acc.totalSets++
			acc.totalReps += s.Reps
			if s.RPE != nil {
				acc.totalRPE += *s.RPE
				acc.rpeCount++
			}
			if s.Weight != nil && *s.Weight > acc.maxWeight {
				acc.maxWeight = *s.Weight
			}
		}
	}

	trends := make([]ExercisePerformance, 0, len(byExercise))
	for name, acc := range byExercise {
		ep := ExercisePerformance{
			Exercise:  name,
			TotalSets: acc.totalSets,
			TotalReps: acc.totalReps,
		}
		if acc.rpeCount > 0 {
			avg := acc.totalRPE / float64(acc.rpeCount)
			ep.AvgRPE = &avg
		}
		if acc.maxWeight > 0 {
			ep.MaxWeight = &acc.maxWeight
		}
		trends = append(trends, ep)
	}
	return trends
}

// buildCoachNotes returns a combined view of coach notes and relevant journal entries.
func buildCoachNotes(ctx context.Context, db *sql.DB, athleteID int64) ([]NoteEntry, error) {
	// Athlete notes (coach observations, pinned items).
	notes, err := models.ListAthleteNotes(ctx, db, athleteID, true)
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
	journal, err := models.ListJournalEntries(ctx, db, athleteID, true, 50)
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

// buildGoals constructs the goal context from the athlete profile and goal history.
func buildGoals(ctx context.Context, db *sql.DB, profile *AthleteProfile, athleteID int64) GoalContext {
	gc := GoalContext{}
	if profile.Goal != nil {
		gc.Current = *profile.Goal
	}

	// Populate goal history from the goal_history table.
	history, err := models.ListGoalHistory(ctx, db, athleteID)
	if err == nil && len(history) > 0 {
		for _, h := range history {
			gc.History = append(gc.History, h.Goal)
		}
	}

	return gc
}

// buildExerciseCatalog returns all exercises annotated with equipment compatibility.
// When methodology is non-nil (ADR 016 Phase 2), the catalog is also scoped to the
// methodology's allow-list:
//
//   - PATTERNS: an exercise is in scope iff it has ≥1 movement-pattern tag AND
//     all of its tags are members of methodology.AllowedPatterns. Untagged
//     exercises do NOT enter via the pattern path (conservative for youth —
//     untagged conditioning/mobility shouldn't sneak into a Yessis program).
//   - EXPLICIT-LIST OVERRIDE: an exercise is also in scope if its id is in
//     methodology.AllowedExerciseIDs (the bespoke/explicit allow-list, e.g.
//     5/3/1's barbell mains).
//   - EQUIPMENT GATE: regardless of how it qualified above, an exercise is
//     DROPPED if any of its REQUIRED equipment (exercise_equipment with
//     optional=0) is NOT in methodology.AllowedEquipmentIDs. Optional=1
//     equipment is informational — it does NOT trigger a drop. This is the
//     structural "a foundational athlete who owns a barbell still isn't
//     offered one" guarantee.
//
// The athlete-side equipment-compatibility flag is preserved on every surviving
// entry; the methodology scope is structural, the compat flag is athlete-specific.
func buildExerciseCatalog(ctx context.Context, db *sql.DB, athleteID int64, methodology *models.MethodologyWithLinks) ([]ExerciseEntry, error) {
	exercises, err := models.ListExercises(ctx, db, "")
	if err != nil {
		return nil, err
	}

	// Batch compatibility check (single query instead of N per-exercise calls).
	compatMap, err := models.BatchCheckExerciseCompatibility(ctx, db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("batch exercise compatibility: %w", err)
	}

	// Methodology scoping prep — build the lookup sets once.
	var (
		scopeActive             bool
		allowedPatterns         map[string]struct{}
		allowedExercises        map[int64]struct{}
		allowedEquipment        map[int64]struct{}
		patternsByExercise      map[int64][]string
		requiredEquipByExercise map[int64][]int64
	)
	if methodology != nil {
		scopeActive = true
		allowedPatterns = sliceToSet(methodology.AllowedPatterns)
		allowedExercises = int64SliceToSet(methodology.AllowedExerciseIDs)
		allowedEquipment = int64SliceToSet(methodology.AllowedEquipmentIDs)

		patternsByExercise, err = batchExerciseMovementPatterns(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("batch exercise movement patterns: %w", err)
		}
		requiredEquipByExercise, err = batchRequiredExerciseEquipment(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("batch required exercise equipment: %w", err)
		}
	}

	entries := make([]ExerciseEntry, 0, len(exercises))
	for _, ex := range exercises {
		if scopeActive && !exerciseInMethodologyScope(ex.ID, patternsByExercise[ex.ID], requiredEquipByExercise[ex.ID], allowedPatterns, allowedExercises, allowedEquipment) {
			continue
		}
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

		entry.Compatible = compatMap[ex.ID]

		entries = append(entries, entry)
	}
	return entries, nil
}

// exerciseInMethodologyScope is the per-exercise admit/drop decision used by
// buildExerciseCatalog when a methodology is bound. Encapsulated so it's
// directly testable. See buildExerciseCatalog's doc comment for the rules.
func exerciseInMethodologyScope(
	exerciseID int64,
	exercisePatterns []string,
	requiredEquipment []int64,
	allowedPatterns map[string]struct{},
	allowedExercises map[int64]struct{},
	allowedEquipment map[int64]struct{},
) bool {
	// 1. Explicit-list override OR pattern admission.
	admitted := false
	if _, ok := allowedExercises[exerciseID]; ok {
		admitted = true
	} else if len(exercisePatterns) > 0 {
		admitted = true
		for _, p := range exercisePatterns {
			if _, ok := allowedPatterns[p]; !ok {
				admitted = false
				break
			}
		}
	}
	if !admitted {
		return false
	}

	// 2. Equipment gate — drop if any REQUIRED equipment is not allowed.
	for _, eqID := range requiredEquipment {
		if _, ok := allowedEquipment[eqID]; !ok {
			return false
		}
	}
	return true
}

// batchExerciseMovementPatterns returns exercise_id → []pattern for all
// tagged exercises. Single query instead of N per-exercise calls.
func batchExerciseMovementPatterns(ctx context.Context, db *sql.DB) (map[int64][]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT exercise_id, pattern FROM exercise_movement_patterns ORDER BY exercise_id, pattern`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var id int64
		var pattern string
		if err := rows.Scan(&id, &pattern); err != nil {
			return nil, err
		}
		out[id] = append(out[id], pattern)
	}
	return out, rows.Err()
}

// batchRequiredExerciseEquipment returns exercise_id → []equipment_id for
// REQUIRED equipment links (optional=0). Optional equipment is intentionally
// excluded — see buildExerciseCatalog's equipment gate.
func batchRequiredExerciseEquipment(ctx context.Context, db *sql.DB) (map[int64][]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT exercise_id, equipment_id FROM exercise_equipment WHERE optional = 0 ORDER BY exercise_id, equipment_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]int64{}
	for rows.Next() {
		var exID, eqID int64
		if err := rows.Scan(&exID, &eqID); err != nil {
			return nil, err
		}
		out[exID] = append(out[exID], eqID)
	}
	return out, rows.Err()
}

func sliceToSet(s []string) map[string]struct{} {
	out := make(map[string]struct{}, len(s))
	for _, v := range s {
		out[v] = struct{}{}
	}
	return out
}

func int64SliceToSet(s []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(s))
	for _, v := range s {
		out[v] = struct{}{}
	}
	return out
}

// buildRecentWorkouts returns the athlete's most recent workouts with their sets.
// Returns up to 20 workouts.
func buildRecentWorkouts(ctx context.Context, db *sql.DB, athleteID int64) ([]WorkoutSummary, error) {
	page, err := models.ListWorkouts(ctx, db, athleteID, 0)
	if err != nil {
		return nil, err
	}

	// Take up to 20 most recent.
	workouts := page.Workouts
	if len(workouts) > 20 {
		workouts = workouts[:20]
	}

	// Batch-load all sets for the selected workouts (single query).
	workoutIDs := make([]int64, len(workouts))
	for i, w := range workouts {
		workoutIDs[i] = w.ID
	}
	allSets, err := models.ListSetsByWorkoutIDs(ctx, db, workoutIDs)
	if err != nil {
		return nil, fmt.Errorf("batch list sets: %w", err)
	}

	summaries := make([]WorkoutSummary, 0, len(workouts))
	for _, w := range workouts {
		ws := WorkoutSummary{
			Date: normalizeDate(w.Date),
		}
		if w.Notes.Valid {
			ws.Notes = &w.Notes.String
		}

		// Use batch-loaded sets for this workout.
		groups := allSets[w.ID]
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

// buildRecentActivity assembles the ADR-018 cross-discipline rollup. It is
// best-effort: any sub-query failure is swallowed (the field is simply omitted)
// because none of this data is required for a valid CatalogJSON — it only
// improves programming judgment and youth safety.
func buildRecentActivity(ctx context.Context, db *sql.DB, athleteID int64, now time.Time) RecentActivity {
	var ra RecentActivity
	cutoff := now.AddDate(0, 0, -14).Format("2006-01-02")
	today := now.Format("2006-01-02")

	// Current season phase: the phase spanning today.
	if phases, err := models.ListSeasonPhases(ctx, db, athleteID); err == nil {
		for _, sp := range phases {
			endsOK := !sp.EndDate.Valid || sp.EndDate.String >= today
			if sp.StartDate <= today && endsOK {
				label := sp.Phase
				if sp.Sport.Valid && sp.Sport.String != "" {
					label = sp.Sport.String + " " + sp.Phase
				}
				ra.SeasonPhase = &label
				break
			}
		}
	}

	// Most recent recovery check-in.
	if checkins, err := models.ListRecoveryCheckins(ctx, db, athleteID, 1); err == nil && len(checkins) > 0 {
		c := checkins[0]
		snap := &RecoverySnapshot{Date: normalizeDate(c.Date)}
		if c.SleepHours.Valid {
			snap.SleepHours = &c.SleepHours.Float64
		}
		if c.Soreness.Valid {
			snap.Soreness = &c.Soreness.Int64
		}
		if c.Energy.Valid {
			snap.Energy = &c.Energy.Int64
		}
		if c.Notes.Valid {
			snap.Notes = c.Notes.String
		}
		ra.LatestRecovery = snap
	}

	// Throwing volume over the last 14 days.
	if sessions, err := models.ListThrowingSessions(ctx, db, athleteID, 100); err == nil {
		count, throws := 0, int64(0)
		for _, s := range sessions {
			if s.Date >= cutoff {
				count++
				if s.ThrowCount.Valid {
					throws += s.ThrowCount.Int64
				}
			}
		}
		if count > 0 {
			ra.Throwing14d = &DisciplineVolume{Sessions: count, Detail: fmt.Sprintf("%d throws", throws)}
		}
	}

	// Conditioning volume over the last 14 days.
	if sessions, err := models.ListConditioningSessions(ctx, db, athleteID, 100); err == nil {
		count, secs := 0, int64(0)
		for _, s := range sessions {
			if s.Date >= cutoff {
				count++
				if s.DurationSeconds.Valid {
					secs += s.DurationSeconds.Int64
				}
			}
		}
		if count > 0 {
			ra.Conditioning14d = &DisciplineVolume{Sessions: count, Detail: fmt.Sprintf("%ds", secs)}
		}
	}

	// Load advisories: surface any discipline whose ACWR is elevated (>1.3) or
	// very high (>1.5), the conventional injury-risk thresholds.
	if ls, err := models.GetLoadSummary(ctx, db, athleteID); err == nil && ls != nil {
		for _, d := range ls.Disciplines {
			if d.ACWR == nil {
				continue
			}
			switch {
			case *d.ACWR > 1.5:
				ra.LoadAdvisory = append(ra.LoadAdvisory, fmt.Sprintf("%s ACWR %.2f — high, reduce load", d.Discipline, *d.ACWR))
			case *d.ACWR > 1.3:
				ra.LoadAdvisory = append(ra.LoadAdvisory, fmt.Sprintf("%s ACWR %.2f — elevated", d.Discipline, *d.ACWR))
			}
		}
	}

	return ra
}

// buildProgramHistory returns all program assignments for the athlete,
// ordered most recent first. Includes start date, notes, goals, and active status.
func buildProgramHistory(ctx context.Context, db *sql.DB, athleteID int64) ([]ProgramHistoryEntry, error) {
	programs, err := models.ListAthletePrograms(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	entries := make([]ProgramHistoryEntry, len(programs))
	for i, p := range programs {
		entries[i] = ProgramHistoryEntry{
			Name:      p.TemplateName,
			Role:      p.Role,
			NumWeeks:  p.NumWeeks,
			NumDays:   p.NumDays,
			IsLoop:    p.IsLoop,
			StartDate: p.StartDate,
			Active:    p.Active,
		}
		if p.Schedule.Valid {
			entries[i].Schedule = &p.Schedule.String
		}
		if p.Notes.Valid {
			entries[i].Notes = &p.Notes.String
		}
		if p.Goal.Valid {
			entries[i].Goal = &p.Goal.String
		}
	}
	return entries, nil
}

// buildPriorTemplates returns athlete-scoped program templates (previously
// generated for this athlete) as lightweight metadata summaries.
// Global reference programs are handled separately by buildReferencePrograms.
func buildPriorTemplates(ctx context.Context, db *sql.DB, athleteID int64) ([]TemplateSummary, error) {
	templates, err := models.ListProgramTemplatesForAthlete(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	var summaries []TemplateSummary
	for _, t := range templates {
		// Skip global templates — they're included in reference_programs with full sets.
		if t.AthleteID == nil {
			continue
		}
		summaries = append(summaries, TemplateSummary{
			Name:     t.Name,
			NumWeeks: t.NumWeeks,
			NumDays:  t.NumDays,
			IsLoop:   t.IsLoop,
		})
	}
	return summaries, nil
}

// buildReferencePrograms returns global seed/reference programs filtered by audience
// ("youth" or "adult") with their full prescribed sets. This gives the LLM concrete
// structural examples of correctly-built programs for the athlete's audience.
func buildReferencePrograms(ctx context.Context, db *sql.DB, audience string) ([]ReferenceProgramSummary, error) {
	templates, err := models.ListReferenceTemplatesByAudience(ctx, db, audience)
	if err != nil {
		return nil, err
	}
	return templatesToReferenceSummaries(ctx, db, templates)
}

// buildReferenceProgramsByIDs loads specific program templates by their IDs
// with full prescribed sets. Used when the coach explicitly selects which
// reference programs to provide to the LLM.
func buildReferenceProgramsByIDs(ctx context.Context, db *sql.DB, ids []int64) ([]ReferenceProgramSummary, error) {
	templates, err := models.ListProgramTemplatesByIDs(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	return templatesToReferenceSummaries(ctx, db, templates)
}

// templatesToReferenceSummaries converts a slice of program templates into
// ReferenceProgramSummary values, loading full prescribed sets for each.
func templatesToReferenceSummaries(ctx context.Context, db *sql.DB, templates []*models.ProgramTemplate) ([]ReferenceProgramSummary, error) {
	var programs []ReferenceProgramSummary
	for _, t := range templates {
		rp := ReferenceProgramSummary{
			Name:     t.Name,
			NumWeeks: t.NumWeeks,
			NumDays:  t.NumDays,
			IsLoop:   t.IsLoop,
			Phase:    phaseForReferenceProgram(t.Name),
		}
		if t.Description.Valid {
			rp.Description = t.Description.String
		}
		if t.Audience.Valid {
			rp.Audience = t.Audience.String
		}

		sets, err := models.ListPrescribedSets(ctx, db, t.ID)
		if err != nil {
			return nil, fmt.Errorf("list prescribed sets for template %d: %w", t.ID, err)
		}
		for _, ps := range sets {
			pss := PrescribedSetSummary{
				Exercise:  ps.ExerciseName,
				Week:      ps.Week,
				Day:       ps.Day,
				SetNumber: ps.SetNumber,
				RepType:   ps.RepType,
				SortOrder: ps.SortOrder,
			}
			if ps.Reps.Valid {
				r := int(ps.Reps.Int64)
				pss.Reps = &r
			}
			if ps.Percentage.Valid {
				p := ps.Percentage.Float64
				pss.Percentage = &p
			}
			if ps.AbsoluteWeight.Valid {
				w := ps.AbsoluteWeight.Float64
				pss.AbsoluteWeight = &w
			}
			if ps.RestSeconds.Valid {
				rest := int(ps.RestSeconds.Int64)
				pss.RestSeconds = &rest
			}
			if ps.Notes.Valid {
				pss.Notes = ps.Notes.String
			}
			rp.PrescribedSets = append(rp.PrescribedSets, pss)
		}

		programs = append(programs, rp)
	}
	return programs, nil
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

// phaseForReferenceProgram returns the youth-tier phase a seeded reference
// program represents, based on its name. Returns "" for programs we don't
// recognise as on-phase (adult programs, future seeded programs). The mapping
// matches the Sarge/Yessis source: 1×20 = foundational, 1×15 = intermediate,
// Sport Performance Months = sport_performance. See HOF-002 + docs/seed-catalog.md.
func phaseForReferenceProgram(name string) string {
	switch {
	case strings.Contains(name, "1×20"):
		return "foundational"
	case strings.Contains(name, "1×15"):
		return "intermediate"
	case strings.HasPrefix(name, "Sport Performance"):
		return "sport_performance"
	default:
		return ""
	}
}

// sortReferencesByTier returns a copy of refs with the on-tier reference (if
// present) moved to the front. Order of off-tier references is preserved.
// This is a stable partition — used so the LLM treats the on-tier program as
// the primary structural exemplar without losing visibility into the adjacent
// phases (sample-size argument; see HOF-002 DISCUSSION).
func sortReferencesByTier(refs []ReferenceProgramSummary, tier string) []ReferenceProgramSummary {
	if len(refs) == 0 || tier == "" {
		return refs
	}
	out := make([]ReferenceProgramSummary, 0, len(refs))
	for _, r := range refs {
		if r.Phase == tier {
			out = append(out, r)
		}
	}
	for _, r := range refs {
		if r.Phase != tier {
			out = append(out, r)
		}
	}
	return out
}

// normalizeDate extracts the YYYY-MM-DD portion from a date string.
func normalizeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
