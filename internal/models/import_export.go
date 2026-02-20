package models

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// --- Export Types ---

// ExportJSON is the top-level structure for RepLog Native JSON export.
type ExportJSON struct {
	Version    string `json:"version"`
	ExportedAt string `json:"exported_at"`
	WeightUnit string `json:"weight_unit"`

	Athlete          ExportAthlete          `json:"athlete"`
	Equipment        []ExportEquipment      `json:"equipment"`
	AthleteEquipment []string               `json:"athlete_equipment"`
	Exercises        []ExportExercise       `json:"exercises"`
	Assignments      []ExportAssignment     `json:"assignments"`
	TrainingMaxes    []ExportTrainingMax    `json:"training_maxes"`
	BodyWeights      []ExportBodyWeight     `json:"body_weights"`
	Workouts         []ExportWorkout        `json:"workouts"`
	Programs         []ExportProgram        `json:"programs"`
}

// ExportAthlete is the athlete profile in a JSON export.
type ExportAthlete struct {
	Name            string  `json:"name"`
	Tier            *string `json:"tier"`
	Notes           *string `json:"notes"`
	Goal            *string `json:"goal"`
	DateOfBirth     *string `json:"date_of_birth,omitempty"`
	Grade           *string `json:"grade,omitempty"`
	Gender          *string `json:"gender,omitempty"`
	TrackBodyWeight bool    `json:"track_body_weight"`
}

// ExportEquipment is an equipment item in a JSON export.
type ExportEquipment struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// ExportExercise is an exercise in a JSON export, including equipment deps.
type ExportExercise struct {
	Name        string                    `json:"name"`
	Tier        *string                   `json:"tier"`
	FormNotes   *string                   `json:"form_notes"`
	DemoURL     *string                   `json:"demo_url"`
	RestSeconds *int                      `json:"rest_seconds"`
	Featured    bool                      `json:"featured"`
	Equipment   []ExportExerciseEquipment `json:"equipment"`
}

// ExportExerciseEquipment is an equipment link for an exercise in a JSON export.
type ExportExerciseEquipment struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional"`
}

// ExportAssignment is an exercise assignment in a JSON export.
type ExportAssignment struct {
	Exercise      string  `json:"exercise"`
	TargetReps    *int    `json:"target_reps"`
	Active        bool    `json:"active"`
	AssignedAt    string  `json:"assigned_at"`
	DeactivatedAt *string `json:"deactivated_at"`
}

// ExportTrainingMax is a training max entry in a JSON export.
type ExportTrainingMax struct {
	Exercise      string  `json:"exercise"`
	Weight        float64 `json:"weight"`
	EffectiveDate string  `json:"effective_date"`
	Notes         *string `json:"notes"`
}

// ExportBodyWeight is a body weight entry in a JSON export.
type ExportBodyWeight struct {
	Date   string  `json:"date"`
	Weight float64 `json:"weight"`
	Notes  *string `json:"notes"`
}

// ExportWorkout is a workout in a JSON export.
type ExportWorkout struct {
	Date   string              `json:"date"`
	Notes  *string             `json:"notes"`
	Review *ExportReview       `json:"review"`
	Sets   []ExportWorkoutSet  `json:"sets"`
}

// ExportReview is a workout review in a JSON export.
type ExportReview struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes"`
}

// ExportWorkoutSet is a single set in a JSON export.
type ExportWorkoutSet struct {
	Exercise  string   `json:"exercise"`
	SetNumber int      `json:"set_number"`
	Reps      int      `json:"reps"`
	RepType   string   `json:"rep_type"`
	Weight    *float64 `json:"weight"`
	RPE       *float64 `json:"rpe"`
	Notes     *string  `json:"notes"`
}

// ExportProgram is a program assignment in a JSON export.
type ExportProgram struct {
	Template  ExportProgramTemplate `json:"template"`
	StartDate string                `json:"start_date"`
	Active    bool                  `json:"active"`
	Notes     *string               `json:"notes"`
	Goal      *string               `json:"goal"`
}

// ExportProgramTemplate is a program template in a JSON export.
type ExportProgramTemplate struct {
	Name             string                   `json:"name"`
	Description      *string                  `json:"description"`
	NumWeeks         int                      `json:"num_weeks"`
	NumDays          int                      `json:"num_days"`
	IsLoop           bool                     `json:"is_loop"`
	PrescribedSets   []ExportPrescribedSet    `json:"prescribed_sets"`
	ProgressionRules []ExportProgressionRule  `json:"progression_rules"`
}

// ExportPrescribedSet is a prescribed set in a JSON export.
type ExportPrescribedSet struct {
	Exercise       string   `json:"exercise"`
	Week           int      `json:"week"`
	Day            int      `json:"day"`
	SetNumber      int      `json:"set_number"`
	Reps           *int     `json:"reps"`
	RepType        string   `json:"rep_type"`
	Percentage     *float64 `json:"percentage"`
	AbsoluteWeight *float64 `json:"absolute_weight"`
	SortOrder      int      `json:"sort_order"`
	Notes          *string  `json:"notes"`
}

// ExportProgressionRule is a progression rule in a JSON export.
type ExportProgressionRule struct {
	Exercise  string  `json:"exercise"`
	Increment float64 `json:"increment"`
}

// --- Export Functions ---

// BuildExportJSON gathers all data for an athlete and returns the full export struct.
func BuildExportJSON(db *sql.DB, athleteID int64) (*ExportJSON, error) {
	athlete, err := GetAthleteByID(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export athlete %d: %w", athleteID, err)
	}

	export := &ExportJSON{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		WeightUnit: "lbs",
	}

	// Athlete profile.
	export.Athlete = ExportAthlete{
		Name:            athlete.Name,
		Tier:            nullStringPtr(athlete.Tier),
		Notes:           nullStringPtr(athlete.Notes),
		Goal:            nullStringPtr(athlete.Goal),
		DateOfBirth:     nullStringPtr(athlete.DateOfBirth),
		Grade:           nullStringPtr(athlete.Grade),
		Gender:          nullStringPtr(athlete.Gender),
		TrackBodyWeight: athlete.TrackBodyWeight,
	}

	// Equipment catalog — gather equipment referenced by athlete's exercises.
	equipmentMap, err := exportEquipment(db, athleteID)
	if err != nil {
		return nil, err
	}
	for _, eq := range equipmentMap {
		export.Equipment = append(export.Equipment, eq)
	}

	// Athlete equipment inventory.
	aeList, err := ListAthleteEquipment(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export athlete equipment: %w", err)
	}
	for _, ae := range aeList {
		export.AthleteEquipment = append(export.AthleteEquipment, ae.EquipmentName)
	}

	// Exercises — all exercises the athlete has worked with or is assigned.
	exerciseMap, err := exportExercises(db, athleteID, equipmentMap)
	if err != nil {
		return nil, err
	}
	for _, ex := range exerciseMap {
		export.Exercises = append(export.Exercises, ex)
	}

	// Assignments (active + deactivated).
	activeAssign, err := ListActiveAssignments(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export active assignments: %w", err)
	}
	deactAssign, err := ListDeactivatedAssignments(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export deactivated assignments: %w", err)
	}
	for _, a := range activeAssign {
		export.Assignments = append(export.Assignments, assignmentToExport(a, true))
	}
	for _, a := range deactAssign {
		export.Assignments = append(export.Assignments, assignmentToExport(a, false))
	}

	// Training maxes for all exported exercises (assigned + used in workouts).
	for exID := range exerciseMap {
		tms, err := ListTrainingMaxHistory(db, athleteID, exID)
		if err != nil {
			return nil, fmt.Errorf("models: export training maxes for exercise %d: %w", exID, err)
		}
		for _, tm := range tms {
			export.TrainingMaxes = append(export.TrainingMaxes, ExportTrainingMax{
				Exercise:      tm.ExerciseName,
				Weight:        tm.Weight,
				EffectiveDate: tm.EffectiveDate,
				Notes:         nullStringPtr(tm.Notes),
			})
		}
	}

	// Body weights.
	export.BodyWeights, err = exportBodyWeights(db, athleteID)
	if err != nil {
		return nil, err
	}

	// Workouts with sets and reviews.
	export.Workouts, err = exportWorkouts(db, athleteID)
	if err != nil {
		return nil, err
	}

	// Programs.
	export.Programs, err = exportPrograms(db, athleteID)
	if err != nil {
		return nil, err
	}

	return export, nil
}

// WriteExportJSON serializes the export to JSON and writes it.
func WriteExportJSON(w io.Writer, export *ExportJSON) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(export)
}

// WriteExportStrongCSV writes workouts as a Strong-compatible CSV.
func WriteExportStrongCSV(w io.Writer, db *sql.DB, athleteID int64) error {
	athlete, err := GetAthleteByID(db, athleteID)
	if err != nil {
		return fmt.Errorf("models: export csv athlete %d: %w", athleteID, err)
	}

	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Header.
	if err := cw.Write([]string{
		"Date", "Workout Name", "Duration", "Exercise Name", "Set Order",
		"Weight", "Reps", "Distance", "Seconds", "Notes", "Workout Notes", "RPE",
	}); err != nil {
		return fmt.Errorf("models: write csv header: %w", err)
	}

	// Iterate all workouts.
	offset := 0
	for {
		page, err := ListWorkouts(db, athleteID, offset)
		if err != nil {
			return fmt.Errorf("models: list workouts for csv: %w", err)
		}

		for _, wo := range page.Workouts {
			groups, err := ListSetsByWorkout(db, wo.ID)
			if err != nil {
				return fmt.Errorf("models: list sets for workout %d: %w", wo.ID, err)
			}

			workoutName := athlete.Name + " — " + wo.Date
			workoutNotes := ""
			if wo.Notes.Valid {
				workoutNotes = wo.Notes.String
			}

			for _, group := range groups {
				for _, set := range group.Sets {
					reps := ""
					seconds := ""
					if set.RepType == "seconds" {
						seconds = strconv.Itoa(set.Reps)
					} else {
						reps = strconv.Itoa(set.Reps)
					}

					weight := ""
					if set.Weight.Valid {
						weight = strconv.FormatFloat(set.Weight.Float64, 'f', -1, 64)
					}

					rpe := ""
					if set.RPE.Valid {
						rpe = strconv.FormatFloat(set.RPE.Float64, 'f', -1, 64)
					}

					notes := ""
					if set.Notes.Valid {
						notes = set.Notes.String
					}

					if err := cw.Write([]string{
						wo.Date + " 00:00:00",
						workoutName,
						"",
						group.ExerciseName,
						strconv.Itoa(set.SetNumber),
						weight,
						reps,
						"",
						seconds,
						notes,
						workoutNotes,
						rpe,
					}); err != nil {
						return fmt.Errorf("models: write csv row: %w", err)
					}

					// Only emit workout notes on the first row.
					workoutNotes = ""
				}
			}
		}

		if !page.HasMore {
			break
		}
		offset += WorkoutPageSize
	}

	return nil
}

// --- Export Helpers ---

func exportEquipment(db *sql.DB, athleteID int64) (map[int64]ExportEquipment, error) {
	// Get all equipment in the system that's relevant to this athlete.
	// Start with athlete equipment, then add equipment used by assigned exercises.
	result := make(map[int64]ExportEquipment)

	// Athlete equipment.
	aeList, err := ListAthleteEquipment(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export equipment (athlete): %w", err)
	}

	allEquipment, err := ListEquipment(db)
	if err != nil {
		return nil, fmt.Errorf("models: export equipment (list): %w", err)
	}
	eqByID := make(map[int64]*Equipment, len(allEquipment))
	for _, eq := range allEquipment {
		eqByID[eq.ID] = eq
	}

	for _, ae := range aeList {
		if eq, ok := eqByID[ae.EquipmentID]; ok {
			result[eq.ID] = ExportEquipment{
				Name:        eq.Name,
				Description: nullStringPtr(eq.Description),
			}
		}
	}

	// Also include equipment linked to assigned exercises (active + deactivated).
	active, err := ListActiveAssignments(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export equipment (active assignments): %w", err)
	}
	deact, err := ListDeactivatedAssignments(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export equipment (deactivated assignments): %w", err)
	}
	allAssignments := append(active, deact...)
	for _, a := range allAssignments {
		eeList, err := ListExerciseEquipment(db, a.ExerciseID)
		if err != nil {
			return nil, fmt.Errorf("models: export equipment for exercise %d: %w", a.ExerciseID, err)
		}
		for _, ee := range eeList {
			if _, exists := result[ee.EquipmentID]; !exists {
				if eq, ok := eqByID[ee.EquipmentID]; ok {
					result[eq.ID] = ExportEquipment{
						Name:        eq.Name,
						Description: nullStringPtr(eq.Description),
					}
				}
			}
		}
	}

	// Also include equipment linked to exercises used in workouts.
	rows, err := db.Query(`
		SELECT DISTINCT ws.exercise_id
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		WHERE w.athlete_id = ?`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export equipment (workout exercises): %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var exID int64
		if err := rows.Scan(&exID); err != nil {
			return nil, err
		}
		eeList, err := ListExerciseEquipment(db, exID)
		if err != nil {
			return nil, fmt.Errorf("models: export equipment for workout exercise %d: %w", exID, err)
		}
		for _, ee := range eeList {
			if _, exists := result[ee.EquipmentID]; !exists {
				if eq, ok := eqByID[ee.EquipmentID]; ok {
					result[eq.ID] = ExportEquipment{
						Name:        eq.Name,
						Description: nullStringPtr(eq.Description),
					}
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func exportExercises(db *sql.DB, athleteID int64, equipmentMap map[int64]ExportEquipment) (map[int64]ExportExercise, error) {
	result := make(map[int64]ExportExercise)

	// Get exercises from workout sets.
	rows, err := db.Query(`
		SELECT DISTINCT ws.exercise_id
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		WHERE w.athlete_id = ?`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export exercise IDs from workouts: %w", err)
	}
	defer rows.Close()

	var exerciseIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		exerciseIDs = append(exerciseIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Also include assigned exercises.
	active, err := ListActiveAssignments(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export exercises (assignments): %w", err)
	}
	deact, err := ListDeactivatedAssignments(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export exercises (deactivated): %w", err)
	}
	for _, a := range active {
		exerciseIDs = append(exerciseIDs, a.ExerciseID)
	}
	for _, a := range deact {
		exerciseIDs = append(exerciseIDs, a.ExerciseID)
	}

	// Deduplicate.
	seen := make(map[int64]bool)
	for _, id := range exerciseIDs {
		if seen[id] {
			continue
		}
		seen[id] = true

		ex, err := GetExerciseByID(db, id)
		if err != nil {
			return nil, fmt.Errorf("models: export exercise %d: %w", id, err)
		}

		ee := ExportExercise{
			Name:      ex.Name,
			Tier:      nullStringPtr(ex.Tier),
			FormNotes: nullStringPtr(ex.FormNotes),
			DemoURL:   nullStringPtr(ex.DemoURL),
			Featured:  ex.Featured,
		}
		if ex.RestSeconds.Valid {
			rs := int(ex.RestSeconds.Int64)
			ee.RestSeconds = &rs
		}

		// Equipment dependencies.
		eqLinks, err := ListExerciseEquipment(db, ex.ID)
		if err != nil {
			return nil, fmt.Errorf("models: export exercise equipment for %d: %w", ex.ID, err)
		}
		for _, link := range eqLinks {
			ee.Equipment = append(ee.Equipment, ExportExerciseEquipment{
				Name:     link.EquipmentName,
				Optional: link.Optional,
			})
		}

		result[ex.ID] = ee
	}

	return result, nil
}

func exportBodyWeights(db *sql.DB, athleteID int64) ([]ExportBodyWeight, error) {
	var result []ExportBodyWeight
	offset := 0
	for {
		page, err := ListBodyWeights(db, athleteID, offset)
		if err != nil {
			return nil, fmt.Errorf("models: export body weights: %w", err)
		}
		for _, bw := range page.Entries {
			result = append(result, ExportBodyWeight{
				Date:   bw.Date,
				Weight: bw.Weight,
				Notes:  nullStringPtr(bw.Notes),
			})
		}
		if !page.HasMore {
			break
		}
		offset += 50 // match BodyWeightPageSize
	}
	return result, nil
}

func exportWorkouts(db *sql.DB, athleteID int64) ([]ExportWorkout, error) {
	var result []ExportWorkout
	offset := 0
	for {
		page, err := ListWorkouts(db, athleteID, offset)
		if err != nil {
			return nil, fmt.Errorf("models: export workouts: %w", err)
		}

		for _, wo := range page.Workouts {
			ew := ExportWorkout{
				Date:  wo.Date,
				Notes: nullStringPtr(wo.Notes),
			}

			// Review.
			rev, err := GetWorkoutReviewByWorkoutID(db, wo.ID)
			if err == nil && rev != nil {
				ew.Review = &ExportReview{
					Status: rev.Status,
					Notes:  nullStringPtr(rev.Notes),
				}
			}

			// Sets.
			groups, err := ListSetsByWorkout(db, wo.ID)
			if err != nil {
				return nil, fmt.Errorf("models: export sets for workout %d: %w", wo.ID, err)
			}
			for _, g := range groups {
				for _, s := range g.Sets {
					es := ExportWorkoutSet{
						Exercise:  g.ExerciseName,
						SetNumber: s.SetNumber,
						Reps:      s.Reps,
						RepType:   s.RepType,
					}
					if s.Weight.Valid {
						w := s.Weight.Float64
						es.Weight = &w
					}
					if s.RPE.Valid {
						r := s.RPE.Float64
						es.RPE = &r
					}
					es.Notes = nullStringPtr(s.Notes)
					ew.Sets = append(ew.Sets, es)
				}
			}

			result = append(result, ew)
		}

		if !page.HasMore {
			break
		}
		offset += WorkoutPageSize
	}
	return result, nil
}

func exportPrograms(db *sql.DB, athleteID int64) ([]ExportProgram, error) {
	// Get athlete programs (active + inactive).
	rows, err := db.Query(`
		SELECT ap.id, ap.template_id, ap.start_date, ap.active, ap.notes, ap.goal,
		       pt.name, pt.description, pt.num_weeks, pt.num_days, pt.is_loop
		FROM athlete_programs ap
		JOIN program_templates pt ON pt.id = ap.template_id
		WHERE ap.athlete_id = ?
		ORDER BY ap.start_date DESC`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export programs for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var result []ExportProgram
	for rows.Next() {
		var (
			apID                         int64
			templateID                   int64
			startDate                    string
			active                       bool
			notes, goal, desc            sql.NullString
			name                         string
			numWeeks, numDays            int
			isLoop                       bool
		)
		if err := rows.Scan(&apID, &templateID, &startDate, &active, &notes, &goal, &name, &desc, &numWeeks, &numDays, &isLoop); err != nil {
			return nil, fmt.Errorf("models: scan export program: %w", err)
		}

		ep := ExportProgram{
			StartDate: startDate,
			Active:    active,
			Notes:     nullStringPtr(notes),
			Goal:      nullStringPtr(goal),
			Template: ExportProgramTemplate{
				Name:        name,
				Description: nullStringPtr(desc),
				NumWeeks:    numWeeks,
				NumDays:     numDays,
				IsLoop:      isLoop,
			},
		}

		// Prescribed sets.
		pSets, err := ListPrescribedSets(db, templateID)
		if err != nil {
			return nil, fmt.Errorf("models: export prescribed sets for template %d: %w", templateID, err)
		}
		for _, ps := range pSets {
			eps := ExportPrescribedSet{
				Exercise:  ps.ExerciseName,
				Week:      ps.Week,
				Day:       ps.Day,
				SetNumber: ps.SetNumber,
				RepType:   ps.RepType,
				SortOrder: ps.SortOrder,
			}
			if ps.Reps.Valid {
				r := int(ps.Reps.Int64)
				eps.Reps = &r
			}
			if ps.Percentage.Valid {
				p := ps.Percentage.Float64
				eps.Percentage = &p
			}
			if ps.AbsoluteWeight.Valid {
				w := ps.AbsoluteWeight.Float64
				eps.AbsoluteWeight = &w
			}
			eps.Notes = nullStringPtr(ps.Notes)
			ep.Template.PrescribedSets = append(ep.Template.PrescribedSets, eps)
		}

		// Progression rules.
		rules, err := ListProgressionRules(db, templateID)
		if err != nil {
			return nil, fmt.Errorf("models: export progression rules for template %d: %w", templateID, err)
		}
		for _, r := range rules {
			ep.Template.ProgressionRules = append(ep.Template.ProgressionRules, ExportProgressionRule{
				Exercise:  r.ExerciseName,
				Increment: r.Increment,
			})
		}

		result = append(result, ep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate export programs: %w", err)
	}

	return result, nil
}

func assignmentToExport(a *AthleteExercise, active bool) ExportAssignment {
	ea := ExportAssignment{
		Exercise:   a.ExerciseName,
		Active:     active,
		AssignedAt: a.AssignedAt.Format(time.RFC3339),
	}
	if a.TargetReps.Valid {
		tr := int(a.TargetReps.Int64)
		ea.TargetReps = &tr
	}
	if a.DeactivatedAt.Valid {
		dt := a.DeactivatedAt.Time.Format(time.RFC3339)
		ea.DeactivatedAt = &dt
	}
	return ea
}

func nullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// --- Catalog Export ---

// CatalogJSON is the top-level structure for a global catalog export
// (exercises, equipment, and program templates — no athlete-specific data).
type CatalogJSON struct {
	Version    string                 `json:"version"`
	ExportedAt string                 `json:"exported_at"`
	Type       string                 `json:"type"` // "catalog"
	Equipment  []ExportEquipment      `json:"equipment"`
	Exercises  []ExportExercise       `json:"exercises"`
	Programs   []ExportProgramTemplate `json:"programs"`
}

// BuildCatalogExportJSON gathers all exercises, equipment, and program templates.
func BuildCatalogExportJSON(db *sql.DB) (*CatalogJSON, error) {
	catalog := &CatalogJSON{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Type:       "catalog",
	}

	// Equipment — all.
	allEquipment, err := ListEquipment(db)
	if err != nil {
		return nil, fmt.Errorf("models: catalog export equipment: %w", err)
	}
	for _, eq := range allEquipment {
		catalog.Equipment = append(catalog.Equipment, ExportEquipment{
			Name:        eq.Name,
			Description: nullStringPtr(eq.Description),
		})
	}

	// Exercises — all, with equipment dependencies.
	allExercises, err := ListExercises(db, "")
	if err != nil {
		return nil, fmt.Errorf("models: catalog export exercises: %w", err)
	}
	for _, ex := range allExercises {
		ee := ExportExercise{
			Name:      ex.Name,
			Tier:      nullStringPtr(ex.Tier),
			FormNotes: nullStringPtr(ex.FormNotes),
			DemoURL:   nullStringPtr(ex.DemoURL),
			Featured:  ex.Featured,
		}
		if ex.RestSeconds.Valid {
			rs := int(ex.RestSeconds.Int64)
			ee.RestSeconds = &rs
		}

		eqLinks, err := ListExerciseEquipment(db, ex.ID)
		if err != nil {
			return nil, fmt.Errorf("models: catalog export exercise equipment for %d: %w", ex.ID, err)
		}
		for _, link := range eqLinks {
			ee.Equipment = append(ee.Equipment, ExportExerciseEquipment{
				Name:     link.EquipmentName,
				Optional: link.Optional,
			})
		}
		catalog.Exercises = append(catalog.Exercises, ee)
	}

	// Program templates — all, with prescribed sets and progression rules.
	allTemplates, err := ListProgramTemplates(db)
	if err != nil {
		return nil, fmt.Errorf("models: catalog export program templates: %w", err)
	}
	for _, pt := range allTemplates {
		ept := ExportProgramTemplate{
			Name:        pt.Name,
			Description: nullStringPtr(pt.Description),
			NumWeeks:    pt.NumWeeks,
			NumDays:     pt.NumDays,
			IsLoop:      pt.IsLoop,
		}

		pSets, err := ListPrescribedSets(db, pt.ID)
		if err != nil {
			return nil, fmt.Errorf("models: catalog export prescribed sets for template %d: %w", pt.ID, err)
		}
		for _, ps := range pSets {
			eps := ExportPrescribedSet{
				Exercise:  ps.ExerciseName,
				Week:      ps.Week,
				Day:       ps.Day,
				SetNumber: ps.SetNumber,
				RepType:   ps.RepType,
				SortOrder: ps.SortOrder,
			}
			if ps.Reps.Valid {
				r := int(ps.Reps.Int64)
				eps.Reps = &r
			}
			if ps.Percentage.Valid {
				p := ps.Percentage.Float64
				eps.Percentage = &p
			}
			if ps.AbsoluteWeight.Valid {
				w := ps.AbsoluteWeight.Float64
				eps.AbsoluteWeight = &w
			}
			eps.Notes = nullStringPtr(ps.Notes)
			ept.PrescribedSets = append(ept.PrescribedSets, eps)
		}

		rules, err := ListProgressionRules(db, pt.ID)
		if err != nil {
			return nil, fmt.Errorf("models: catalog export progression rules for template %d: %w", pt.ID, err)
		}
		for _, r := range rules {
			ept.ProgressionRules = append(ept.ProgressionRules, ExportProgressionRule{
				Exercise:  r.ExerciseName,
				Increment: r.Increment,
			})
		}

		catalog.Programs = append(catalog.Programs, ept)
	}

	return catalog, nil
}

// WriteCatalogJSON serializes the catalog export to JSON and writes it.
func WriteCatalogJSON(w io.Writer, catalog *CatalogJSON) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(catalog)
}

// --- Import Types ---

// ImportMapping holds resolved entity ID mappings for import execution.
type ImportMapping struct {
	// ExerciseMap: import name → existing/new exercise ID
	ExerciseMap map[string]int64
	// EquipmentMap: import name → existing/new equipment ID (RepLog JSON only)
	EquipmentMap map[string]int64
	// ProgramMap: import name → existing/new template ID (RepLog JSON only)
	ProgramMap map[string]int64
}

// ImportPreview summarizes what an import will do before execution.
// ValidationWarning represents a non-blocking data quality issue found during
// import preview. Warnings are shown to the user but do not prevent import.
type ValidationWarning struct {
	Entity  string // "workout", "set", "training_max", "body_weight"
	Field   string // "weight", "reps", "rpe", "date", "rep_type"
	Message string
}

type ImportPreview struct {
	WorkoutCount     int
	SetCount         int
	ExercisesNew     int
	ExercisesMapped  int
	ExerciseCount    int      // ExercisesNew + ExercisesMapped (for template)
	EquipmentNew     int
	EquipmentMapped  int
	EquipmentCount   int      // EquipmentNew + EquipmentMapped (for template)
	ProgramsNew      int
	ProgramsMapped   int
	ProgramCount     int      // ProgramsNew + ProgramsMapped (for template)
	ConflictDates    []string // dates that already have workouts
	AssignmentCount  int
	TrainingMaxCount int
	BodyWeightCount  int
	ReviewCount      int
	DateRange        string // "YYYY-MM-DD to YYYY-MM-DD"
	Warnings         []ValidationWarning
}

// ImportResult summarizes what was imported after execution.
type ImportResult struct {
	WorkoutsCreated      int
	SetsCreated          int
	ExercisesCreated     int
	ExercisesSkipped     int
	EquipmentCreated     int
	EquipmentSkipped     int
	AssignmentsCreated   int
	AssignmentsSkipped   int
	TrainingMaxesCreated int
	TrainingMaxesSkipped int
	BodyWeightsCreated   int
	BodyWeightsSkipped   int
	ReviewsCreated       int
	ProgramsCreated      int
	ProgramsSkipped      int
	WorkoutsSkipped      int // existing date conflicts
}
