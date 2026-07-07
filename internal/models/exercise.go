package models

import (
	"context"
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
func CreateExercise(ctx context.Context, db *sql.DB, name, tier string, formNotes, demoURL string, restSeconds int, featured ...bool) (*Exercise, error) {
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

	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO exercises (name, tier, form_notes, demo_url, rest_seconds, featured) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		name, tierVal, notesVal, demoVal, restVal, feat,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateExerciseName
		}
		return nil, fmt.Errorf("models: create exercise %q: %w", name, err)
	}

	return GetExerciseByID(ctx, db, id)
}

// GetExerciseByID retrieves an exercise by primary key.
func GetExerciseByID(ctx context.Context, db *sql.DB, id int64) (*Exercise, error) {
	e := &Exercise{}
	err := db.QueryRowContext(ctx,
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
func UpdateExercise(ctx context.Context, db *sql.DB, id int64, name, tier string, formNotes, demoURL string, restSeconds int, featured ...bool) (*Exercise, error) {
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

	result, err := db.ExecContext(ctx,
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

	return GetExerciseByID(ctx, db, id)
}

// DeleteExercise removes an exercise by ID. Returns ErrExerciseInUse if the
// exercise has been logged in any workout sets (RESTRICT).
func DeleteExercise(ctx context.Context, db *sql.DB, id int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM exercises WHERE id = ?`, id)
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
func ListExercises(ctx context.Context, db *sql.DB, tierFilter string) ([]*Exercise, error) {
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

	rows, err := db.QueryContext(ctx, query, args...)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate exercises: %w", err)
	}
	return exercises, nil
}

// MovementPattern enumerates the Dan John fundamental movement patterns
// used to tag exercises (ADR 016 Phase 1). The set is enforced by a CHECK
// constraint on exercise_movement_patterns.pattern in migration 0004.
type MovementPattern = string

const (
	MovementPatternPush   MovementPattern = "push"
	MovementPatternPull   MovementPattern = "pull"
	MovementPatternHinge  MovementPattern = "hinge"
	MovementPatternSquat  MovementPattern = "squat"
	MovementPatternCarry  MovementPattern = "carry"
	MovementPatternGround MovementPattern = "ground"
)

// validMovementPatterns mirrors the CHECK constraint so callers can validate
// input before the round-trip to SQLite returns a constraint error.
var validMovementPatterns = map[string]struct{}{
	MovementPatternPush:   {},
	MovementPatternPull:   {},
	MovementPatternHinge:  {},
	MovementPatternSquat:  {},
	MovementPatternCarry:  {},
	MovementPatternGround: {},
}

// IsValidMovementPattern reports whether s is one of the six Dan John tags.
func IsValidMovementPattern(s string) bool {
	_, ok := validMovementPatterns[s]
	return ok
}

// ListExerciseMovementPatterns returns the movement-pattern tags for an
// exercise, in stable alphabetical order. Returns an empty slice when the
// exercise has no tags (the common case for exercises seeded before ADR 016
// Phase 1 ran).
func ListExerciseMovementPatterns(ctx context.Context, db *sql.DB, exerciseID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT pattern FROM exercise_movement_patterns
		 WHERE exercise_id = ? ORDER BY pattern`,
		exerciseID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list exercise movement patterns %d: %w", exerciseID, err)
	}
	defer rows.Close()

	patterns := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("models: scan movement pattern: %w", err)
		}
		patterns = append(patterns, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate movement patterns: %w", err)
	}
	return patterns, nil
}

// SetExerciseMovementPatterns replaces the movement-pattern tags for an
// exercise with the given set. Use this for admin edits to an exercise's
// tags after seeding; the seed path writes via the import-tx helper so all
// inserts share the same transaction as the exercise itself.
//
// All patterns are validated against IsValidMovementPattern; if any value
// is invalid the entire operation is rejected and no rows are touched.
func SetExerciseMovementPatterns(ctx context.Context, db *sql.DB, exerciseID int64, patterns []string) error {
	for _, p := range patterns {
		if !IsValidMovementPattern(p) {
			return fmt.Errorf("models: invalid movement pattern %q (allowed: push, pull, hinge, squat, carry, ground)", p)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("models: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM exercise_movement_patterns WHERE exercise_id = ?`, exerciseID); err != nil {
		return fmt.Errorf("models: clear movement patterns %d: %w", exerciseID, err)
	}
	for _, p := range patterns {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO exercise_movement_patterns (exercise_id, pattern) VALUES (?, ?)`,
			exerciseID, p,
		); err != nil {
			return fmt.Errorf("models: insert movement pattern %q for %d: %w", p, exerciseID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("models: commit movement patterns: %w", err)
	}
	return nil
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
//
// Uses a single query with window functions instead of N+1 queries per exercise.
func ListFeaturedLifts(ctx context.Context, db *sql.DB, athleteID int64) ([]*FeaturedLift, error) {
	rows, err := db.QueryContext(ctx, `
		WITH current_tms AS (
			SELECT tm.exercise_id, tm.id AS tm_id, tm.weight AS tm_weight,
			       tm.effective_date AS tm_date, tm.notes AS tm_notes, tm.created_at AS tm_created,
			       ROW_NUMBER() OVER (PARTITION BY tm.exercise_id ORDER BY tm.effective_date DESC) AS rn
			FROM training_maxes tm
			WHERE tm.athlete_id = ?
		),
		best_sets AS (
			SELECT ws.exercise_id, ws.weight AS best_weight, ws.reps AS best_reps, w.date AS best_date,
			       ROW_NUMBER() OVER (PARTITION BY ws.exercise_id ORDER BY ws.weight DESC, ws.reps DESC) AS rn
			FROM workout_sets ws
			JOIN workouts w ON w.id = ws.workout_id
			WHERE w.athlete_id = ? AND ws.weight IS NOT NULL AND ws.weight > 0
		)
		SELECT e.id, e.name,
		       ct.tm_id, ct.tm_weight, ct.tm_date, ct.tm_notes, ct.tm_created,
		       bs.best_weight, bs.best_reps, bs.best_date
		FROM exercises e
		LEFT JOIN current_tms ct ON ct.exercise_id = e.id AND ct.rn = 1
		LEFT JOIN best_sets bs ON bs.exercise_id = e.id AND bs.rn = 1
		WHERE e.featured = 1
		  AND (ct.tm_id IS NOT NULL OR bs.best_weight IS NOT NULL)
		ORDER BY e.name COLLATE NOCASE
		LIMIT 50`, athleteID, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list featured lifts: %w", err)
	}
	defer rows.Close()

	var lifts []*FeaturedLift
	for rows.Next() {
		lift := &FeaturedLift{}

		var tmID sql.NullInt64
		var tmWeight sql.NullFloat64
		var tmDate sql.NullString
		var tmNotes sql.NullString
		var tmCreated sql.NullTime

		var bestReps sql.NullInt64
		var bestDate sql.NullString

		if err := rows.Scan(
			&lift.ExerciseID, &lift.ExerciseName,
			&tmID, &tmWeight, &tmDate, &tmNotes, &tmCreated,
			&lift.BestWeight, &bestReps, &bestDate,
		); err != nil {
			return nil, fmt.Errorf("models: scan featured lift: %w", err)
		}

		if bestReps.Valid {
			lift.BestReps = int(bestReps.Int64)
		}
		if bestDate.Valid {
			lift.BestDate = bestDate.String
		}

		// Populate current TM if present.
		if tmID.Valid {
			lift.CurrentTM = &TrainingMax{
				ID:            tmID.Int64,
				AthleteID:     athleteID,
				ExerciseID:    lift.ExerciseID,
				Weight:        tmWeight.Float64,
				EffectiveDate: tmDate.String,
				Notes:         tmNotes,
				CreatedAt:     tmCreated.Time,
				ExerciseName:  lift.ExerciseName,
			}
		}

		// Estimated 1RM via Epley formula: weight × (1 + reps/30).
		if lift.BestWeight.Valid && lift.BestWeight.Float64 > 0 && lift.BestReps > 0 {
			if lift.BestReps == 1 {
				lift.Estimated1RM = lift.BestWeight.Float64
			} else {
				lift.Estimated1RM = lift.BestWeight.Float64 * (1 + float64(lift.BestReps)/30.0)
			}
		}

		lifts = append(lifts, lift)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate featured lifts: %w", err)
	}

	return lifts, nil
}
