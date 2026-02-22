package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AccessoryPlan represents a planned accessory exercise for an athlete on a
// specific program day. Decoupled from program templates — coaches/athletes
// manage these independently of prescribed main/supplemental work.
type AccessoryPlan struct {
	ID           int64
	AthleteID    int64
	Day          int
	ExerciseID   int64
	TargetSets   sql.NullInt64
	TargetRepMin sql.NullInt64
	TargetRepMax sql.NullInt64
	TargetWeight sql.NullFloat64
	Notes        sql.NullString
	SortOrder    int
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Joined fields populated by list queries.
	ExerciseName string
}

// RepRangeLabel returns a display string like "3×10-15" or "4×8" or "—".
func (ap *AccessoryPlan) RepRangeLabel() string {
	sets := ""
	if ap.TargetSets.Valid {
		sets = fmt.Sprintf("%d×", ap.TargetSets.Int64)
	}
	if ap.TargetRepMin.Valid && ap.TargetRepMax.Valid {
		if ap.TargetRepMin.Int64 == ap.TargetRepMax.Int64 {
			return fmt.Sprintf("%s%d", sets, ap.TargetRepMin.Int64)
		}
		return fmt.Sprintf("%s%d-%d", sets, ap.TargetRepMin.Int64, ap.TargetRepMax.Int64)
	}
	if ap.TargetRepMin.Valid {
		return fmt.Sprintf("%s%d+", sets, ap.TargetRepMin.Int64)
	}
	if ap.TargetRepMax.Valid {
		return fmt.Sprintf("%s≤%d", sets, ap.TargetRepMax.Int64)
	}
	if sets != "" {
		return sets[:len(sets)-1] + " sets" // strip trailing ×
	}
	return "—"
}

// CreateAccessoryPlan inserts a new accessory plan entry.
func CreateAccessoryPlan(db *sql.DB, athleteID int64, day int, exerciseID int64, targetSets, targetRepMin, targetRepMax int, targetWeight float64, notes string, sortOrder int) (*AccessoryPlan, error) {
	var tsVal sql.NullInt64
	if targetSets > 0 {
		tsVal = sql.NullInt64{Int64: int64(targetSets), Valid: true}
	}
	var minVal sql.NullInt64
	if targetRepMin > 0 {
		minVal = sql.NullInt64{Int64: int64(targetRepMin), Valid: true}
	}
	var maxVal sql.NullInt64
	if targetRepMax > 0 {
		maxVal = sql.NullInt64{Int64: int64(targetRepMax), Valid: true}
	}
	var wVal sql.NullFloat64
	if targetWeight > 0 {
		wVal = sql.NullFloat64{Float64: targetWeight, Valid: true}
	}
	var nVal sql.NullString
	if notes != "" {
		nVal = sql.NullString{String: notes, Valid: true}
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO accessory_plans (athlete_id, day, exercise_id, target_sets, target_rep_min, target_rep_max, target_weight, notes, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING id`,
		athleteID, day, exerciseID, tsVal, minVal, maxVal, wVal, nVal, sortOrder,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: accessory plan already exists for athlete %d day %d exercise %d", athleteID, day, exerciseID)
		}
		return nil, fmt.Errorf("models: create accessory plan: %w", err)
	}
	return GetAccessoryPlanByID(db, id)
}

// GetAccessoryPlanByID retrieves an accessory plan by primary key.
func GetAccessoryPlanByID(db *sql.DB, id int64) (*AccessoryPlan, error) {
	ap := &AccessoryPlan{}
	err := db.QueryRow(
		`SELECT ap.id, ap.athlete_id, ap.day, ap.exercise_id, ap.target_sets, ap.target_rep_min, ap.target_rep_max,
		        ap.target_weight, ap.notes, ap.sort_order, ap.active, ap.created_at, ap.updated_at,
		        e.name
		 FROM accessory_plans ap
		 JOIN exercises e ON e.id = ap.exercise_id
		 WHERE ap.id = ?`, id,
	).Scan(&ap.ID, &ap.AthleteID, &ap.Day, &ap.ExerciseID, &ap.TargetSets, &ap.TargetRepMin, &ap.TargetRepMax,
		&ap.TargetWeight, &ap.Notes, &ap.SortOrder, &ap.Active, &ap.CreatedAt, &ap.UpdatedAt,
		&ap.ExerciseName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get accessory plan %d: %w", id, err)
	}
	return ap, nil
}

// ListAccessoryPlansForDay returns active accessory plans for an athlete+day,
// ordered by sort_order then exercise name.
func ListAccessoryPlansForDay(db *sql.DB, athleteID int64, day int) ([]*AccessoryPlan, error) {
	rows, err := db.Query(`
		SELECT ap.id, ap.athlete_id, ap.day, ap.exercise_id, ap.target_sets, ap.target_rep_min, ap.target_rep_max,
		       ap.target_weight, ap.notes, ap.sort_order, ap.active, ap.created_at, ap.updated_at,
		       e.name
		FROM accessory_plans ap
		JOIN exercises e ON e.id = ap.exercise_id
		WHERE ap.athlete_id = ? AND ap.day = ? AND ap.active = 1
		ORDER BY ap.sort_order, e.name COLLATE NOCASE`, athleteID, day)
	if err != nil {
		return nil, fmt.Errorf("models: list accessory plans for athlete %d day %d: %w", athleteID, day, err)
	}
	defer rows.Close()

	var plans []*AccessoryPlan
	for rows.Next() {
		ap := &AccessoryPlan{}
		if err := rows.Scan(&ap.ID, &ap.AthleteID, &ap.Day, &ap.ExerciseID, &ap.TargetSets, &ap.TargetRepMin, &ap.TargetRepMax,
			&ap.TargetWeight, &ap.Notes, &ap.SortOrder, &ap.Active, &ap.CreatedAt, &ap.UpdatedAt,
			&ap.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan accessory plan: %w", err)
		}
		plans = append(plans, ap)
	}
	return plans, rows.Err()
}

// ListAllAccessoryPlans returns all accessory plans for an athlete (active and
// inactive), grouped by day. Ordered by day, sort_order, exercise name.
func ListAllAccessoryPlans(db *sql.DB, athleteID int64) ([]*AccessoryPlan, error) {
	rows, err := db.Query(`
		SELECT ap.id, ap.athlete_id, ap.day, ap.exercise_id, ap.target_sets, ap.target_rep_min, ap.target_rep_max,
		       ap.target_weight, ap.notes, ap.sort_order, ap.active, ap.created_at, ap.updated_at,
		       e.name
		FROM accessory_plans ap
		JOIN exercises e ON e.id = ap.exercise_id
		WHERE ap.athlete_id = ?
		ORDER BY ap.day, ap.active DESC, ap.sort_order, e.name COLLATE NOCASE`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list all accessory plans for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var plans []*AccessoryPlan
	for rows.Next() {
		ap := &AccessoryPlan{}
		if err := rows.Scan(&ap.ID, &ap.AthleteID, &ap.Day, &ap.ExerciseID, &ap.TargetSets, &ap.TargetRepMin, &ap.TargetRepMax,
			&ap.TargetWeight, &ap.Notes, &ap.SortOrder, &ap.Active, &ap.CreatedAt, &ap.UpdatedAt,
			&ap.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan accessory plan: %w", err)
		}
		plans = append(plans, ap)
	}
	return plans, rows.Err()
}

// UpdateAccessoryPlan updates an existing accessory plan entry.
func UpdateAccessoryPlan(db *sql.DB, id int64, targetSets, targetRepMin, targetRepMax int, targetWeight float64, notes string, sortOrder int) error {
	var tsVal sql.NullInt64
	if targetSets > 0 {
		tsVal = sql.NullInt64{Int64: int64(targetSets), Valid: true}
	}
	var minVal sql.NullInt64
	if targetRepMin > 0 {
		minVal = sql.NullInt64{Int64: int64(targetRepMin), Valid: true}
	}
	var maxVal sql.NullInt64
	if targetRepMax > 0 {
		maxVal = sql.NullInt64{Int64: int64(targetRepMax), Valid: true}
	}
	var wVal sql.NullFloat64
	if targetWeight > 0 {
		wVal = sql.NullFloat64{Float64: targetWeight, Valid: true}
	}
	var nVal sql.NullString
	if notes != "" {
		nVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE accessory_plans SET target_sets = ?, target_rep_min = ?, target_rep_max = ?, target_weight = ?, notes = ?, sort_order = ? WHERE id = ?`,
		tsVal, minVal, maxVal, wVal, nVal, sortOrder, id,
	)
	if err != nil {
		return fmt.Errorf("models: update accessory plan %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeactivateAccessoryPlan sets active = 0 on an accessory plan.
func DeactivateAccessoryPlan(db *sql.DB, id int64) error {
	result, err := db.Exec(`UPDATE accessory_plans SET active = 0 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: deactivate accessory plan %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAccessoryPlan permanently removes an accessory plan entry.
func DeleteAccessoryPlan(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM accessory_plans WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete accessory plan %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// MaxAccessoryDay returns the highest day number in use for an athlete's
// accessory plans. Returns 0 if no plans exist.
func MaxAccessoryDay(db *sql.DB, athleteID int64) (int, error) {
	var maxDay int
	err := db.QueryRow(
		`SELECT COALESCE(MAX(day), 0) FROM accessory_plans WHERE athlete_id = ?`,
		athleteID,
	).Scan(&maxDay)
	if err != nil {
		return 0, fmt.Errorf("models: max accessory day for athlete %d: %w", athleteID, err)
	}
	return maxDay, nil
}
