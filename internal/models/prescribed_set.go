package models

import (
	"database/sql"
	"fmt"
)

// PrescribedSet represents one prescribed set within a program template.
type PrescribedSet struct {
	ID         int64
	TemplateID int64
	ExerciseID int64
	Week       int
	Day        int
	SetNumber  int
	Reps       sql.NullInt64   // NULL = AMRAP
	Percentage sql.NullFloat64 // of training max, NULL for bodyweight/accessories
	Notes      sql.NullString

	// Joined fields.
	ExerciseName string
}

// RepsLabel returns a display string for reps (e.g. "5" or "AMRAP").
func (ps *PrescribedSet) RepsLabel() string {
	if !ps.Reps.Valid {
		return "AMRAP"
	}
	return fmt.Sprintf("%d", ps.Reps.Int64)
}

// CreatePrescribedSet inserts a new prescribed set into a program template.
func CreatePrescribedSet(db *sql.DB, templateID, exerciseID int64, week, day, setNumber int, reps *int, percentage *float64, notes string) (*PrescribedSet, error) {
	var repsVal sql.NullInt64
	if reps != nil {
		repsVal = sql.NullInt64{Int64: int64(*reps), Valid: true}
	}
	var pctVal sql.NullFloat64
	if percentage != nil {
		pctVal = sql.NullFloat64{Float64: *percentage, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO prescribed_sets (template_id, exercise_id, week, day, set_number, reps, percentage, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		templateID, exerciseID, week, day, setNumber, repsVal, pctVal, notesVal,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: prescribed set already exists for week %d day %d set %d", week, day, setNumber)
		}
		return nil, fmt.Errorf("models: create prescribed set: %w", err)
	}

	id, _ := result.LastInsertId()
	return GetPrescribedSetByID(db, id)
}

// GetPrescribedSetByID retrieves a prescribed set by primary key.
func GetPrescribedSetByID(db *sql.DB, id int64) (*PrescribedSet, error) {
	ps := &PrescribedSet{}
	err := db.QueryRow(
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.notes, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.id = ?`,
		id,
	).Scan(&ps.ID, &ps.TemplateID, &ps.ExerciseID, &ps.Week, &ps.Day, &ps.SetNumber,
		&ps.Reps, &ps.Percentage, &ps.Notes, &ps.ExerciseName)
	if err != nil {
		return nil, fmt.Errorf("models: get prescribed set %d: %w", id, err)
	}
	return ps, nil
}

// ListPrescribedSets returns all prescribed sets for a template, ordered by week, day, exercise, set_number.
func ListPrescribedSets(db *sql.DB, templateID int64) ([]*PrescribedSet, error) {
	rows, err := db.Query(
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.notes, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.template_id = ?
		 ORDER BY ps.week, ps.day, e.name COLLATE NOCASE, ps.set_number`,
		templateID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list prescribed sets for template %d: %w", templateID, err)
	}
	defer rows.Close()

	var sets []*PrescribedSet
	for rows.Next() {
		ps := &PrescribedSet{}
		if err := rows.Scan(&ps.ID, &ps.TemplateID, &ps.ExerciseID, &ps.Week, &ps.Day, &ps.SetNumber,
			&ps.Reps, &ps.Percentage, &ps.Notes, &ps.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan prescribed set: %w", err)
		}
		sets = append(sets, ps)
	}
	return sets, rows.Err()
}

// ListPrescribedSetsForDay returns prescribed sets for a specific week/day.
func ListPrescribedSetsForDay(db *sql.DB, templateID int64, week, day int) ([]*PrescribedSet, error) {
	rows, err := db.Query(
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.notes, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.template_id = ? AND ps.week = ? AND ps.day = ?
		 ORDER BY e.name COLLATE NOCASE, ps.set_number`,
		templateID, week, day,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list prescribed sets for template %d week %d day %d: %w", templateID, week, day, err)
	}
	defer rows.Close()

	var sets []*PrescribedSet
	for rows.Next() {
		ps := &PrescribedSet{}
		if err := rows.Scan(&ps.ID, &ps.TemplateID, &ps.ExerciseID, &ps.Week, &ps.Day, &ps.SetNumber,
			&ps.Reps, &ps.Percentage, &ps.Notes, &ps.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan prescribed set: %w", err)
		}
		sets = append(sets, ps)
	}
	return sets, rows.Err()
}

// DeletePrescribedSet removes a prescribed set.
func DeletePrescribedSet(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM prescribed_sets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete prescribed set %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("models: prescribed set %d not found", id)
	}
	return nil
}
