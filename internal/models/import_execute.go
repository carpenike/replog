package models

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/importers"
)

// BuildImportPreview generates a preview of what an import will do without
// making any changes. The mapping must have all entities resolved (MappedID > 0
// or Create = true).
func BuildImportPreview(db *sql.DB, athleteID int64, ms *importers.MappingState) (*ImportPreview, error) {
	pf := ms.Parsed
	p := &ImportPreview{}

	// Count exercises.
	for _, m := range ms.Exercises {
		if m.Create {
			p.ExercisesNew++
		} else {
			p.ExercisesMapped++
		}
	}

	// Count equipment (RepLog JSON only).
	for _, m := range ms.Equipment {
		if m.Create {
			p.EquipmentNew++
		} else {
			p.EquipmentMapped++
		}
	}

	// Count programs (RepLog JSON only).
	for _, m := range ms.Programs {
		if m.Create {
			p.ProgramsNew++
		} else {
			p.ProgramsMapped++
		}
	}

	// Count sets and check for date conflicts.
	var minDate, maxDate string
	for _, w := range pf.Workouts {
		p.WorkoutCount++
		p.SetCount += len(w.Sets)

		date := normalizeDate(w.Date)
		if minDate == "" || date < minDate {
			minDate = date
		}
		if maxDate == "" || date > maxDate {
			maxDate = date
		}

		// Check for existing workout on this date.
		_, err := GetWorkoutByAthleteDate(db, athleteID, date)
		if err == nil {
			p.ConflictDates = append(p.ConflictDates, date)
		}
	}
	if minDate != "" && maxDate != "" {
		p.DateRange = minDate + " to " + maxDate
	}

	// Count reviews.
	for _, w := range pf.Workouts {
		if w.Review != nil {
			p.ReviewCount++
		}
	}

	p.AssignmentCount = len(pf.Assignments)
	p.TrainingMaxCount = len(pf.TrainingMaxes)
	p.BodyWeightCount = len(pf.BodyWeights)

	// Computed aggregate counts for template display.
	p.ExerciseCount = p.ExercisesNew + p.ExercisesMapped
	p.EquipmentCount = p.EquipmentNew + p.EquipmentMapped
	p.ProgramCount = p.ProgramsNew + p.ProgramsMapped

	// Validate data quality.
	p.Warnings = validateImportData(pf)

	return p, nil
}

// validRepTypes are the allowed values for rep_type.
var validRepTypes = map[string]bool{
	"reps":      true,
	"seconds":   true,
	"each_side": true,
}

// validateImportData checks parsed data for quality issues and returns warnings.
// Warnings do not prevent import but are shown in the preview for user review.
func validateImportData(pf *importers.ParsedFile) []ValidationWarning {
	var warnings []ValidationWarning
	today := time.Now().Format("2006-01-02")

	for _, w := range pf.Workouts {
		date := normalizeDate(w.Date)
		if date > today {
			warnings = append(warnings, ValidationWarning{
				Entity:  "workout",
				Field:   "date",
				Message: fmt.Sprintf("Workout on %s is in the future", date),
			})
		}

		for _, s := range w.Sets {
			if s.Weight != nil && *s.Weight < 0 {
				warnings = append(warnings, ValidationWarning{
					Entity:  "set",
					Field:   "weight",
					Message: fmt.Sprintf("Negative weight (%.1f) for %s on %s", *s.Weight, s.Exercise, date),
				})
			}
			if s.Reps < 0 {
				warnings = append(warnings, ValidationWarning{
					Entity:  "set",
					Field:   "reps",
					Message: fmt.Sprintf("Negative reps (%d) for %s on %s", s.Reps, s.Exercise, date),
				})
			}
			if s.RPE != nil && (*s.RPE < 0 || *s.RPE > 10) {
				warnings = append(warnings, ValidationWarning{
					Entity:  "set",
					Field:   "rpe",
					Message: fmt.Sprintf("RPE %.1f out of range (0-10) for %s on %s", *s.RPE, s.Exercise, date),
				})
			}
			if s.RepType != "" && !validRepTypes[s.RepType] {
				warnings = append(warnings, ValidationWarning{
					Entity:  "set",
					Field:   "rep_type",
					Message: fmt.Sprintf("Unknown rep type %q for %s on %s", s.RepType, s.Exercise, date),
				})
			}
		}
	}

	for _, tm := range pf.TrainingMaxes {
		if tm.Weight < 0 {
			warnings = append(warnings, ValidationWarning{
				Entity:  "training_max",
				Field:   "weight",
				Message: fmt.Sprintf("Negative training max (%.1f) for %s", tm.Weight, tm.Exercise),
			})
		}
	}

	for _, bw := range pf.BodyWeights {
		if bw.Weight <= 0 {
			warnings = append(warnings, ValidationWarning{
				Entity:  "body_weight",
				Field:   "weight",
				Message: fmt.Sprintf("Invalid body weight (%.1f) on %s", bw.Weight, bw.Date),
			})
		}
	}

	return warnings
}

// ExecuteImport performs the import in a single transaction. It creates new
// entities as specified by the mapping, then imports all data.
func ExecuteImport(db *sql.DB, athleteID, coachID int64, ms *importers.MappingState) (*ImportResult, error) {
	pf := ms.Parsed
	result := &ImportResult{}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin import tx: %w", err)
	}
	defer tx.Rollback()

	// Phase 1: Create new equipment (RepLog JSON only).
	equipmentIDMap := make(map[string]int64)
	for _, m := range ms.Equipment {
		if m.MappedID > 0 {
			equipmentIDMap[strings.ToLower(m.ImportName)] = m.MappedID
		} else if m.Create {
			desc := ""
			for _, pe := range pf.Equipment {
				if strings.EqualFold(pe.Name, m.ImportName) && pe.Description != nil {
					desc = *pe.Description
					break
				}
			}
			id, err := insertEquipment(tx, m.ImportName, desc)
			if err != nil {
				return nil, fmt.Errorf("models: import create equipment %q: %w", m.ImportName, err)
			}
			equipmentIDMap[strings.ToLower(m.ImportName)] = id
			result.EquipmentCreated++
		}
	}

	// Phase 2: Create new exercises.
	exerciseIDMap := make(map[string]int64)
	for _, m := range ms.Exercises {
		if m.MappedID > 0 {
			exerciseIDMap[strings.ToLower(m.ImportName)] = m.MappedID
		} else if m.Create {
			pe := findParsedExercise(pf.Exercises, m.ImportName)
			tier := ""
			formNotes := ""
			demoURL := ""
			restSeconds := 0
			featured := false
			if pe != nil {
				if pe.Tier != nil {
					tier = *pe.Tier
				}
				if pe.FormNotes != nil {
					formNotes = *pe.FormNotes
				}
				if pe.DemoURL != nil {
					demoURL = *pe.DemoURL
				}
				if pe.RestSeconds != nil {
					restSeconds = *pe.RestSeconds
				}
				featured = pe.Featured
			}
			id, err := insertExercise(tx, m.ImportName, tier, formNotes, demoURL, restSeconds, featured)
			if err != nil {
				return nil, fmt.Errorf("models: import create exercise %q: %w", m.ImportName, err)
			}
			exerciseIDMap[strings.ToLower(m.ImportName)] = id
			result.ExercisesCreated++

			// Wire up equipment dependencies if this is a RepLog JSON import.
			if pe != nil {
				for _, eq := range pe.Equipment {
					eqID, ok := equipmentIDMap[strings.ToLower(eq.Name)]
					if ok {
						if err := insertExerciseEquipment(tx, id, eqID, eq.Optional); err != nil {
							return nil, fmt.Errorf("models: import exercise equipment link: %w", err)
						}
					}
				}
			}
		}
	}

	// Phase 3: Athlete equipment inventory (RepLog JSON only).
	for _, eqName := range pf.AthleteEquipment {
		eqID, ok := equipmentIDMap[strings.ToLower(eqName)]
		if ok {
			if err := insertAthleteEquipment(tx, athleteID, eqID); err != nil {
				log.Printf("models: insert athlete equipment (athlete=%d, eq=%d): %v", athleteID, eqID, err)
			}
		}
	}

	// Phase 4: Assignments (RepLog JSON only).
	for _, a := range pf.Assignments {
		exID, ok := exerciseIDMap[strings.ToLower(a.Exercise)]
		if !ok {
			continue
		}
		targetReps := 0
		if a.TargetReps != nil {
			targetReps = *a.TargetReps
		}
		if err := insertAssignment(tx, athleteID, exID, targetReps, a.Active); err != nil {
			// Skip duplicates silently.
			if !isUniqueViolation(err) {
				return nil, fmt.Errorf("models: import assignment for %q: %w", a.Exercise, err)
			}
			result.AssignmentsSkipped++
			continue
		}
		result.AssignmentsCreated++
	}

	// Phase 5: Training maxes (RepLog JSON only).
	for _, tm := range pf.TrainingMaxes {
		exID, ok := exerciseIDMap[strings.ToLower(tm.Exercise)]
		if !ok {
			continue
		}
		date := normalizeDate(tm.EffectiveDate)
		notes := ""
		if tm.Notes != nil {
			notes = *tm.Notes
		}
		if err := insertTrainingMax(tx, athleteID, exID, tm.Weight, date, notes); err != nil {
			// Skip duplicates.
			if !isUniqueViolation(err) {
				return nil, fmt.Errorf("models: import training max: %w", err)
			}
			result.TrainingMaxesSkipped++
			continue
		}
		result.TrainingMaxesCreated++
	}

	// Phase 6: Body weights (RepLog JSON only).
	for _, bw := range pf.BodyWeights {
		date := normalizeDate(bw.Date)
		notes := ""
		if bw.Notes != nil {
			notes = *bw.Notes
		}
		if err := insertBodyWeight(tx, athleteID, date, bw.Weight, notes); err != nil {
			if !isUniqueViolation(err) {
				return nil, fmt.Errorf("models: import body weight: %w", err)
			}
			result.BodyWeightsSkipped++
			continue
		}
		result.BodyWeightsCreated++
	}

	// Phase 7: Workouts + sets.
	for _, w := range pf.Workouts {
		date := normalizeDate(w.Date)

		// Skip if workout already exists on this date.
		_, err := getWorkoutByAthleteDateTx(tx, athleteID, date)
		if err == nil {
			result.WorkoutsSkipped++
			continue
		}

		notes := ""
		if w.Notes != nil {
			notes = *w.Notes
		}

		workoutID, err := insertWorkout(tx, athleteID, date, notes)
		if err != nil {
			if isUniqueViolation(err) {
				result.WorkoutsSkipped++
				continue
			}
			return nil, fmt.Errorf("models: import workout on %s: %w", date, err)
		}
		result.WorkoutsCreated++

		// Sets.
		for _, s := range w.Sets {
			exID, ok := exerciseIDMap[strings.ToLower(s.Exercise)]
			if !ok {
				continue
			}
			weight := 0.0
			if s.Weight != nil {
				weight = *s.Weight
			}
			rpe := 0.0
			if s.RPE != nil {
				rpe = *s.RPE
			}
			sNotes := ""
			if s.Notes != nil {
				sNotes = *s.Notes
			}
			repType := s.RepType
			if repType == "" {
				repType = "reps"
			}
			if err := insertSet(tx, workoutID, exID, s.SetNumber, s.Reps, weight, rpe, repType, sNotes); err != nil {
				return nil, fmt.Errorf("models: import set: %w", err)
			}
			result.SetsCreated++
		}

		// Review (RepLog JSON only).
		if w.Review != nil && coachID > 0 {
			rNotes := ""
			if w.Review.Notes != nil {
				rNotes = *w.Review.Notes
			}
			if err := insertReview(tx, workoutID, coachID, w.Review.Status, rNotes); err != nil {
				return nil, fmt.Errorf("models: import review: %w", err)
			}
			result.ReviewsCreated++
		}
	}

	// Phase 8: Programs (RepLog JSON only).
	for _, prog := range pf.Programs {
		var templateID int64
		var ok bool

		// Look up program in mapping state.
		for _, m := range ms.Programs {
			if strings.EqualFold(m.ImportName, prog.Template.Name) {
				if m.MappedID > 0 {
					templateID = m.MappedID
					ok = true
				} else if m.Create {
					// Create the program template.
					id, err := insertProgramTemplate(tx, prog.Template, nil)
					if err != nil {
						return nil, fmt.Errorf("models: import program template %q: %w", prog.Template.Name, err)
					}
					templateID = id
					ok = true

					// Prescribed sets.
					for _, ps := range prog.Template.PrescribedSets {
						exID, exOK := exerciseIDMap[strings.ToLower(ps.Exercise)]
						if !exOK {
							continue
						}
						if err := insertPrescribedSet(tx, templateID, exID, ps); err != nil {
							return nil, fmt.Errorf("models: import prescribed set: %w", err)
						}
					}

					// Progression rules.
					for _, pr := range prog.Template.ProgressionRules {
						exID, exOK := exerciseIDMap[strings.ToLower(pr.Exercise)]
						if !exOK {
							continue
						}
						if err := insertProgressionRule(tx, templateID, exID, pr.Increment); err != nil {
							return nil, fmt.Errorf("models: import progression rule: %w", err)
						}
					}
				}
				break
			}
		}

		if !ok || templateID == 0 {
			continue
		}

		notes := ""
		if prog.Notes != nil {
			notes = *prog.Notes
		}
		goal := ""
		if prog.Goal != nil {
			goal = *prog.Goal
		}
		startDate := normalizeDate(prog.StartDate)
		if err := insertAthleteProgram(tx, athleteID, templateID, startDate, notes, goal, prog.Active); err != nil {
			if !isUniqueViolation(err) {
				return nil, fmt.Errorf("models: import athlete program: %w", err)
			}
			result.ProgramsSkipped++
		} else {
			result.ProgramsCreated++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit import: %w", err)
	}

	return result, nil
}

// --- Transaction-level insert helpers ---

func insertEquipment(tx *sql.Tx, name, description string) (int64, error) {
	var descVal sql.NullString
	if description != "" {
		descVal = sql.NullString{String: description, Valid: true}
	}
	var id int64
	err := tx.QueryRow(`INSERT INTO equipment (name, description) VALUES (?, ?) RETURNING id`, name, descVal).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func insertExercise(tx *sql.Tx, name, tier, formNotes, demoURL string, restSeconds int, featured bool) (int64, error) {
	var tierVal, notesVal, demoVal sql.NullString
	var restVal sql.NullInt64
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	if formNotes != "" {
		notesVal = sql.NullString{String: formNotes, Valid: true}
	}
	if demoURL != "" {
		demoVal = sql.NullString{String: demoURL, Valid: true}
	}
	if restSeconds > 0 {
		restVal = sql.NullInt64{Int64: int64(restSeconds), Valid: true}
	}
	featuredInt := 0
	if featured {
		featuredInt = 1
	}
	var id int64
	err := tx.QueryRow(
		`INSERT INTO exercises (name, tier, form_notes, demo_url, rest_seconds, featured) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		name, tierVal, notesVal, demoVal, restVal, featuredInt,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func insertExerciseEquipment(tx *sql.Tx, exerciseID, equipmentID int64, optional bool) error {
	_, err := tx.Exec(
		`INSERT OR IGNORE INTO exercise_equipment (exercise_id, equipment_id, optional) VALUES (?, ?, ?)`,
		exerciseID, equipmentID, optional,
	)
	return err
}

func insertAthleteEquipment(tx *sql.Tx, athleteID, equipmentID int64) error {
	_, err := tx.Exec(
		`INSERT OR IGNORE INTO athlete_equipment (athlete_id, equipment_id) VALUES (?, ?)`,
		athleteID, equipmentID,
	)
	return err
}

func insertAssignment(tx *sql.Tx, athleteID, exerciseID int64, targetReps int, active bool) error {
	var trVal sql.NullInt64
	if targetReps > 0 {
		trVal = sql.NullInt64{Int64: int64(targetReps), Valid: true}
	}
	_, err := tx.Exec(
		`INSERT INTO athlete_exercises (athlete_id, exercise_id, target_reps, active) VALUES (?, ?, ?, ?)`,
		athleteID, exerciseID, trVal, active,
	)
	return err
}

func insertTrainingMax(tx *sql.Tx, athleteID, exerciseID int64, weight float64, date, notes string) error {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	_, err := tx.Exec(
		`INSERT INTO training_maxes (athlete_id, exercise_id, weight, effective_date, notes) VALUES (?, ?, ?, ?, ?)`,
		athleteID, exerciseID, weight, date, notesVal,
	)
	return err
}

func insertBodyWeight(tx *sql.Tx, athleteID int64, date string, weight float64, notes string) error {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	_, err := tx.Exec(
		`INSERT INTO body_weights (athlete_id, date, weight, notes) VALUES (?, ?, ?, ?)`,
		athleteID, date, weight, notesVal,
	)
	return err
}

func insertWorkout(tx *sql.Tx, athleteID int64, date, notes string) (int64, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var id int64
	err := tx.QueryRow(
		`INSERT INTO workouts (athlete_id, date, notes) VALUES (?, ?, ?) RETURNING id`,
		athleteID, date, notesVal,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func getWorkoutByAthleteDateTx(tx *sql.Tx, athleteID int64, date string) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM workouts WHERE athlete_id = ? AND date = ?`, athleteID, date).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func insertSet(tx *sql.Tx, workoutID, exerciseID int64, setNumber, reps int, weight, rpe float64, repType, notes string) error {
	var weightVal sql.NullFloat64
	if weight > 0 {
		weightVal = sql.NullFloat64{Float64: weight, Valid: true}
	}
	var rpeVal sql.NullFloat64
	if rpe > 0 {
		rpeVal = sql.NullFloat64{Float64: rpe, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	_, err := tx.Exec(
		`INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps, weight, rpe, rep_type, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		workoutID, exerciseID, setNumber, reps, weightVal, rpeVal, repType, notesVal,
	)
	return err
}

func insertReview(tx *sql.Tx, workoutID, coachID int64, status, notes string) error {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	_, err := tx.Exec(
		`INSERT INTO workout_reviews (workout_id, coach_id, status, notes) VALUES (?, ?, ?, ?)`,
		workoutID, coachID, status, notesVal,
	)
	return err
}

func insertProgramTemplate(tx *sql.Tx, pt importers.ParsedProgramTemplate, athleteID *int64) (int64, error) {
	var descVal sql.NullString
	if pt.Description != nil && *pt.Description != "" {
		descVal = sql.NullString{String: *pt.Description, Valid: true}
	}
	isLoopInt := 0
	if pt.IsLoop {
		isLoopInt = 1
	}
	var audVal sql.NullString
	if pt.Audience != nil && *pt.Audience != "" {
		audVal = sql.NullString{String: *pt.Audience, Valid: true}
	}
	var id int64
	err := tx.QueryRow(
		`INSERT INTO program_templates (athlete_id, name, description, num_weeks, num_days, is_loop, audience) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, pt.Name, descVal, pt.NumWeeks, pt.NumDays, isLoopInt, audVal,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func insertPrescribedSet(tx *sql.Tx, templateID, exerciseID int64, ps importers.ParsedPrescribedSet) error {
	var repsVal sql.NullInt64
	if ps.Reps != nil {
		repsVal = sql.NullInt64{Int64: int64(*ps.Reps), Valid: true}
	}
	var pctVal sql.NullFloat64
	if ps.Percentage != nil {
		pctVal = sql.NullFloat64{Float64: *ps.Percentage, Valid: true}
	}
	var absWeightVal sql.NullFloat64
	if ps.AbsoluteWeight != nil {
		absWeightVal = sql.NullFloat64{Float64: *ps.AbsoluteWeight, Valid: true}
	}
	var notesVal sql.NullString
	if ps.Notes != nil && *ps.Notes != "" {
		notesVal = sql.NullString{String: *ps.Notes, Valid: true}
	}
	repType := ps.RepType
	if repType == "" {
		repType = "reps"
	}
	_, err := tx.Exec(
		`INSERT INTO prescribed_sets (template_id, exercise_id, week, day, set_number, reps, percentage, absolute_weight, sort_order, rep_type, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		templateID, exerciseID, ps.Week, ps.Day, ps.SetNumber, repsVal, pctVal, absWeightVal, ps.SortOrder, repType, notesVal,
	)
	return err
}

func insertProgressionRule(tx *sql.Tx, templateID, exerciseID int64, increment float64) error {
	_, err := tx.Exec(
		`INSERT OR REPLACE INTO progression_rules (template_id, exercise_id, increment) VALUES (?, ?, ?)`,
		templateID, exerciseID, increment,
	)
	return err
}

func insertAthleteProgram(tx *sql.Tx, athleteID, templateID int64, startDate, notes, goal string, active bool) error {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var goalVal sql.NullString
	if goal != "" {
		goalVal = sql.NullString{String: goal, Valid: true}
	}
	_, err := tx.Exec(
		`INSERT INTO athlete_programs (athlete_id, template_id, start_date, active, notes, goal) VALUES (?, ?, ?, ?, ?, ?)`,
		athleteID, templateID, startDate, active, notesVal, goalVal,
	)
	return err
}

func findParsedExercise(exercises []importers.ParsedExercise, name string) *importers.ParsedExercise {
	for _, e := range exercises {
		if strings.EqualFold(e.Name, name) {
			return &e
		}
	}
	return nil
}

// --- Catalog Import (global — no athlete) ---

// CatalogImportPreview summarizes what a catalog import will do.
type CatalogImportPreview struct {
	ExercisesNew    int
	ExercisesMapped int
	EquipmentNew    int
	EquipmentMapped int
	ProgramsNew     int
	ProgramsMapped  int
}

// CatalogImportResult summarizes what was imported.
type CatalogImportResult struct {
	ExercisesCreated    int
	EquipmentCreated    int
	ProgramsCreated     int
	ProgramsAssigned    int
	PrescribedSets      int
	ProgressionRules    int
	ExerciseEquipLinks  int
	CreatedTemplateIDs  []int64 // template IDs created, for post-import exercise auto-assignment
}

// BuildCatalogImportPreview generates a preview of a catalog import.
func BuildCatalogImportPreview(ms *importers.MappingState) *CatalogImportPreview {
	p := &CatalogImportPreview{}

	for _, m := range ms.Exercises {
		if m.Create {
			p.ExercisesNew++
		} else {
			p.ExercisesMapped++
		}
	}
	for _, m := range ms.Equipment {
		if m.Create {
			p.EquipmentNew++
		} else {
			p.EquipmentMapped++
		}
	}
	for _, m := range ms.Programs {
		if m.Create {
			p.ProgramsNew++
		} else {
			p.ProgramsMapped++
		}
	}

	return p
}

// ExecuteCatalogImport creates equipment, exercises, and program templates
// from a parsed catalog file. athleteID scopes new program templates: nil =
// global, non-nil = athlete-specific (e.g. AI-generated).
func ExecuteCatalogImport(db *sql.DB, ms *importers.MappingState, athleteID *int64) (*CatalogImportResult, error) {
	pf := ms.Parsed
	result := &CatalogImportResult{}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin catalog import tx: %w", err)
	}
	defer tx.Rollback()

	// Phase 1: Equipment.
	equipmentIDMap := make(map[string]int64)
	for _, m := range ms.Equipment {
		if m.MappedID > 0 {
			equipmentIDMap[strings.ToLower(m.ImportName)] = m.MappedID
			continue
		}
		if !m.Create {
			continue
		}
		desc := ""
		for _, pe := range pf.Equipment {
			if strings.EqualFold(pe.Name, m.ImportName) {
				if pe.Description != nil {
					desc = *pe.Description
				}
				break
			}
		}
		id, err := insertEquipment(tx, m.ImportName, desc)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return nil, fmt.Errorf("models: catalog import equipment %q: %w", m.ImportName, err)
		}
		equipmentIDMap[strings.ToLower(m.ImportName)] = id
		result.EquipmentCreated++
	}

	// Phase 2: Exercises + equipment dependencies.
	exerciseIDMap := make(map[string]int64)
	for _, m := range ms.Exercises {
		if m.MappedID > 0 {
			exerciseIDMap[strings.ToLower(m.ImportName)] = m.MappedID
			continue
		}
		if !m.Create {
			continue
		}

		pe := findParsedExercise(pf.Exercises, m.ImportName)
		if pe == nil {
			continue
		}

		tier := ""
		formNotes := ""
		demoURL := ""
		restSeconds := 0
		if pe.Tier != nil {
			tier = *pe.Tier
		}
		if pe.FormNotes != nil {
			formNotes = *pe.FormNotes
		}
		if pe.DemoURL != nil {
			demoURL = *pe.DemoURL
		}
		if pe.RestSeconds != nil {
			restSeconds = *pe.RestSeconds
		}

		id, err := insertExercise(tx, pe.Name, tier, formNotes, demoURL, restSeconds, pe.Featured)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return nil, fmt.Errorf("models: catalog import exercise %q: %w", pe.Name, err)
		}
		exerciseIDMap[strings.ToLower(pe.Name)] = id
		result.ExercisesCreated++

		// Wire equipment dependencies.
		for _, eq := range pe.Equipment {
			eqID, ok := equipmentIDMap[strings.ToLower(eq.Name)]
			if !ok {
				continue
			}
			if err := insertExerciseEquipment(tx, id, eqID, eq.Optional); err != nil {
				return nil, fmt.Errorf("models: catalog import exercise-equipment link: %w", err)
			}
			result.ExerciseEquipLinks++
		}
	}

	// Phase 3: Program templates + prescribed sets + progression rules.
	for _, m := range ms.Programs {
		if m.MappedID > 0 || !m.Create {
			continue
		}

		// Find parsed program template.
		var pt *importers.ParsedProgramTemplate
		for _, prog := range pf.Programs {
			if strings.EqualFold(prog.Template.Name, m.ImportName) {
				pt = &prog.Template
				break
			}
		}
		if pt == nil {
			continue
		}

		templateID, err := insertProgramTemplate(tx, *pt, athleteID)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return nil, fmt.Errorf("models: catalog import program template %q: %w", pt.Name, err)
		}
		result.ProgramsCreated++
		result.CreatedTemplateIDs = append(result.CreatedTemplateIDs, templateID)

		// Assign the program to the athlete when scoped to one.
		if athleteID != nil {
			// Deactivate any currently active program first (unique index enforces one active).
			_, _ = tx.Exec(`UPDATE athlete_programs SET active = 0 WHERE athlete_id = ? AND active = 1`, *athleteID)

			startDate := time.Now().Format("2006-01-02")
			if err := insertAthleteProgram(tx, *athleteID, templateID, startDate, "", "", true); err != nil {
				return nil, fmt.Errorf("models: catalog import assign program %q to athlete %d: %w", pt.Name, *athleteID, err)
			}
			result.ProgramsAssigned++
		}

		// Prescribed sets.
		for _, ps := range pt.PrescribedSets {
			exID, ok := exerciseIDMap[strings.ToLower(ps.Exercise)]
			if !ok {
				continue
			}
			if err := insertPrescribedSet(tx, templateID, exID, ps); err != nil {
				return nil, fmt.Errorf("models: catalog import prescribed set: %w", err)
			}
			result.PrescribedSets++
		}

		// Progression rules.
		for _, pr := range pt.ProgressionRules {
			exID, ok := exerciseIDMap[strings.ToLower(pr.Exercise)]
			if !ok {
				continue
			}
			if err := insertProgressionRule(tx, templateID, exID, pr.Increment); err != nil {
				return nil, fmt.Errorf("models: catalog import progression rule: %w", err)
			}
			result.ProgressionRules++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit catalog import: %w", err)
	}

	return result, nil
}
