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

// ExerciseHistoryEntry represents a single set in the exercise history view,
// enriched with workout date for grouping.
type ExerciseHistoryEntry struct {
	WorkoutID   int64
	WorkoutDate string
	SetNumber   int
	Reps        int
	Weight      sql.NullFloat64
	Notes       sql.NullString
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
		SELECT ws.workout_id, w.date, ws.set_number, ws.reps, ws.weight, ws.notes
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
		if err := rows.Scan(&e.WorkoutID, &e.WorkoutDate, &e.SetNumber, &e.Reps, &e.Weight, &e.Notes); err != nil {
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
}

// ListRecentSetsForExercise returns the most recent sets logged for an exercise
// across all athletes, limited to 20 entries.
func ListRecentSetsForExercise(db *sql.DB, exerciseID int64) ([]*RecentExerciseSet, error) {
	rows, err := db.Query(`
		SELECT a.name, a.id, w.date, ws.set_number, ws.reps, ws.weight
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
		if err := rows.Scan(&s.AthleteName, &s.AthleteID, &s.WorkoutDate, &s.SetNumber, &s.Reps, &s.Weight); err != nil {
			return nil, fmt.Errorf("models: scan recent exercise set: %w", err)
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}
