package models

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// --- Per-Athlete Export Types (ADR 006) ---
//
// These types back the per-athlete export endpoints
// (GET /api/athletes/{id}/export/json and .../export/csv). The catalog export
// types live in import_export.go; this file adds the athlete-scoped aggregate.
// Where practical the shared Export* types (ExportExercise, ExportPrescribedSet,
// ExportProgressionRule, ExportProgramTemplate) are reused rather than
// duplicated.

// nullFloatPtr converts a sql.NullFloat64 to *float64 for JSON serialization.
func nullFloatPtr(nf sql.NullFloat64) *float64 {
	if nf.Valid {
		return &nf.Float64
	}
	return nil
}

// nullIntPtr converts a sql.NullInt64 to *int64 for JSON serialization.
func nullIntPtr(ni sql.NullInt64) *int64 {
	if ni.Valid {
		return &ni.Int64
	}
	return nil
}

// nullTimePtr converts a sql.NullTime to an *RFC3339 string for JSON.
func nullTimePtr(nt sql.NullTime) *string {
	if nt.Valid {
		s := nt.Time.UTC().Format(time.RFC3339)
		return &s
	}
	return nil
}

// ExportAthleteProfile is the athlete profile block of a per-athlete export.
type ExportAthleteProfile struct {
	Name            string  `json:"name"`
	Tier            *string `json:"tier"`
	Notes           *string `json:"notes"`
	Goal            *string `json:"goal"`
	DateOfBirth     *string `json:"date_of_birth"`
	Grade           *string `json:"grade"`
	Gender          *string `json:"gender"`
	TrackBodyWeight bool    `json:"track_body_weight"`
}

// ExportAssignment is an exercise assignment (active or historical).
type ExportAssignment struct {
	Exercise      string  `json:"exercise"`
	TargetReps    *int64  `json:"target_reps"`
	Active        bool    `json:"active"`
	AssignedAt    string  `json:"assigned_at"`
	DeactivatedAt *string `json:"deactivated_at"`
}

// ExportTrainingMax is one training-max history record.
type ExportTrainingMax struct {
	Exercise      string  `json:"exercise"`
	Weight        float64 `json:"weight"`
	EffectiveDate string  `json:"effective_date"`
	Notes         *string `json:"notes"`
}

// ExportBodyWeight is one body-weight history record.
type ExportBodyWeight struct {
	Date   string  `json:"date"`
	Weight float64 `json:"weight"`
	Notes  *string `json:"notes"`
}

// ExportNote is a coach/athlete note attached to the athlete.
type ExportNote struct {
	Date      string `json:"date"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	IsPrivate bool   `json:"is_private"`
	Pinned    bool   `json:"pinned"`
}

// ExportReview is a coach's review of a workout.
type ExportReview struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes"`
}

// ExportSet is one logged set within a workout.
type ExportSet struct {
	Exercise  string   `json:"exercise"`
	SetNumber int      `json:"set_number"`
	Reps      int      `json:"reps"`
	RepType   string   `json:"rep_type"`
	Weight    *float64 `json:"weight"`
	RPE       *float64 `json:"rpe"`
	Category  string   `json:"category"`
	Notes     *string  `json:"notes"`
}

// ExportWorkout is a single workout with its sets and optional review.
type ExportWorkout struct {
	Date       string        `json:"date"`
	Discipline string        `json:"discipline"`
	Notes      *string       `json:"notes"`
	Review     *ExportReview `json:"review"`
	Sets       []ExportSet   `json:"sets"`
}

// ExportProgramAssignment is a program assignment with its (embedded) template.
type ExportProgramAssignment struct {
	Template  ExportProgramTemplate `json:"template"`
	StartDate string                `json:"start_date"`
	Active    bool                  `json:"active"`
	Role      string                `json:"role"`
	Notes     *string               `json:"notes"`
	Goal      *string               `json:"goal"`
}

// ExportConditioningSession is a conditioning multimodal session.
type ExportConditioningSession struct {
	Date            string   `json:"date"`
	Modality        string   `json:"modality"`
	SessionType     string   `json:"session_type"`
	TotalDistance   *float64 `json:"total_distance"`
	DistanceUnit    *string  `json:"distance_unit"`
	DurationSeconds *int64   `json:"duration_seconds"`
	AvgHR           *int64   `json:"avg_hr"`
	RPE             *float64 `json:"rpe"`
	Notes           *string  `json:"notes"`
}

// ExportThrowingSession is a throwing/arm-care multimodal session.
type ExportThrowingSession struct {
	Date       string   `json:"date"`
	ThrowType  string   `json:"throw_type"`
	ThrowCount *int64   `json:"throw_count"`
	MaxIntent  *int64   `json:"max_intent"`
	Velocity   *float64 `json:"velocity"`
	Fatigue    bool     `json:"fatigue"`
	Pain       bool     `json:"pain"`
	Source     string   `json:"source"`
	Team       *string  `json:"team"`
	Notes      *string  `json:"notes"`
}

// ExportSkillSession is a sport-skill multimodal session.
type ExportSkillSession struct {
	Date            string   `json:"date"`
	SkillType       string   `json:"skill_type"`
	RepCount        *int64   `json:"rep_count"`
	LoadKg          *float64 `json:"load_kg"`
	Velocity        *float64 `json:"velocity"`
	DurationSeconds *int64   `json:"duration_seconds"`
	Notes           *string  `json:"notes"`
}

// ExportRecoveryCheckin is a subjective recovery check-in.
type ExportRecoveryCheckin struct {
	Date       string   `json:"date"`
	SleepHours *float64 `json:"sleep_hours"`
	Soreness   *int64   `json:"soreness"`
	Energy     *int64   `json:"energy"`
	Notes      *string  `json:"notes"`
}

// ExportSessions groups the non-resistance (multimodal) sessions by discipline.
type ExportSessions struct {
	Conditioning []ExportConditioningSession `json:"conditioning"`
	Throwing     []ExportThrowingSession     `json:"throwing"`
	Skill        []ExportSkillSession        `json:"skill"`
	Recovery     []ExportRecoveryCheckin     `json:"recovery"`
}

// AthleteExportJSON is the top-level structure for a per-athlete export.
type AthleteExportJSON struct {
	Version       string                    `json:"version"`
	ExportedAt    string                    `json:"exported_at"`
	Type          string                    `json:"type"` // "athlete"
	WeightUnit    string                    `json:"weight_unit"`
	Athlete       ExportAthleteProfile      `json:"athlete"`
	Equipment     []string                  `json:"equipment"`
	Exercises     []ExportExercise          `json:"exercises"`
	Assignments   []ExportAssignment        `json:"assignments"`
	TrainingMaxes []ExportTrainingMax       `json:"training_maxes"`
	BodyWeights   []ExportBodyWeight        `json:"body_weights"`
	Notes         []ExportNote              `json:"notes"`
	Workouts      []ExportWorkout           `json:"workouts"`
	Programs      []ExportProgramAssignment `json:"programs"`
	Sessions      ExportSessions            `json:"sessions"`
}

// BuildAthleteExportJSON assembles a complete, re-importable snapshot of one
// athlete's data (ADR 006). It reuses the existing model query functions where
// available and issues a handful of full-history queries for data that only has
// paginated list helpers (workouts, training maxes, body weights, assignments).
func BuildAthleteExportJSON(db *sql.DB, athleteID int64) (*AthleteExportJSON, error) {
	athlete, err := GetAthleteByID(db, athleteID)
	if err != nil {
		return nil, err // includes ErrNotFound
	}

	out := &AthleteExportJSON{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Type:       "athlete",
		WeightUnit: "lbs",
		Athlete: ExportAthleteProfile{
			Name:            athlete.Name,
			Tier:            nullStringPtr(athlete.Tier),
			Notes:           nullStringPtr(athlete.Notes),
			Goal:            nullStringPtr(athlete.Goal),
			DateOfBirth:     nullStringPtr(athlete.DateOfBirth),
			Grade:           nullStringPtr(athlete.Grade),
			Gender:          nullStringPtr(athlete.Gender),
			TrackBodyWeight: athlete.TrackBodyWeight,
		},
		// Non-nil empty slices so the JSON has stable keys ([] not null).
		Equipment:     []string{},
		Exercises:     []ExportExercise{},
		Assignments:   []ExportAssignment{},
		TrainingMaxes: []ExportTrainingMax{},
		BodyWeights:   []ExportBodyWeight{},
		Notes:         []ExportNote{},
		Workouts:      []ExportWorkout{},
		Programs:      []ExportProgramAssignment{},
		Sessions: ExportSessions{
			Conditioning: []ExportConditioningSession{},
			Throwing:     []ExportThrowingSession{},
			Skill:        []ExportSkillSession{},
			Recovery:     []ExportRecoveryCheckin{},
		},
	}

	// referencedExercises collects every exercise ID referenced anywhere in the
	// export so we can emit a self-contained exercise catalog with equipment deps.
	referencedExercises := make(map[int64]bool)

	// --- Athlete equipment inventory ---
	equip, err := ListAthleteEquipment(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export athlete equipment: %w", err)
	}
	for _, e := range equip {
		out.Equipment = append(out.Equipment, e.EquipmentName)
	}

	// --- Assignments (active + historical) ---
	assignRows, err := db.Query(`
		SELECT ae.exercise_id, e.name, ae.target_reps, ae.active, ae.assigned_at, ae.deactivated_at
		FROM athlete_exercises ae
		JOIN exercises e ON e.id = ae.exercise_id
		WHERE ae.athlete_id = ?
		ORDER BY ae.assigned_at`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export assignments: %w", err)
	}
	for assignRows.Next() {
		var exerciseID int64
		var name string
		var targetReps sql.NullInt64
		var active bool
		var assignedAt time.Time
		var deactivatedAt sql.NullTime
		if err := assignRows.Scan(&exerciseID, &name, &targetReps, &active, &assignedAt, &deactivatedAt); err != nil {
			assignRows.Close()
			return nil, fmt.Errorf("models: scan export assignment: %w", err)
		}
		referencedExercises[exerciseID] = true
		out.Assignments = append(out.Assignments, ExportAssignment{
			Exercise:      name,
			TargetReps:    nullIntPtr(targetReps),
			Active:        active,
			AssignedAt:    assignedAt.UTC().Format(time.RFC3339),
			DeactivatedAt: nullTimePtr(deactivatedAt),
		})
	}
	if err := assignRows.Err(); err != nil {
		assignRows.Close()
		return nil, fmt.Errorf("models: iterate export assignments: %w", err)
	}
	assignRows.Close()

	// --- Training maxes (full history) ---
	tmRows, err := db.Query(`
		SELECT tm.exercise_id, e.name, tm.weight, tm.effective_date, tm.notes
		FROM training_maxes tm
		JOIN exercises e ON e.id = tm.exercise_id
		WHERE tm.athlete_id = ?
		ORDER BY e.name COLLATE NOCASE, tm.effective_date`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export training maxes: %w", err)
	}
	for tmRows.Next() {
		var exerciseID int64
		var name, effectiveDate string
		var weight float64
		var notes sql.NullString
		if err := tmRows.Scan(&exerciseID, &name, &weight, &effectiveDate, &notes); err != nil {
			tmRows.Close()
			return nil, fmt.Errorf("models: scan export training max: %w", err)
		}
		referencedExercises[exerciseID] = true
		out.TrainingMaxes = append(out.TrainingMaxes, ExportTrainingMax{
			Exercise:      name,
			Weight:        weight,
			EffectiveDate: effectiveDate,
			Notes:         nullStringPtr(notes),
		})
	}
	if err := tmRows.Err(); err != nil {
		tmRows.Close()
		return nil, fmt.Errorf("models: iterate export training maxes: %w", err)
	}
	tmRows.Close()

	// --- Body weights (full history) ---
	bwRows, err := db.Query(`
		SELECT date, weight, notes
		FROM body_weights
		WHERE athlete_id = ?
		ORDER BY date`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export body weights: %w", err)
	}
	for bwRows.Next() {
		var date string
		var weight float64
		var notes sql.NullString
		if err := bwRows.Scan(&date, &weight, &notes); err != nil {
			bwRows.Close()
			return nil, fmt.Errorf("models: scan export body weight: %w", err)
		}
		out.BodyWeights = append(out.BodyWeights, ExportBodyWeight{
			Date:   date,
			Weight: weight,
			Notes:  nullStringPtr(notes),
		})
	}
	if err := bwRows.Err(); err != nil {
		bwRows.Close()
		return nil, fmt.Errorf("models: iterate export body weights: %w", err)
	}
	bwRows.Close()

	// --- Notes (includes private) ---
	notes, err := ListAthleteNotes(db, athleteID, true)
	if err != nil {
		return nil, fmt.Errorf("models: export notes: %w", err)
	}
	for _, n := range notes {
		out.Notes = append(out.Notes, ExportNote{
			Date:      n.Date,
			Content:   n.Content,
			Author:    n.AuthorName,
			IsPrivate: n.IsPrivate,
			Pinned:    n.Pinned,
		})
	}

	// --- Workouts (all disciplines) with sets and reviews ---
	if err := buildExportWorkouts(db, athleteID, out, referencedExercises); err != nil {
		return nil, err
	}

	// --- Program assignments (with template definition) ---
	programs, err := ListAthletePrograms(db, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: export programs: %w", err)
	}
	for _, ap := range programs {
		tmpl := ExportProgramTemplate{
			Name:     ap.TemplateName,
			NumWeeks: ap.NumWeeks,
			NumDays:  ap.NumDays,
			IsLoop:   ap.IsLoop,
		}

		pSets, err := ListPrescribedSets(db, ap.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("models: export prescribed sets for template %d: %w", ap.TemplateID, err)
		}
		for _, ps := range pSets {
			referencedExercises[ps.ExerciseID] = true
			eps := ExportPrescribedSet{
				Exercise:  ps.ExerciseName,
				Week:      ps.Week,
				Day:       ps.Day,
				SetNumber: ps.SetNumber,
				RepType:   ps.RepType,
				SortOrder: ps.SortOrder,
				Notes:     nullStringPtr(ps.Notes),
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
			tmpl.PrescribedSets = append(tmpl.PrescribedSets, eps)
		}

		rules, err := ListProgressionRules(db, ap.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("models: export progression rules for template %d: %w", ap.TemplateID, err)
		}
		for _, r := range rules {
			tmpl.ProgressionRules = append(tmpl.ProgressionRules, ExportProgressionRule{
				Exercise:  r.ExerciseName,
				Increment: r.Increment,
			})
		}

		out.Programs = append(out.Programs, ExportProgramAssignment{
			Template:  tmpl,
			StartDate: ap.StartDate,
			Active:    ap.Active,
			Role:      ap.Role,
			Notes:     nullStringPtr(ap.Notes),
			Goal:      nullStringPtr(ap.Goal),
		})
	}

	// --- Multimodal sessions ---
	if err := buildExportSessions(db, athleteID, out); err != nil {
		return nil, err
	}

	// --- Referenced exercise catalog (name, tier, equipment deps) ---
	ids := make([]int64, 0, len(referencedExercises))
	for id := range referencedExercises {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		ex, err := GetExerciseByID(db, id)
		if err != nil {
			// Skip a missing exercise rather than fail the whole export.
			continue
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
		out.Exercises = append(out.Exercises, ee)
	}

	return out, nil
}

// buildExportWorkouts loads every workout for the athlete (all disciplines) with
// its sets and review, appending them to out.Workouts and recording referenced
// exercise IDs.
func buildExportWorkouts(db *sql.DB, athleteID int64, out *AthleteExportJSON, refs map[int64]bool) error {
	rows, err := db.Query(`
		SELECT id, date, discipline, notes
		FROM workouts
		WHERE athlete_id = ?
		ORDER BY date`, athleteID)
	if err != nil {
		return fmt.Errorf("models: export workouts: %w", err)
	}
	defer rows.Close()

	type workoutRow struct {
		id         int64
		date       string
		discipline string
		notes      sql.NullString
	}
	var wrs []workoutRow
	var ids []int64
	for rows.Next() {
		var wr workoutRow
		if err := rows.Scan(&wr.id, &wr.date, &wr.discipline, &wr.notes); err != nil {
			return fmt.Errorf("models: scan export workout: %w", err)
		}
		wrs = append(wrs, wr)
		ids = append(ids, wr.id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("models: iterate export workouts: %w", err)
	}

	if len(wrs) == 0 {
		return nil
	}

	setsByWorkout, err := ListSetsByWorkoutIDs(db, ids)
	if err != nil {
		return fmt.Errorf("models: export workout sets: %w", err)
	}

	for _, wr := range wrs {
		ew := ExportWorkout{
			Date:       wr.date,
			Discipline: wr.discipline,
			Notes:      nullStringPtr(wr.notes),
			Sets:       []ExportSet{},
		}

		for _, group := range setsByWorkout[wr.id] {
			for _, s := range group.Sets {
				refs[s.ExerciseID] = true
				ew.Sets = append(ew.Sets, ExportSet{
					Exercise:  s.ExerciseName,
					SetNumber: s.SetNumber,
					Reps:      s.Reps,
					RepType:   s.RepType,
					Weight:    nullFloatPtr(s.Weight),
					RPE:       nullFloatPtr(s.RPE),
					Category:  s.Category,
					Notes:     nullStringPtr(s.Notes),
				})
			}
		}

		review, err := GetWorkoutReviewByWorkoutID(db, wr.id)
		if err == nil {
			ew.Review = &ExportReview{
				Status: review.Status,
				Notes:  nullStringPtr(review.Notes),
			}
		}

		out.Workouts = append(out.Workouts, ew)
	}
	return nil
}

// buildExportSessions loads the four multimodal session types for the athlete.
func buildExportSessions(db *sql.DB, athleteID int64, out *AthleteExportJSON) error {
	const allSessions = 100000

	cond, err := ListConditioningSessions(db, athleteID, allSessions)
	if err != nil {
		return fmt.Errorf("models: export conditioning sessions: %w", err)
	}
	for _, c := range cond {
		out.Sessions.Conditioning = append(out.Sessions.Conditioning, ExportConditioningSession{
			Date:            c.Date,
			Modality:        c.Modality,
			SessionType:     c.SessionType,
			TotalDistance:   nullFloatPtr(c.TotalDistance),
			DistanceUnit:    nullStringPtr(c.DistanceUnit),
			DurationSeconds: nullIntPtr(c.DurationSeconds),
			AvgHR:           nullIntPtr(c.AvgHR),
			RPE:             nullFloatPtr(c.RPE),
			Notes:           nullStringPtr(c.Notes),
		})
	}

	throw, err := ListThrowingSessions(db, athleteID, allSessions)
	if err != nil {
		return fmt.Errorf("models: export throwing sessions: %w", err)
	}
	for _, t := range throw {
		out.Sessions.Throwing = append(out.Sessions.Throwing, ExportThrowingSession{
			Date:       t.Date,
			ThrowType:  t.ThrowType,
			ThrowCount: nullIntPtr(t.ThrowCount),
			MaxIntent:  nullIntPtr(t.MaxIntent),
			Velocity:   nullFloatPtr(t.Velocity),
			Fatigue:    t.Fatigue,
			Pain:       t.Pain,
			Source:     t.Source,
			Team:       nullStringPtr(t.Team),
			Notes:      nullStringPtr(t.Notes),
		})
	}

	skill, err := ListSkillSessions(db, athleteID, allSessions)
	if err != nil {
		return fmt.Errorf("models: export skill sessions: %w", err)
	}
	for _, s := range skill {
		out.Sessions.Skill = append(out.Sessions.Skill, ExportSkillSession{
			Date:            s.Date,
			SkillType:       s.SkillType,
			RepCount:        nullIntPtr(s.RepCount),
			LoadKg:          nullFloatPtr(s.LoadKg),
			Velocity:        nullFloatPtr(s.Velocity),
			DurationSeconds: nullIntPtr(s.DurationSeconds),
			Notes:           nullStringPtr(s.Notes),
		})
	}

	rec, err := ListRecoveryCheckins(db, athleteID, allSessions)
	if err != nil {
		return fmt.Errorf("models: export recovery checkins: %w", err)
	}
	for _, r := range rec {
		out.Sessions.Recovery = append(out.Sessions.Recovery, ExportRecoveryCheckin{
			Date:       r.Date,
			SleepHours: nullFloatPtr(r.SleepHours),
			Soreness:   nullIntPtr(r.Soreness),
			Energy:     nullIntPtr(r.Energy),
			Notes:      nullStringPtr(r.Notes),
		})
	}

	return nil
}
