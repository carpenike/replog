package models

import (
	"database/sql"
	"fmt"
)

// PrescribedSet represents one prescribed set within a program template.
type PrescribedSet struct {
	ID             int64
	TemplateID     int64
	ExerciseID     int64
	Week           int
	Day            int
	SetNumber      int
	Reps           sql.NullInt64   // NULL = AMRAP
	Percentage     sql.NullFloat64 // of training max, NULL for bodyweight/accessories
	AbsoluteWeight sql.NullFloat64 // fixed weight (lbs/kg), NULL when using percentage
	SortOrder      int             // display order within a day (lower = first)
	RepType        string          // "reps", "each_side", "seconds", or "distance"
	Notes          sql.NullString

	// Joined fields.
	ExerciseName string

	// TargetWeight is the per-set target weight computed from percentage × training max
	// or from absolute_weight. Populated by GetPrescription; not stored in the database.
	TargetWeight *float64
}

// TargetWeightLabel returns the formatted target weight for this set, or "" if none.
func (ps *PrescribedSet) TargetWeightLabel() string {
	if ps.TargetWeight == nil {
		return ""
	}
	w := *ps.TargetWeight
	if w == float64(int(w)) {
		return fmt.Sprintf("%.0f", w)
	}
	return fmt.Sprintf("%.1f", w)
}

// AbsoluteWeightLabel returns the formatted absolute weight for this set, or "" if none.
func (ps *PrescribedSet) AbsoluteWeightLabel() string {
	if !ps.AbsoluteWeight.Valid {
		return ""
	}
	w := ps.AbsoluteWeight.Float64
	if w == float64(int(w)) {
		return fmt.Sprintf("%.0f", w)
	}
	return fmt.Sprintf("%.1f", w)
}

// PercentageLabel returns a formatted percentage string (e.g. "75%"), or "" if none.
func (ps *PrescribedSet) PercentageLabel() string {
	if !ps.Percentage.Valid {
		return ""
	}
	pct := ps.Percentage.Float64
	if pct == float64(int(pct)) {
		return fmt.Sprintf("%.0f%%", pct)
	}
	return fmt.Sprintf("%.1f%%", pct)
}

// RepsLabel returns a display string for reps (e.g. "5", "5/ea", "30s", "30yd", or "AMRAP").
func (ps *PrescribedSet) RepsLabel() string {
	if !ps.Reps.Valid {
		return "AMRAP"
	}
	switch ps.RepType {
	case "each_side":
		return fmt.Sprintf("%d/ea", ps.Reps.Int64)
	case "seconds":
		return fmt.Sprintf("%ds", ps.Reps.Int64)
	case "distance":
		return fmt.Sprintf("%dyd", ps.Reps.Int64)
	default:
		return fmt.Sprintf("%d", ps.Reps.Int64)
	}
}

// CreatePrescribedSet inserts a new prescribed set into a program template.
func CreatePrescribedSet(db *sql.DB, templateID, exerciseID int64, week, day, setNumber int, reps *int, percentage *float64, absoluteWeight *float64, sortOrder int, repType, notes string) (*PrescribedSet, error) {
	var repsVal sql.NullInt64
	if reps != nil {
		repsVal = sql.NullInt64{Int64: int64(*reps), Valid: true}
	}
	var pctVal sql.NullFloat64
	if percentage != nil {
		pctVal = sql.NullFloat64{Float64: *percentage, Valid: true}
	}
	var absWeightVal sql.NullFloat64
	if absoluteWeight != nil {
		absWeightVal = sql.NullFloat64{Float64: *absoluteWeight, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	if repType == "" {
		repType = "reps"
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO prescribed_sets (template_id, exercise_id, week, day, set_number, reps, percentage, absolute_weight, sort_order, rep_type, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		templateID, exerciseID, week, day, setNumber, repsVal, pctVal, absWeightVal, sortOrder, repType, notesVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: prescribed set already exists for week %d day %d set %d", week, day, setNumber)
		}
		return nil, fmt.Errorf("models: create prescribed set: %w", err)
	}

	return GetPrescribedSetByID(db, id)
}

// GetPrescribedSetByID retrieves a prescribed set by primary key.
func GetPrescribedSetByID(db *sql.DB, id int64) (*PrescribedSet, error) {
	ps := &PrescribedSet{}
	err := db.QueryRow(
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.absolute_weight, ps.sort_order, ps.rep_type, ps.notes, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.id = ?`,
		id,
	).Scan(&ps.ID, &ps.TemplateID, &ps.ExerciseID, &ps.Week, &ps.Day, &ps.SetNumber,
		&ps.Reps, &ps.Percentage, &ps.AbsoluteWeight, &ps.SortOrder, &ps.RepType, &ps.Notes, &ps.ExerciseName)
	if err != nil {
		return nil, fmt.Errorf("models: get prescribed set %d: %w", id, err)
	}
	return ps, nil
}

// ListPrescribedSets returns all prescribed sets for a template, ordered by week, day, sort_order, set_number.
func ListPrescribedSets(db *sql.DB, templateID int64) ([]*PrescribedSet, error) {
	rows, err := db.Query(
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.absolute_weight, ps.sort_order, ps.rep_type, ps.notes, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.template_id = ?
		 ORDER BY ps.week, ps.day, ps.sort_order, e.name COLLATE NOCASE, ps.set_number`,
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
			&ps.Reps, &ps.Percentage, &ps.AbsoluteWeight, &ps.SortOrder, &ps.RepType, &ps.Notes, &ps.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan prescribed set: %w", err)
		}
		sets = append(sets, ps)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate prescribed sets: %w", err)
	}
	return sets, nil
}

// ListPrescribedSetsForDay returns prescribed sets for a specific week/day.
func ListPrescribedSetsForDay(db *sql.DB, templateID int64, week, day int) ([]*PrescribedSet, error) {
	rows, err := db.Query(
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.absolute_weight, ps.sort_order, ps.rep_type, ps.notes, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.template_id = ? AND ps.week = ? AND ps.day = ?
		 ORDER BY ps.sort_order, e.name COLLATE NOCASE, ps.set_number`,
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
			&ps.Reps, &ps.Percentage, &ps.AbsoluteWeight, &ps.SortOrder, &ps.RepType, &ps.Notes, &ps.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan prescribed set: %w", err)
		}
		sets = append(sets, ps)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate prescribed sets for day: %w", err)
	}
	return sets, nil
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

// UpdatePrescribedSet updates an existing prescribed set's fields.
func UpdatePrescribedSet(db *sql.DB, id int64, exerciseID int64, setNumber int, reps *int, percentage *float64, absoluteWeight *float64, sortOrder int, repType, notes string) (*PrescribedSet, error) {
	var repsVal sql.NullInt64
	if reps != nil {
		repsVal = sql.NullInt64{Int64: int64(*reps), Valid: true}
	}
	var pctVal sql.NullFloat64
	if percentage != nil {
		pctVal = sql.NullFloat64{Float64: *percentage, Valid: true}
	}
	var absWeightVal sql.NullFloat64
	if absoluteWeight != nil {
		absWeightVal = sql.NullFloat64{Float64: *absoluteWeight, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	if repType == "" {
		repType = "reps"
	}

	_, err := db.Exec(
		`UPDATE prescribed_sets
		 SET exercise_id = ?, set_number = ?, reps = ?, percentage = ?, absolute_weight = ?, sort_order = ?, rep_type = ?, notes = ?
		 WHERE id = ?`,
		exerciseID, setNumber, repsVal, pctVal, absWeightVal, sortOrder, repType, notesVal, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: prescribed set conflicts with existing set")
		}
		return nil, fmt.Errorf("models: update prescribed set %d: %w", id, err)
	}

	return GetPrescribedSetByID(db, id)
}

// CopyWeek replaces all prescribed sets in targetWeek with copies from
// sourceWeek within the same program template. Any existing sets in the
// target week are deleted first. Returns the number of sets inserted.
func CopyWeek(db *sql.DB, templateID int64, sourceWeek, targetWeek int) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("models: copy week begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete existing sets in target week.
	_, err = tx.Exec(
		`DELETE FROM prescribed_sets WHERE template_id = ? AND week = ?`,
		templateID, targetWeek,
	)
	if err != nil {
		return 0, fmt.Errorf("models: copy week delete target: %w", err)
	}

	rows, err := tx.Query(
		`SELECT day, exercise_id, set_number, reps, percentage,
		        absolute_weight, sort_order, rep_type, notes
		   FROM prescribed_sets
		  WHERE template_id = ? AND week = ?
		  ORDER BY day, sort_order`,
		templateID, sourceWeek,
	)
	if err != nil {
		return 0, fmt.Errorf("models: copy week query source: %w", err)
	}
	defer rows.Close()

	type setRow struct {
		day            int
		exerciseID     int64
		setNumber      int
		reps           sql.NullInt64
		percentage     sql.NullFloat64
		absoluteWeight sql.NullFloat64
		sortOrder      int
		repType        string
		notes          sql.NullString
	}
	var sets []setRow
	for rows.Next() {
		var s setRow
		if err := rows.Scan(&s.day, &s.exerciseID, &s.setNumber,
			&s.reps, &s.percentage, &s.absoluteWeight,
			&s.sortOrder, &s.repType, &s.notes); err != nil {
			return 0, fmt.Errorf("models: copy week scan: %w", err)
		}
		sets = append(sets, s)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("models: copy week rows: %w", err)
	}

	inserted := 0
	for _, s := range sets {
		_, err := tx.Exec(
			`INSERT INTO prescribed_sets
			   (template_id, week, day, exercise_id, set_number,
			    reps, percentage, absolute_weight, sort_order, rep_type, notes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			templateID, targetWeek, s.day, s.exerciseID, s.setNumber,
			s.reps, s.percentage, s.absoluteWeight, s.sortOrder, s.repType, s.notes,
		)
		if err != nil {
			return 0, fmt.Errorf("models: copy week insert: %w", err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("models: copy week commit: %w", err)
	}
	return inserted, nil
}
