package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrExerciseInUse is returned when attempting to delete an exercise that has
// been logged in workout sets (ON DELETE RESTRICT).
var ErrExerciseInUse = errors.New("exercise is referenced by workout sets")

// ErrDuplicateExerciseName is returned when an exercise name is already taken.
var ErrDuplicateExerciseName = errors.New("duplicate exercise name")

// DefaultRestSeconds is the global fallback rest time (in seconds) when an
// exercise does not specify its own.
const DefaultRestSeconds = 90

// Exercise represents a movement tracked in the system.
type Exercise struct {
	ID          int64
	Name        string
	Tier        sql.NullString
	FormNotes   sql.NullString
	DemoURL     sql.NullString
	RestSeconds sql.NullInt64
	Featured    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EffectiveRestSeconds returns the exercise's rest time or the global default.
func (e *Exercise) EffectiveRestSeconds() int {
	if e.RestSeconds.Valid {
		return int(e.RestSeconds.Int64)
	}
	return DefaultRestSeconds
}

// CreateExercise inserts a new exercise.
func CreateExercise(db *sql.DB, name, tier string, formNotes, demoURL string, restSeconds int, featured ...bool) (*Exercise, error) {
	feat := false
	if len(featured) > 0 {
		feat = featured[0]
	}
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var notesVal sql.NullString
	if formNotes != "" {
		notesVal = sql.NullString{String: formNotes, Valid: true}
	}
	var demoVal sql.NullString
	if demoURL != "" {
		demoVal = sql.NullString{String: demoURL, Valid: true}
	}
	var restVal sql.NullInt64
	if restSeconds > 0 {
		restVal = sql.NullInt64{Int64: int64(restSeconds), Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO exercises (name, tier, form_notes, demo_url, rest_seconds, featured) VALUES (?, ?, ?, ?, ?, ?)`,
		name, tierVal, notesVal, demoVal, restVal, feat,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateExerciseName
		}
		return nil, fmt.Errorf("models: create exercise %q: %w", name, err)
	}

	id, _ := result.LastInsertId()
	return GetExerciseByID(db, id)
}

// GetExerciseByID retrieves an exercise by primary key.
func GetExerciseByID(db *sql.DB, id int64) (*Exercise, error) {
	e := &Exercise{}
	err := db.QueryRow(
		`SELECT id, name, tier, form_notes, demo_url, rest_seconds, featured, created_at, updated_at
		 FROM exercises WHERE id = ?`, id,
	).Scan(&e.ID, &e.Name, &e.Tier, &e.FormNotes, &e.DemoURL, &e.RestSeconds, &e.Featured, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get exercise %d: %w", id, err)
	}
	return e, nil
}

// UpdateExercise modifies an existing exercise's fields.
func UpdateExercise(db *sql.DB, id int64, name, tier string, formNotes, demoURL string, restSeconds int, featured ...bool) (*Exercise, error) {
	feat := false
	if len(featured) > 0 {
		feat = featured[0]
	}
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var notesVal sql.NullString
	if formNotes != "" {
		notesVal = sql.NullString{String: formNotes, Valid: true}
	}
	var demoVal sql.NullString
	if demoURL != "" {
		demoVal = sql.NullString{String: demoURL, Valid: true}
	}
	var restVal sql.NullInt64
	if restSeconds > 0 {
		restVal = sql.NullInt64{Int64: int64(restSeconds), Valid: true}
	}

	result, err := db.Exec(
		`UPDATE exercises SET name = ?, tier = ?, form_notes = ?, demo_url = ?, rest_seconds = ?, featured = ? WHERE id = ?`,
		name, tierVal, notesVal, demoVal, restVal, feat, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateExerciseName
		}
		return nil, fmt.Errorf("models: update exercise %d: %w", id, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, ErrNotFound
	}

	return GetExerciseByID(db, id)
}

// DeleteExercise removes an exercise by ID. Returns ErrExerciseInUse if the
// exercise has been logged in any workout sets (RESTRICT).
func DeleteExercise(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM exercises WHERE id = ?`, id)
	if err != nil {
		if errContains(err, "FOREIGN KEY constraint failed") {
			return ErrExerciseInUse
		}
		return fmt.Errorf("models: delete exercise %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListExercises returns all exercises, optionally filtered by tier.
// Pass empty string for tier to list all.
func ListExercises(db *sql.DB, tierFilter string) ([]*Exercise, error) {
	var query string
	var args []any

	if tierFilter == "none" {
		query = `SELECT id, name, tier, form_notes, demo_url, rest_seconds, featured, created_at, updated_at
		         FROM exercises WHERE tier IS NULL ORDER BY name COLLATE NOCASE LIMIT 200`
	} else if tierFilter != "" {
		query = `SELECT id, name, tier, form_notes, demo_url, rest_seconds, featured, created_at, updated_at
		         FROM exercises WHERE tier = ? ORDER BY name COLLATE NOCASE LIMIT 200`
		args = append(args, tierFilter)
	} else {
		query = `SELECT id, name, tier, form_notes, demo_url, rest_seconds, featured, created_at, updated_at
		         FROM exercises ORDER BY name COLLATE NOCASE LIMIT 200`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("models: list exercises: %w", err)
	}
	defer rows.Close()

	var exercises []*Exercise
	for rows.Next() {
		e := &Exercise{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Tier, &e.FormNotes, &e.DemoURL, &e.RestSeconds, &e.Featured, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("models: scan exercise: %w", err)
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}

// FeaturedLift holds summary data for one featured exercise for an athlete.
type FeaturedLift struct {
	ExerciseID   int64
	ExerciseName string

	// Current training max (nil if none set).
	CurrentTM *TrainingMax

	// Best logged set: heaviest weight×reps combo from workout_sets.
	BestWeight sql.NullFloat64
	BestReps   int
	BestDate   string // YYYY-MM-DD

	// Estimated 1RM via Epley formula from the best set (0 if no data).
	Estimated1RM float64
}

// ListFeaturedLifts returns featured exercise summaries for an athlete.
// For each featured exercise that the athlete has assigned (active) or has
// logged sets for, it returns the current TM, best set, and estimated 1RM.
func ListFeaturedLifts(db *sql.DB, athleteID int64) ([]*FeaturedLift, error) {
	// Get all featured exercises.
	rows, err := db.Query(
		`SELECT id, name FROM exercises WHERE featured = 1 ORDER BY name COLLATE NOCASE LIMIT 50`,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list featured exercises: %w", err)
	}
	defer rows.Close()

	type exerciseRef struct {
		id   int64
		name string
	}
	var refs []exerciseRef
	for rows.Next() {
		var r exerciseRef
		if err := rows.Scan(&r.id, &r.name); err != nil {
			return nil, fmt.Errorf("models: scan featured exercise: %w", err)
		}
		refs = append(refs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate featured exercises: %w", err)
	}
	if len(refs) == 0 {
		return nil, nil
	}

	var lifts []*FeaturedLift
	for _, ref := range refs {
		lift := &FeaturedLift{
			ExerciseID:   ref.id,
			ExerciseName: ref.name,
		}

		// Current training max.
		tm, err := CurrentTrainingMax(db, athleteID, ref.id)
		if err == nil {
			lift.CurrentTM = tm
		}

		// Best logged set: highest weight with its reps (for weighted exercises).
		err = db.QueryRow(
			`SELECT ws.weight, ws.reps, w.date
			 FROM workout_sets ws
			 JOIN workouts w ON w.id = ws.workout_id
			 WHERE w.athlete_id = ? AND ws.exercise_id = ? AND ws.weight IS NOT NULL AND ws.weight > 0
			 ORDER BY ws.weight DESC, ws.reps DESC
			 LIMIT 1`,
			athleteID, ref.id,
		).Scan(&lift.BestWeight, &lift.BestReps, &lift.BestDate)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("models: best set for athlete %d exercise %d: %w", athleteID, ref.id, err)
		}

		// Estimated 1RM via Epley formula: weight × (1 + reps/30).
		if lift.BestWeight.Valid && lift.BestWeight.Float64 > 0 && lift.BestReps > 0 {
			if lift.BestReps == 1 {
				lift.Estimated1RM = lift.BestWeight.Float64
			} else {
				lift.Estimated1RM = lift.BestWeight.Float64 * (1 + float64(lift.BestReps)/30.0)
			}
		}

		// Only include if there's any data for this athlete.
		if lift.CurrentTM != nil || lift.BestWeight.Valid {
			lifts = append(lifts, lift)
		}
	}

	return lifts, nil
}
