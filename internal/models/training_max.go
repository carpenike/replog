package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TrainingMax represents a training max record for an athlete+exercise.
type TrainingMax struct {
	ID            int64
	AthleteID     int64
	ExerciseID    int64
	Weight        float64
	EffectiveDate string // DATE as string (YYYY-MM-DD)
	Notes         sql.NullString
	CreatedAt     time.Time

	// Joined fields populated by list queries.
	ExerciseName string
}

// SetTrainingMax inserts a new training max row. Each row is a historical record;
// the current TM is the one with the latest effective_date.
func SetTrainingMax(db *sql.DB, athleteID, exerciseID int64, weight float64, effectiveDate, notes string) (*TrainingMax, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO training_maxes (athlete_id, exercise_id, weight, effective_date, notes) VALUES (?, ?, ?, ?, ?)`,
		athleteID, exerciseID, weight, effectiveDate, notesVal,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: training max already exists for that date: %w", err)
		}
		return nil, fmt.Errorf("models: set training max: %w", err)
	}

	id, _ := result.LastInsertId()
	return GetTrainingMaxByID(db, id)
}

// GetTrainingMaxByID retrieves a training max by primary key.
func GetTrainingMaxByID(db *sql.DB, id int64) (*TrainingMax, error) {
	tm := &TrainingMax{}
	err := db.QueryRow(
		`SELECT tm.id, tm.athlete_id, tm.exercise_id, tm.weight, tm.effective_date, tm.notes, tm.created_at,
		        e.name
		 FROM training_maxes tm
		 JOIN exercises e ON e.id = tm.exercise_id
		 WHERE tm.id = ?`, id,
	).Scan(&tm.ID, &tm.AthleteID, &tm.ExerciseID, &tm.Weight, &tm.EffectiveDate, &tm.Notes, &tm.CreatedAt, &tm.ExerciseName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get training max %d: %w", id, err)
	}
	return tm, nil
}

// CurrentTrainingMax returns the most recent training max for an athlete+exercise.
func CurrentTrainingMax(db *sql.DB, athleteID, exerciseID int64) (*TrainingMax, error) {
	tm := &TrainingMax{}
	err := db.QueryRow(
		`SELECT tm.id, tm.athlete_id, tm.exercise_id, tm.weight, tm.effective_date, tm.notes, tm.created_at,
		        e.name
		 FROM training_maxes tm
		 JOIN exercises e ON e.id = tm.exercise_id
		 WHERE tm.athlete_id = ? AND tm.exercise_id = ?
		 ORDER BY tm.effective_date DESC LIMIT 1`, athleteID, exerciseID,
	).Scan(&tm.ID, &tm.AthleteID, &tm.ExerciseID, &tm.Weight, &tm.EffectiveDate, &tm.Notes, &tm.CreatedAt, &tm.ExerciseName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: current training max for athlete %d exercise %d: %w", athleteID, exerciseID, err)
	}
	return tm, nil
}

// ListTrainingMaxHistory returns all training max records for an athlete+exercise,
// ordered by effective_date descending (most recent first).
func ListTrainingMaxHistory(db *sql.DB, athleteID, exerciseID int64) ([]*TrainingMax, error) {
	rows, err := db.Query(`
		SELECT tm.id, tm.athlete_id, tm.exercise_id, tm.weight, tm.effective_date, tm.notes, tm.created_at,
		       e.name
		FROM training_maxes tm
		JOIN exercises e ON e.id = tm.exercise_id
		WHERE tm.athlete_id = ? AND tm.exercise_id = ?
		ORDER BY tm.effective_date DESC`, athleteID, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("models: list training max history: %w", err)
	}
	defer rows.Close()

	var maxes []*TrainingMax
	for rows.Next() {
		tm := &TrainingMax{}
		if err := rows.Scan(&tm.ID, &tm.AthleteID, &tm.ExerciseID, &tm.Weight, &tm.EffectiveDate, &tm.Notes, &tm.CreatedAt, &tm.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan training max: %w", err)
		}
		maxes = append(maxes, tm)
	}
	return maxes, rows.Err()
}

// ListCurrentTrainingMaxes returns the current (latest) training max for each
// exercise assigned to an athlete.
func ListCurrentTrainingMaxes(db *sql.DB, athleteID int64) ([]*TrainingMax, error) {
	rows, err := db.Query(`
		SELECT tm.id, tm.athlete_id, tm.exercise_id, tm.weight, tm.effective_date, tm.notes, tm.created_at,
		       e.name
		FROM training_maxes tm
		JOIN exercises e ON e.id = tm.exercise_id
		WHERE tm.athlete_id = ?
		  AND tm.effective_date = (
		      SELECT MAX(tm2.effective_date)
		      FROM training_maxes tm2
		      WHERE tm2.athlete_id = tm.athlete_id AND tm2.exercise_id = tm.exercise_id
		  )
		ORDER BY e.name COLLATE NOCASE`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list current training maxes for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var maxes []*TrainingMax
	for rows.Next() {
		tm := &TrainingMax{}
		if err := rows.Scan(&tm.ID, &tm.AthleteID, &tm.ExerciseID, &tm.Weight, &tm.EffectiveDate, &tm.Notes, &tm.CreatedAt, &tm.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan training max: %w", err)
		}
		maxes = append(maxes, tm)
	}
	return maxes, rows.Err()
}
