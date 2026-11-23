package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	RestSeconds    sql.NullInt64   // per-set timer override, NULL uses the exercise default
	PrescribedRPE  sql.NullFloat64 // target RPE, NULL when the set has no prescribed effort
	GroupLabel     sql.NullString  // optional grouping label (for example "T1")
	RepsMax        sql.NullInt64   // top of a rep range ("8–12"); Reps is the bottom. NULL = fixed reps
	RIRMin         sql.NullInt64   // reps in reserve, low end; NULL when not prescribed
	RIRMax         sql.NullInt64   // reps in reserve, high end; NULL when not prescribed
	NonWorking     bool            // warm-up / non-working set: excluded from progress and PRs
	GroupKey       sql.NullString  // superset/circuit group identity (A, B, ...); NULL = not grouped
	SortOrder      int             // display order within a day (lower = first)
	RepType        string          // "reps", "each_side", "seconds", or "distance"
	Notes          sql.NullString

	// Joined fields.
	ExerciseName string

	// TargetWeight is the per-set target weight computed from percentage × training max
	// or from absolute_weight. Populated by GetPrescription; not stored in the database.
	TargetWeight *float64
}

// RepsLabel returns a display string for reps (e.g. "5", "5/ea", "30s", "30yd", or "AMRAP").
func (ps *PrescribedSet) RepsLabel() string {
	if !ps.Reps.Valid {
		return "AMRAP"
	}
	reps := fmt.Sprintf("%d", ps.Reps.Int64)
	if ps.RepsMax.Valid && ps.RepsMax.Int64 > ps.Reps.Int64 {
		reps = fmt.Sprintf("%d-%d", ps.Reps.Int64, ps.RepsMax.Int64)
	}
	switch ps.RepType {
	case "each_side":
		return reps + "/ea"
	case "seconds":
		return reps + "s"
	case "distance":
		return reps + "yd"
	default:
		return reps
	}
}

// CreatePrescribedSet inserts a new prescribed set into a program template.
func CreatePrescribedSet(ctx context.Context, db *sql.DB, templateID, exerciseID int64, week, day, setNumber int, reps *int, percentage *float64, absoluteWeight *float64, sortOrder int, repType, notes string) (*PrescribedSet, error) {
	return CreatePrescribedSetWithRest(ctx, db, templateID, exerciseID, week, day, setNumber, reps, percentage, absoluteWeight, nil, sortOrder, repType, notes, nil, "")
}

// CreatePrescribedSetWithRest inserts a new prescribed set with an optional
// per-set rest timer override, prescribed RPE, and group label.
func CreatePrescribedSetWithRest(ctx context.Context, db *sql.DB, templateID, exerciseID int64, week, day, setNumber int, reps *int, percentage *float64, absoluteWeight *float64, restSeconds *int, sortOrder int, repType, notes string, prescribedRPE *float64, groupLabel string) (*PrescribedSet, error) {
	return CreatePrescribedSetDetailed(ctx, db, templateID, exerciseID, week, day, setNumber, reps, percentage, absoluteWeight, restSeconds, sortOrder, repType, notes, prescribedRPE, groupLabel, PrescribedSetDetail{})
}

// PrescribedSetDetail is the optional prescription detail beyond the base
// columns: a rep range top, an RIR range, a warm-up flag, and a
// superset/circuit group key. The zero value prescribes none of them.
type PrescribedSetDetail struct {
	RepsMax    *int
	RIRMin     *int
	RIRMax     *int
	NonWorking bool
	GroupKey   string
}

// NormalizeGroupKey trims and upper-cases a group key. An empty result
// means the set is not grouped.
func NormalizeGroupKey(key string) string {
	return strings.ToUpper(strings.TrimSpace(key))
}

// ValidGroupKey reports whether key (already normalized) is empty or 1–4
// letters/digits starting with a letter, such as "A" or "B2".
func ValidGroupKey(key string) bool {
	if key == "" {
		return true
	}
	if len(key) > 4 || key[0] < 'A' || key[0] > 'Z' {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// GroupKeyForIndex returns the group key for the i-th (0-based) grouped
// block of a day: A, B, ..., Z, AA, AB, ...
func GroupKeyForIndex(i int) string {
	if i < 0 {
		return ""
	}
	key := ""
	for n := i; ; n = n/26 - 1 {
		key = string(rune('A'+n%26)) + key
		if n < 26 {
			break
		}
	}
	return key
}

// ValidatePrescribedSetDetail checks the optional detail against the base
// reps. It returns the offending field and a message, or "" when valid.
func ValidatePrescribedSetDetail(reps *int, d PrescribedSetDetail) (string, string) {
	if d.RepsMax != nil {
		if *d.RepsMax < 0 {
			return "reps_max", "must be zero or greater"
		}
		if reps != nil && *d.RepsMax < *reps {
			return "reps_max", "must be at least reps"
		}
	}
	if field, msg := validRIR(d.RIRMin, d.RIRMax); field != "" {
		return field, msg
	}
	if !ValidGroupKey(NormalizeGroupKey(d.GroupKey)) {
		return "group_key", "must be 1-4 letters or digits starting with a letter"
	}
	return "", ""
}

func validRIR(lo, hi *int) (string, string) {
	if lo != nil && (*lo < 0 || *lo > 10) {
		return "rir_min", "must be between 0 and 10"
	}
	if hi != nil && (*hi < 0 || *hi > 10) {
		return "rir_max", "must be between 0 and 10"
	}
	if lo != nil && hi != nil && *lo > *hi {
		return "rir_max", "must be at least rir_min"
	}
	return "", ""
}

// CreatePrescribedSetDetailed inserts a prescribed set with the optional
// rep range, RIR range, warm-up flag, and group key.
func CreatePrescribedSetDetailed(ctx context.Context, db *sql.DB, templateID, exerciseID int64, week, day, setNumber int, reps *int, percentage *float64, absoluteWeight *float64, restSeconds *int, sortOrder int, repType, notes string, prescribedRPE *float64, groupLabel string, detail PrescribedSetDetail) (*PrescribedSet, error) {
	id, err := insertPrescribedSetRow(ctx, db, templateID, exerciseID, week, day, setNumber, reps, percentage, absoluteWeight, restSeconds, sortOrder, repType, notes, prescribedRPE, groupLabel, detail)
	if err != nil {
		return nil, err
	}
	return GetPrescribedSetByID(ctx, db, id)
}

// setWriter is the subset of *sql.DB and *sql.Tx used to insert a set.
type setWriter interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func insertPrescribedSetRow(ctx context.Context, q setWriter, templateID, exerciseID int64, week, day, setNumber int, reps *int, percentage *float64, absoluteWeight *float64, restSeconds *int, sortOrder int, repType, notes string, prescribedRPE *float64, groupLabel string, detail PrescribedSetDetail) (int64, error) {
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
	var restSecondsVal sql.NullInt64
	if restSeconds != nil {
		restSecondsVal = sql.NullInt64{Int64: int64(*restSeconds), Valid: true}
	}
	var rpeVal sql.NullFloat64
	if prescribedRPE != nil {
		rpeVal = sql.NullFloat64{Float64: *prescribedRPE, Valid: true}
	}
	var groupVal sql.NullString
	if strings.TrimSpace(groupLabel) != "" {
		groupVal = sql.NullString{String: strings.TrimSpace(groupLabel), Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	if repType == "" {
		repType = "reps"
	}

	var groupKeyVal sql.NullString
	if key := NormalizeGroupKey(detail.GroupKey); key != "" {
		groupKeyVal = sql.NullString{String: key, Valid: true}
	}

	var id int64
	err := q.QueryRowContext(ctx,
		`INSERT INTO prescribed_sets
		    (template_id, exercise_id, week, day, set_number, reps, percentage, absolute_weight, rest_seconds, prescribed_rpe, group_label,
		     reps_max, rir_min, rir_max, non_working, group_key, sort_order, rep_type, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		templateID, exerciseID, week, day, setNumber, repsVal, pctVal, absWeightVal, restSecondsVal, rpeVal, groupVal,
		sqlInt(detail.RepsMax), sqlInt(detail.RIRMin), sqlInt(detail.RIRMax), boolToInt(detail.NonWorking), groupKeyVal,
		sortOrder, repType, notesVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, fmt.Errorf("models: prescribed set already exists for week %d day %d set %d", week, day, setNumber)
		}
		return 0, fmt.Errorf("models: create prescribed set: %w", err)
	}
	return id, nil
}

// GetPrescribedSetByID retrieves a prescribed set by primary key.
func GetPrescribedSetByID(ctx context.Context, db *sql.DB, id int64) (*PrescribedSet, error) {
	ps := &PrescribedSet{}
	err := db.QueryRowContext(ctx,
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.absolute_weight, ps.rest_seconds, ps.prescribed_rpe, ps.group_label, ps.reps_max, ps.rir_min, ps.rir_max, ps.non_working, ps.group_key, ps.sort_order, ps.rep_type, ps.notes, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.id = ?`,
		id,
	).Scan(&ps.ID, &ps.TemplateID, &ps.ExerciseID, &ps.Week, &ps.Day, &ps.SetNumber,
		&ps.Reps, &ps.Percentage, &ps.AbsoluteWeight, &ps.RestSeconds, &ps.PrescribedRPE, &ps.GroupLabel, &ps.RepsMax, &ps.RIRMin, &ps.RIRMax, &ps.NonWorking, &ps.GroupKey, &ps.SortOrder, &ps.RepType, &ps.Notes, &ps.ExerciseName)
	if err != nil {
		return nil, fmt.Errorf("models: get prescribed set %d: %w", id, err)
	}
	return ps, nil
}

// ListPrescribedSets returns all prescribed sets for a template, ordered by week, day, sort_order, set_number.
func ListPrescribedSets(ctx context.Context, db *sql.DB, templateID int64) ([]*PrescribedSet, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.absolute_weight, ps.rest_seconds, ps.prescribed_rpe, ps.group_label, ps.reps_max, ps.rir_min, ps.rir_max, ps.non_working, ps.group_key, ps.sort_order, ps.rep_type, ps.notes, e.name
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
			&ps.Reps, &ps.Percentage, &ps.AbsoluteWeight, &ps.RestSeconds, &ps.PrescribedRPE, &ps.GroupLabel, &ps.RepsMax, &ps.RIRMin, &ps.RIRMax, &ps.NonWorking, &ps.GroupKey, &ps.SortOrder, &ps.RepType, &ps.Notes, &ps.ExerciseName); err != nil {
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
func ListPrescribedSetsForDay(ctx context.Context, db *sql.DB, templateID int64, week, day int) ([]*PrescribedSet, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT ps.id, ps.template_id, ps.exercise_id, ps.week, ps.day, ps.set_number,
		        ps.reps, ps.percentage, ps.absolute_weight, ps.rest_seconds, ps.prescribed_rpe, ps.group_label, ps.reps_max, ps.rir_min, ps.rir_max, ps.non_working, ps.group_key, ps.sort_order, ps.rep_type, ps.notes, e.name
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
			&ps.Reps, &ps.Percentage, &ps.AbsoluteWeight, &ps.RestSeconds, &ps.PrescribedRPE, &ps.GroupLabel, &ps.RepsMax, &ps.RIRMin, &ps.RIRMax, &ps.NonWorking, &ps.GroupKey, &ps.SortOrder, &ps.RepType, &ps.Notes, &ps.ExerciseName); err != nil {
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
func DeletePrescribedSet(ctx context.Context, db *sql.DB, id int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM prescribed_sets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete prescribed set %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("models: prescribed set %d not found", id)
	}
	return nil
}

// PrescribedSetDetailPatch changes the optional prescription detail on an
// update. A nil field leaves the stored column unchanged, so an editor
// that does not know a field cannot wipe it. ClearRepsMax and ClearRIR
// set those columns to NULL; an empty non-nil GroupKey ungroups the set.
type PrescribedSetDetailPatch struct {
	RepsMax      *int
	ClearRepsMax bool
	RIRMin       *int
	RIRMax       *int
	ClearRIR     bool
	NonWorking   *bool
	GroupKey     *string
}

// UpdatePrescribedSet updates an existing prescribed set's fields. The
// base fields are overwritten; the detail fields follow patch.
func UpdatePrescribedSet(ctx context.Context, db *sql.DB, id int64, exerciseID int64, setNumber int, reps *int, percentage *float64, absoluteWeight *float64, restSeconds *int, sortOrder int, repType, notes string, prescribedRPE *float64, groupLabel string, patch PrescribedSetDetailPatch) (*PrescribedSet, error) {
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
	var restSecondsVal sql.NullInt64
	if restSeconds != nil {
		restSecondsVal = sql.NullInt64{Int64: int64(*restSeconds), Valid: true}
	}
	var rpeVal sql.NullFloat64
	if prescribedRPE != nil {
		rpeVal = sql.NullFloat64{Float64: *prescribedRPE, Valid: true}
	}
	var groupVal sql.NullString
	if strings.TrimSpace(groupLabel) != "" {
		groupVal = sql.NullString{String: strings.TrimSpace(groupLabel), Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	if repType == "" {
		repType = "reps"
	}

	clauses := []string{"exercise_id = ?", "set_number = ?", "reps = ?", "percentage = ?", "absolute_weight = ?", "rest_seconds = ?", "prescribed_rpe = ?", "group_label = ?", "sort_order = ?", "rep_type = ?", "notes = ?"}
	args := []any{exerciseID, setNumber, repsVal, pctVal, absWeightVal, restSecondsVal, rpeVal, groupVal, sortOrder, repType, notesVal}
	switch {
	case patch.ClearRepsMax:
		clauses = append(clauses, "reps_max = NULL")
	case patch.RepsMax != nil:
		clauses = append(clauses, "reps_max = ?")
		args = append(args, *patch.RepsMax)
	}
	if patch.ClearRIR {
		clauses = append(clauses, "rir_min = NULL", "rir_max = NULL")
	} else {
		if patch.RIRMin != nil {
			clauses = append(clauses, "rir_min = ?")
			args = append(args, *patch.RIRMin)
		}
		if patch.RIRMax != nil {
			clauses = append(clauses, "rir_max = ?")
			args = append(args, *patch.RIRMax)
		}
	}
	if patch.NonWorking != nil {
		clauses = append(clauses, "non_working = ?")
		args = append(args, boolToInt(*patch.NonWorking))
	}
	if patch.GroupKey != nil {
		var keyVal sql.NullString
		if key := NormalizeGroupKey(*patch.GroupKey); key != "" {
			keyVal = sql.NullString{String: key, Valid: true}
		}
		clauses = append(clauses, "group_key = ?")
		args = append(args, keyVal)
	}
	args = append(args, id)

	result, err := db.ExecContext(ctx,
		`UPDATE prescribed_sets SET `+strings.Join(clauses, ", ")+` WHERE id = ?`,
		args...,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: prescribed set conflicts with existing set")
		}
		return nil, fmt.Errorf("models: update prescribed set %d: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	return GetPrescribedSetByID(ctx, db, id)
}

// CopyWeek replaces all prescribed sets in targetWeek with copies from
// sourceWeek within the same program template. Any existing sets in the
// target week are deleted first. Returns the number of sets inserted.
func CopyWeek(ctx context.Context, db *sql.DB, templateID int64, sourceWeek, targetWeek int) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("models: copy week begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete existing sets in target week.
	_, err = tx.ExecContext(ctx,
		`DELETE FROM prescribed_sets WHERE template_id = ? AND week = ?`,
		templateID, targetWeek,
	)
	if err != nil {
		return 0, fmt.Errorf("models: copy week delete target: %w", err)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT day, exercise_id, set_number, reps, percentage,
		        absolute_weight, rest_seconds, prescribed_rpe, group_label,
		        reps_max, rir_min, rir_max, non_working, group_key, sort_order, rep_type, notes
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
		restSeconds    sql.NullInt64
		prescribedRPE  sql.NullFloat64
		groupLabel     sql.NullString
		repsMax        sql.NullInt64
		rirMin         sql.NullInt64
		rirMax         sql.NullInt64
		nonWorking     int
		groupKey       sql.NullString
		sortOrder      int
		repType        string
		notes          sql.NullString
	}
	var sets []setRow
	for rows.Next() {
		var s setRow
		if err := rows.Scan(&s.day, &s.exerciseID, &s.setNumber,
			&s.reps, &s.percentage, &s.absoluteWeight,
			&s.restSeconds, &s.prescribedRPE, &s.groupLabel,
			&s.repsMax, &s.rirMin, &s.rirMax, &s.nonWorking, &s.groupKey, &s.sortOrder, &s.repType, &s.notes); err != nil {
			return 0, fmt.Errorf("models: copy week scan: %w", err)
		}
		sets = append(sets, s)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("models: copy week rows: %w", err)
	}

	inserted := 0
	for _, s := range sets {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO prescribed_sets
			   (template_id, week, day, exercise_id, set_number,
			    reps, percentage, absolute_weight, rest_seconds, prescribed_rpe, group_label,
			    reps_max, rir_min, rir_max, non_working, group_key, sort_order, rep_type, notes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			templateID, targetWeek, s.day, s.exerciseID, s.setNumber,
			s.reps, s.percentage, s.absoluteWeight, s.restSeconds, s.prescribedRPE, s.groupLabel,
			s.repsMax, s.rirMin, s.rirMax, s.nonWorking, s.groupKey, s.sortOrder, s.repType, s.notes,
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
