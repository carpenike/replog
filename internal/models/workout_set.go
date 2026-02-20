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
	RPE        sql.NullFloat64
	RepType    string // "reps", "each_side", "seconds", or "distance"
	Notes      sql.NullString
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Joined fields populated by list queries.
	ExerciseName string
}

// RepsLabel returns a display string for reps (e.g. "5", "5/ea", "30s", "30yd").
func (ws *WorkoutSet) RepsLabel() string {
	switch ws.RepType {
	case "each_side":
		return fmt.Sprintf("%d/ea", ws.Reps)
	case "seconds":
		return fmt.Sprintf("%ds", ws.Reps)
	case "distance":
		return fmt.Sprintf("%dyd", ws.Reps)
	default:
		return fmt.Sprintf("%d", ws.Reps)
	}
}

// AddSet inserts a new set into a workout. set_number is auto-calculated as the
// next number for the given workout+exercise. The read-then-write is wrapped in
// a transaction to prevent duplicate set numbers under concurrent requests.
func AddSet(db *sql.DB, workoutID, exerciseID int64, reps int, weight float64, rpe float64, repType, notes string) (*WorkoutSet, error) {
	var weightVal sql.NullFloat64
	if weight > 0 {
		weightVal = sql.NullFloat64{Float64: weight, Valid: true}
	}
	var rpeVal sql.NullFloat64
	if rpe > 0 {
		rpeVal = sql.NullFloat64{Float64: rpe, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	if repType == "" {
		repType = "reps"
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin tx for add set: %w", err)
	}
	defer tx.Rollback()

	// Compute next set_number for this workout+exercise.
	var nextSet int
	err = tx.QueryRow(
		`SELECT COALESCE(MAX(set_number), 0) + 1 FROM workout_sets WHERE workout_id = ? AND exercise_id = ?`,
		workoutID, exerciseID,
	).Scan(&nextSet)
	if err != nil {
		return nil, fmt.Errorf("models: compute next set number: %w", err)
	}

	var id int64
	err = tx.QueryRow(
		`INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps, weight, rpe, rep_type, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		workoutID, exerciseID, nextSet, reps, weightVal, rpeVal, repType, notesVal,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: add set to workout %d: %w", workoutID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit add set: %w", err)
	}

	return GetSetByID(db, id)
}

// AddMultipleSets inserts count identical sets for a workout+exercise in a
// single transaction. Returns the created sets. Useful for logging e.g.
// "5×5 @ 135 lbs" in one action.
func AddMultipleSets(db *sql.DB, workoutID, exerciseID int64, count, reps int, weight float64, rpe float64, repType, notes string) ([]*WorkoutSet, error) {
	if count <= 0 {
		return nil, fmt.Errorf("models: set count must be positive, got %d", count)
	}
	if count == 1 {
		s, err := AddSet(db, workoutID, exerciseID, reps, weight, rpe, repType, notes)
		if err != nil {
			return nil, err
		}
		return []*WorkoutSet{s}, nil
	}

	var weightVal sql.NullFloat64
	if weight > 0 {
		weightVal = sql.NullFloat64{Float64: weight, Valid: true}
	}
	var rpeVal sql.NullFloat64
	if rpe > 0 {
		rpeVal = sql.NullFloat64{Float64: rpe, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	if repType == "" {
		repType = "reps"
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin tx for add multiple sets: %w", err)
	}
	defer tx.Rollback()

	// Compute next set_number for this workout+exercise.
	var nextSet int
	err = tx.QueryRow(
		`SELECT COALESCE(MAX(set_number), 0) + 1 FROM workout_sets WHERE workout_id = ? AND exercise_id = ?`,
		workoutID, exerciseID,
	).Scan(&nextSet)
	if err != nil {
		return nil, fmt.Errorf("models: compute next set number: %w", err)
	}

	var ids []int64
	for i := 0; i < count; i++ {
		var id int64
		err := tx.QueryRow(
			`INSERT INTO workout_sets (workout_id, exercise_id, set_number, reps, weight, rpe, rep_type, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			workoutID, exerciseID, nextSet+i, reps, weightVal, rpeVal, repType, notesVal,
		).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("models: add set %d of %d to workout %d: %w", i+1, count, workoutID, err)
		}
		ids = append(ids, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit add multiple sets: %w", err)
	}

	sets := make([]*WorkoutSet, 0, len(ids))
	for _, id := range ids {
		s, err := GetSetByID(db, id)
		if err != nil {
			return nil, fmt.Errorf("models: get created set %d: %w", id, err)
		}
		sets = append(sets, s)
	}
	return sets, nil
}

// GetSetByID retrieves a workout set by primary key.
func GetSetByID(db *sql.DB, id int64) (*WorkoutSet, error) {
	s := &WorkoutSet{}
	err := db.QueryRow(
		`SELECT ws.id, ws.workout_id, ws.exercise_id, ws.set_number, ws.reps, ws.weight, ws.rpe, ws.rep_type, ws.notes, ws.created_at, ws.updated_at,
		        e.name
		 FROM workout_sets ws
		 JOIN exercises e ON e.id = ws.exercise_id
		 WHERE ws.id = ?`, id,
	).Scan(&s.ID, &s.WorkoutID, &s.ExerciseID, &s.SetNumber, &s.Reps, &s.Weight, &s.RPE, &s.RepType, &s.Notes, &s.CreatedAt, &s.UpdatedAt, &s.ExerciseName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get set %d: %w", id, err)
	}
	return s, nil
}

// UpdateSet updates a set's reps, weight, RPE, and notes.
func UpdateSet(db *sql.DB, id int64, reps int, weight float64, rpe float64, notes string) (*WorkoutSet, error) {
	var weightVal sql.NullFloat64
	if weight > 0 {
		weightVal = sql.NullFloat64{Float64: weight, Valid: true}
	}
	var rpeVal sql.NullFloat64
	if rpe > 0 {
		rpeVal = sql.NullFloat64{Float64: rpe, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE workout_sets SET reps = ?, weight = ?, rpe = ?, notes = ? WHERE id = ?`,
		reps, weightVal, rpeVal, notesVal, id,
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

// DeleteSet removes a set from a workout and renumbers the remaining sets
// for the same workout+exercise to maintain a contiguous sequence.
func DeleteSet(db *sql.DB, id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("models: begin tx for delete set: %w", err)
	}
	defer tx.Rollback()

	// Look up the set's workout and exercise before deleting.
	var workoutID, exerciseID int64
	err = tx.QueryRow(`SELECT workout_id, exercise_id FROM workout_sets WHERE id = ?`, id).Scan(&workoutID, &exerciseID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("models: lookup set %d for delete: %w", id, err)
	}

	// Delete the target set.
	_, err = tx.Exec(`DELETE FROM workout_sets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete set %d: %w", id, err)
	}

	// Renumber remaining sets for this workout+exercise.
	// First negate all set_numbers to avoid unique constraint violations.
	_, err = tx.Exec(
		`UPDATE workout_sets SET set_number = -set_number WHERE workout_id = ? AND exercise_id = ?`,
		workoutID, exerciseID,
	)
	if err != nil {
		return fmt.Errorf("models: negate set numbers for renumber: %w", err)
	}

	// Read remaining set IDs in original order (negate back for ORDER BY).
	rows, err := tx.Query(
		`SELECT id FROM workout_sets WHERE workout_id = ? AND exercise_id = ? ORDER BY -set_number`,
		workoutID, exerciseID,
	)
	if err != nil {
		return fmt.Errorf("models: read sets for renumber: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var setID int64
		if err := rows.Scan(&setID); err != nil {
			rows.Close()
			return fmt.Errorf("models: scan set id for renumber: %w", err)
		}
		ids = append(ids, setID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("models: iterate sets for renumber: %w", err)
	}

	// Assign new contiguous set_numbers starting at 1.
	for i, setID := range ids {
		_, err = tx.Exec(`UPDATE workout_sets SET set_number = ? WHERE id = ?`, i+1, setID)
		if err != nil {
			return fmt.Errorf("models: renumber set %d: %w", setID, err)
		}
	}

	return tx.Commit()
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
		SELECT ws.id, ws.workout_id, ws.exercise_id, ws.set_number, ws.reps, ws.weight, ws.rpe, ws.rep_type, ws.notes, ws.created_at, ws.updated_at,
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
		if err := rows.Scan(&s.ID, &s.WorkoutID, &s.ExerciseID, &s.SetNumber, &s.Reps, &s.Weight, &s.RPE, &s.RepType, &s.Notes, &s.CreatedAt, &s.UpdatedAt, &s.ExerciseName); err != nil {
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

// ListSetsByWorkoutIDs returns all sets for multiple workouts in a single query,
// keyed by workout ID. Each value is a slice of ExerciseGroups for that workout.
// This replaces N calls to ListSetsByWorkout with 1 query.
func ListSetsByWorkoutIDs(db *sql.DB, workoutIDs []int64) (map[int64][]*ExerciseGroup, error) {
	if len(workoutIDs) == 0 {
		return make(map[int64][]*ExerciseGroup), nil
	}

	// Build IN clause placeholders.
	placeholders := make([]byte, 0, len(workoutIDs)*2)
	args := make([]any, len(workoutIDs))
	for i, id := range workoutIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args[i] = id
	}

	rows, err := db.Query(`
		SELECT ws.id, ws.workout_id, ws.exercise_id, ws.set_number, ws.reps, ws.weight, ws.rpe, ws.rep_type, ws.notes, ws.created_at, ws.updated_at,
		       e.name
		FROM workout_sets ws
		JOIN exercises e ON e.id = ws.exercise_id
		WHERE ws.workout_id IN (`+string(placeholders)+`)
		ORDER BY ws.workout_id, e.name COLLATE NOCASE, ws.set_number`, args...)
	if err != nil {
		return nil, fmt.Errorf("models: list sets for %d workouts: %w", len(workoutIDs), err)
	}
	defer rows.Close()

	// Track exercise groups per workout, preserving insertion order.
	type workoutGroups struct {
		groupMap   map[int64]*ExerciseGroup
		groupOrder []int64
	}
	byWorkout := make(map[int64]*workoutGroups, len(workoutIDs))

	for rows.Next() {
		s := &WorkoutSet{}
		if err := rows.Scan(&s.ID, &s.WorkoutID, &s.ExerciseID, &s.SetNumber, &s.Reps, &s.Weight, &s.RPE, &s.RepType, &s.Notes, &s.CreatedAt, &s.UpdatedAt, &s.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan set in batch: %w", err)
		}

		wg, exists := byWorkout[s.WorkoutID]
		if !exists {
			wg = &workoutGroups{groupMap: make(map[int64]*ExerciseGroup)}
			byWorkout[s.WorkoutID] = wg
		}

		g, gExists := wg.groupMap[s.ExerciseID]
		if !gExists {
			g = &ExerciseGroup{
				ExerciseID:   s.ExerciseID,
				ExerciseName: s.ExerciseName,
			}
			wg.groupMap[s.ExerciseID] = g
			wg.groupOrder = append(wg.groupOrder, s.ExerciseID)
		}
		g.Sets = append(g.Sets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert to final map.
	result := make(map[int64][]*ExerciseGroup, len(byWorkout))
	for wid, wg := range byWorkout {
		groups := make([]*ExerciseGroup, 0, len(wg.groupOrder))
		for _, eid := range wg.groupOrder {
			groups = append(groups, wg.groupMap[eid])
		}
		result[wid] = groups
	}
	return result, nil
}

// ExerciseHistoryEntry represents a single set in the exercise history view,
// enriched with workout date for grouping.
type ExerciseHistoryEntry struct {
	WorkoutID   int64
	WorkoutDate string
	SetNumber   int
	Reps        int
	Weight      sql.NullFloat64
	RPE         sql.NullFloat64
	Notes       sql.NullString
}

// LastSessionSummary holds the previous session's data for an exercise.
type LastSessionSummary struct {
	Date string
	Sets []*LastSessionSet
}

// LastSessionSet holds one set from the previous session.
type LastSessionSet struct {
	SetNumber int
	Reps      int
	Weight    sql.NullFloat64
	RPE       sql.NullFloat64
	RepType   string
}

// RepsLabel returns a display string for reps (e.g. "5", "5/ea", "30s", "30yd").
func (ls *LastSessionSet) RepsLabel() string {
	switch ls.RepType {
	case "each_side":
		return fmt.Sprintf("%d/ea", ls.Reps)
	case "seconds":
		return fmt.Sprintf("%ds", ls.Reps)
	case "distance":
		return fmt.Sprintf("%dyd", ls.Reps)
	default:
		return fmt.Sprintf("%d", ls.Reps)
	}
}

// LastSessionSets returns the sets from the most recent workout before beforeDate
// for the given athlete and exercise. Returns nil if no previous session exists.
func LastSessionSets(db *sql.DB, athleteID, exerciseID int64, beforeDate string) (*LastSessionSummary, error) {
	// Find the most recent workout ID for this athlete+exercise before the given date.
	var workoutID int64
	var workoutDate string
	err := db.QueryRow(`
		SELECT w.id, w.date
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		WHERE w.athlete_id = ? AND ws.exercise_id = ? AND date(w.date) < date(?)
		ORDER BY w.date DESC
		LIMIT 1`, athleteID, exerciseID, beforeDate).Scan(&workoutID, &workoutDate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models: last session for athlete %d exercise %d: %w", athleteID, exerciseID, err)
	}

	rows, err := db.Query(`
		SELECT set_number, reps, weight, rpe, rep_type
		FROM workout_sets
		WHERE workout_id = ? AND exercise_id = ?
		ORDER BY set_number`, workoutID, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("models: last session sets: %w", err)
	}
	defer rows.Close()

	var sets []*LastSessionSet
	for rows.Next() {
		s := &LastSessionSet{}
		if err := rows.Scan(&s.SetNumber, &s.Reps, &s.Weight, &s.RPE, &s.RepType); err != nil {
			return nil, fmt.Errorf("models: scan last session set: %w", err)
		}
		sets = append(sets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &LastSessionSummary{
		Date: normalizeDate(workoutDate),
		Sets: sets,
	}, nil
}

// ExerciseHistoryDay groups sets performed on a single workout date.
type ExerciseHistoryDay struct {
	WorkoutID   int64
	WorkoutDate string
	Sets        []*ExerciseHistoryEntry
}

// ExerciseHistoryPageSize is the max number of workout days returned per page
// of exercise history.
const ExerciseHistoryPageSize = 20

// ExerciseHistoryPage holds a page of exercise history days and whether more exist.
type ExerciseHistoryPage struct {
	Days    []*ExerciseHistoryDay
	HasMore bool
}

// ListExerciseHistory returns sets for a specific exercise performed by an
// athlete, grouped by workout date (most recent first). Uses offset-based
// pagination on distinct workout dates. Pass offset=0 for the first page.
func ListExerciseHistory(db *sql.DB, athleteID, exerciseID int64, offset int) (*ExerciseHistoryPage, error) {
	// First, get the distinct workout IDs for this athlete+exercise with pagination.
	idRows, err := db.Query(`
		SELECT DISTINCT w.id, w.date
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		WHERE w.athlete_id = ? AND ws.exercise_id = ?
		ORDER BY w.date DESC
		LIMIT ? OFFSET ?`, athleteID, exerciseID, ExerciseHistoryPageSize+1, offset)
	if err != nil {
		return nil, fmt.Errorf("models: list exercise history workout ids for athlete %d exercise %d: %w", athleteID, exerciseID, err)
	}
	defer idRows.Close()

	type workoutRef struct {
		ID   int64
		Date string
	}
	var refs []workoutRef
	for idRows.Next() {
		var ref workoutRef
		if err := idRows.Scan(&ref.ID, &ref.Date); err != nil {
			return nil, fmt.Errorf("models: scan exercise history workout ref: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := idRows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(refs) > ExerciseHistoryPageSize
	if hasMore {
		refs = refs[:ExerciseHistoryPageSize]
	}

	if len(refs) == 0 {
		return &ExerciseHistoryPage{Days: nil, HasMore: false}, nil
	}

	// Build placeholders and args for the IN clause.
	placeholders := make([]byte, 0, len(refs)*2)
	args := make([]any, 0, len(refs)+1)
	for i, ref := range refs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, ref.ID)
	}
	args = append(args, exerciseID)

	rows, err := db.Query(`
		SELECT ws.workout_id, w.date, ws.set_number, ws.reps, ws.weight, ws.rpe, ws.notes
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		WHERE ws.workout_id IN (`+string(placeholders)+`) AND ws.exercise_id = ?
		ORDER BY w.date DESC, ws.set_number`, args...)
	if err != nil {
		return nil, fmt.Errorf("models: list exercise history for athlete %d exercise %d: %w", athleteID, exerciseID, err)
	}
	defer rows.Close()

	dayMap := make(map[int64]*ExerciseHistoryDay)
	var dayOrder []int64

	for rows.Next() {
		e := &ExerciseHistoryEntry{}
		if err := rows.Scan(&e.WorkoutID, &e.WorkoutDate, &e.SetNumber, &e.Reps, &e.Weight, &e.RPE, &e.Notes); err != nil {
			return nil, fmt.Errorf("models: scan exercise history entry: %w", err)
		}

		d, exists := dayMap[e.WorkoutID]
		if !exists {
			d = &ExerciseHistoryDay{
				WorkoutID:   e.WorkoutID,
				WorkoutDate: e.WorkoutDate,
			}
			dayMap[e.WorkoutID] = d
			dayOrder = append(dayOrder, e.WorkoutID)
		}
		d.Sets = append(d.Sets, e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	days := make([]*ExerciseHistoryDay, 0, len(dayOrder))
	for _, wid := range dayOrder {
		days = append(days, dayMap[wid])
	}
	return &ExerciseHistoryPage{Days: days, HasMore: hasMore}, nil
}

// RecentExerciseSet represents a recently logged set for an exercise, with
// athlete context for the exercise detail view.
type RecentExerciseSet struct {
	AthleteName string
	AthleteID   int64
	WorkoutDate string
	SetNumber   int
	Reps        int
	Weight      sql.NullFloat64
	RPE         sql.NullFloat64
}

// ListRecentSetsForExercise returns the most recent sets logged for an exercise
// across all athletes, limited to 20 entries.
func ListRecentSetsForExercise(db *sql.DB, exerciseID int64) ([]*RecentExerciseSet, error) {
	rows, err := db.Query(`
		SELECT a.name, a.id, w.date, ws.set_number, ws.reps, ws.weight, ws.rpe
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		JOIN athletes a ON a.id = w.athlete_id
		WHERE ws.exercise_id = ?
		ORDER BY w.date DESC, a.name COLLATE NOCASE, ws.set_number
		LIMIT 20`, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("models: list recent sets for exercise %d: %w", exerciseID, err)
	}
	defer rows.Close()

	var sets []*RecentExerciseSet
	for rows.Next() {
		s := &RecentExerciseSet{}
		if err := rows.Scan(&s.AthleteName, &s.AthleteID, &s.WorkoutDate, &s.SetNumber, &s.Reps, &s.Weight, &s.RPE); err != nil {
			return nil, fmt.Errorf("models: scan recent exercise set: %w", err)
		}
		sets = append(sets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate recent exercise sets: %w", err)
	}
	return sets, nil
}
