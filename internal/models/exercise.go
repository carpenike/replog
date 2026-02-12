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

// Exercise represents a movement tracked in the system.
type Exercise struct {
	ID         int64
	Name       string
	Tier       sql.NullString
	TargetReps sql.NullInt64
	FormNotes  sql.NullString
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateExercise inserts a new exercise.
func CreateExercise(db *sql.DB, name, tier string, targetReps int, formNotes string) (*Exercise, error) {
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var repsVal sql.NullInt64
	if targetReps > 0 {
		repsVal = sql.NullInt64{Int64: int64(targetReps), Valid: true}
	}
	var notesVal sql.NullString
	if formNotes != "" {
		notesVal = sql.NullString{String: formNotes, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO exercises (name, tier, target_reps, form_notes) VALUES (?, ?, ?, ?)`,
		name, tierVal, repsVal, notesVal,
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
		`SELECT id, name, tier, target_reps, form_notes, created_at, updated_at
		 FROM exercises WHERE id = ?`, id,
	).Scan(&e.ID, &e.Name, &e.Tier, &e.TargetReps, &e.FormNotes, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get exercise %d: %w", id, err)
	}
	return e, nil
}

// UpdateExercise modifies an existing exercise's fields.
func UpdateExercise(db *sql.DB, id int64, name, tier string, targetReps int, formNotes string) (*Exercise, error) {
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var repsVal sql.NullInt64
	if targetReps > 0 {
		repsVal = sql.NullInt64{Int64: int64(targetReps), Valid: true}
	}
	var notesVal sql.NullString
	if formNotes != "" {
		notesVal = sql.NullString{String: formNotes, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE exercises SET name = ?, tier = ?, target_reps = ?, form_notes = ? WHERE id = ?`,
		name, tierVal, repsVal, notesVal, id,
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
		query = `SELECT id, name, tier, target_reps, form_notes, created_at, updated_at
		         FROM exercises WHERE tier IS NULL ORDER BY name COLLATE NOCASE`
	} else if tierFilter != "" {
		query = `SELECT id, name, tier, target_reps, form_notes, created_at, updated_at
		         FROM exercises WHERE tier = ? ORDER BY name COLLATE NOCASE`
		args = append(args, tierFilter)
	} else {
		query = `SELECT id, name, tier, target_reps, form_notes, created_at, updated_at
		         FROM exercises ORDER BY name COLLATE NOCASE`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("models: list exercises: %w", err)
	}
	defer rows.Close()

	var exercises []*Exercise
	for rows.Next() {
		e := &Exercise{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Tier, &e.TargetReps, &e.FormNotes, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("models: scan exercise: %w", err)
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}
