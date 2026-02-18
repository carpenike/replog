package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrDuplicateTrainingMax is returned when a training max already exists for
// the given athlete+exercise+date combination.
var ErrDuplicateTrainingMax = errors.New("training max already exists for this date")

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

	var id int64
	err := db.QueryRow(
		`INSERT INTO training_maxes (athlete_id, exercise_id, weight, effective_date, notes) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		athleteID, exerciseID, weight, effectiveDate, notesVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateTrainingMax
		}
		return nil, fmt.Errorf("models: set training max: %w", err)
	}

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
		ORDER BY tm.effective_date DESC
		LIMIT 100`, athleteID, exerciseID)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate training max history: %w", err)
	}
	return maxes, nil
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
		ORDER BY e.name COLLATE NOCASE
		LIMIT 100`, athleteID)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate current training maxes: %w", err)
	}
	return maxes, nil
}

// ProgramExerciseTM pairs an exercise from a program template with the athlete's
// current training max (if any). Used for the post-assign TM setup form.
type ProgramExerciseTM struct {
	ExerciseID   int64
	ExerciseName string
	CurrentTM    *float64 // nil if no TM set
}

// ListProgramExerciseTMs returns all distinct exercises in a program template,
// each paired with the athlete's current TM (if one exists). Results are
// ordered by exercise name.
func ListProgramExerciseTMs(db *sql.DB, templateID, athleteID int64) ([]*ProgramExerciseTM, error) {
	rows, err := db.Query(`
		SELECT DISTINCT e.id, e.name,
		       (SELECT tm.weight FROM training_maxes tm
		        WHERE tm.athlete_id = ? AND tm.exercise_id = e.id
		        ORDER BY tm.effective_date DESC LIMIT 1) AS current_tm
		FROM prescribed_sets ps
		JOIN exercises e ON e.id = ps.exercise_id
		WHERE ps.template_id = ?
		ORDER BY e.name COLLATE NOCASE`,
		athleteID, templateID)
	if err != nil {
		return nil, fmt.Errorf("models: list program exercise TMs (template=%d, athlete=%d): %w", templateID, athleteID, err)
	}
	defer rows.Close()

	var results []*ProgramExerciseTM
	for rows.Next() {
		pet := &ProgramExerciseTM{}
		var tm sql.NullFloat64
		if err := rows.Scan(&pet.ExerciseID, &pet.ExerciseName, &tm); err != nil {
			return nil, fmt.Errorf("models: scan program exercise TM: %w", err)
		}
		if tm.Valid {
			pet.CurrentTM = &tm.Float64
		}
		results = append(results, pet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate program exercise TMs: %w", err)
	}
	return results, nil
}

// MissingProgramTM identifies a program exercise that uses percentage-based
// sets but has no training max defined for the athlete.
type MissingProgramTM struct {
	ExerciseID   int64
	ExerciseName string
}

// ListMissingProgramTMs returns exercises in a program template that have at
// least one percentage-based prescribed set but the athlete has no current
// training max for them. Only percentage-based sets need a TM; exercises
// with only absolute_weight sets are excluded.
func ListMissingProgramTMs(db *sql.DB, templateID, athleteID int64) ([]*MissingProgramTM, error) {
	rows, err := db.Query(`
		SELECT DISTINCT e.id, e.name
		FROM prescribed_sets ps
		JOIN exercises e ON e.id = ps.exercise_id
		WHERE ps.template_id = ?
		  AND ps.percentage IS NOT NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM training_maxes tm
		      WHERE tm.athlete_id = ? AND tm.exercise_id = e.id
		  )
		ORDER BY e.name COLLATE NOCASE`,
		templateID, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list missing program TMs (template=%d, athlete=%d): %w", templateID, athleteID, err)
	}
	defer rows.Close()

	var missing []*MissingProgramTM
	for rows.Next() {
		m := &MissingProgramTM{}
		if err := rows.Scan(&m.ExerciseID, &m.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan missing program TM: %w", err)
		}
		missing = append(missing, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate missing program TMs: %w", err)
	}
	return missing, nil
}