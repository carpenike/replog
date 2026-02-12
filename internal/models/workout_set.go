package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// WorkoutSet represents a single set logged within a workout.
type WorkoutSet struct {
	ID         int64
	WorkoutID  int64
	ExerciseID int64
	SetNumber  int
	Reps       int
	Weight     sql.NullFloat64
	Notes      sql.NullString
	CreatedAt  time.Time

	// Joined fields populated by list queries.
	ExerciseName string
}

// AddSet inserts a new set into a workout. set_number is auto-calculated as the
// next number for the given workout+exercise.
func AddSet(db *sql.DB, workoutID, exerciseID int64, reps int, weight float64, notes string) (*WorkoutSet, error) {
	var weightVal sql.NullFloat64
	if weight > 0 {
		weightVal = sql.NullFloat64{Float64: weight, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	// Compute next set_number for this workout+exercise.
	var nextSet int
	err := db.QueryRow(
		`SELECT COALESCE(MAX(set_number), 0) + 1 FROM workout_sets WHERE workout_id = ? AND exercise_id = ?`,
		workoutID, exerciseID,
	).Scan(&nextSet)
	if err != nil {
		return nil, fmt.Errorf("models: compute next set number: %w", err)
	}

	result, err := db.Exec(
		`INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps, weight, notes) VALUES (?, ?, ?, ?, ?, ?)`,
		workoutID, exerciseID, nextSet, reps, weightVal, notesVal,
	)
	if err != nil {
		return nil, fmt.Errorf("models: add set to workout %d: %w", workoutID, err)
	}

	id, _ := result.LastInsertId()
	return GetSetByID(db, id)
}

// GetSetByID retrieves a workout set by primary key.
func GetSetByID(db *sql.DB, id int64) (*WorkoutSet, error) {
	s := &WorkoutSet{}
	err := db.QueryRow(
		`SELECT ws.id, ws.workout_id, ws.exercise_id, ws.set_number, ws.reps, ws.weight, ws.notes, ws.created_at,
		        e.name
		 FROM workout_sets ws
		 JOIN exercises e ON e.id = ws.exercise_id
		 WHERE ws.id = ?`, id,
	).Scan(&s.ID, &s.WorkoutID, &s.ExerciseID, &s.SetNumber, &s.Reps, &s.Weight, &s.Notes, &s.CreatedAt, &s.ExerciseName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get set %d: %w", id, err)
	}
	return s, nil
}

// UpdateSet updates a set's reps, weight, and notes.
func UpdateSet(db *sql.DB, id int64, reps int, weight float64, notes string) (*WorkoutSet, error) {
	var weightVal sql.NullFloat64
	if weight > 0 {
		weightVal = sql.NullFloat64{Float64: weight, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE workout_sets SET reps = ?, weight = ?, notes = ? WHERE id = ?`,
		reps, weightVal, notesVal, id,
	)
	if err != nil {
		return nil, fmt.Errorf("models: update set %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, ErrNotFound
	}
	return GetSetByID(db, id)
}

// DeleteSet removes a set from a workout.
func DeleteSet(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM workout_sets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete set %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ExerciseGroup groups sets by exercise for a workout detail view.
type ExerciseGroup struct {
	ExerciseID   int64
	ExerciseName string
	Sets         []*WorkoutSet
}

// ListSetsByWorkout returns all sets for a workout, grouped by exercise.
func ListSetsByWorkout(db *sql.DB, workoutID int64) ([]*ExerciseGroup, error) {
	rows, err := db.Query(`
		SELECT ws.id, ws.workout_id, ws.exercise_id, ws.set_number, ws.reps, ws.weight, ws.notes, ws.created_at,
		       e.name
		FROM workout_sets ws
		JOIN exercises e ON e.id = ws.exercise_id
		WHERE ws.workout_id = ?
		ORDER BY e.name COLLATE NOCASE, ws.set_number`, workoutID)
	if err != nil {
		return nil, fmt.Errorf("models: list sets for workout %d: %w", workoutID, err)
	}
	defer rows.Close()

	groupMap := make(map[int64]*ExerciseGroup)
	var groupOrder []int64

	for rows.Next() {
		s := &WorkoutSet{}
		if err := rows.Scan(&s.ID, &s.WorkoutID, &s.ExerciseID, &s.SetNumber, &s.Reps, &s.Weight, &s.Notes, &s.CreatedAt, &s.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan set: %w", err)
		}

		g, exists := groupMap[s.ExerciseID]
		if !exists {
			g = &ExerciseGroup{
				ExerciseID:   s.ExerciseID,
				ExerciseName: s.ExerciseName,
			}
			groupMap[s.ExerciseID] = g
			groupOrder = append(groupOrder, s.ExerciseID)
		}
		g.Sets = append(g.Sets, s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return in stable order.
	groups := make([]*ExerciseGroup, 0, len(groupOrder))
	for _, eid := range groupOrder {
		groups = append(groups, groupMap[eid])
	}
	return groups, nil
}
