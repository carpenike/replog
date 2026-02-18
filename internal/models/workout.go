package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrWorkoutExists is returned when a workout already exists for an athlete+date.
var ErrWorkoutExists = errors.New("workout already exists for this date")

// Workout represents a training session for one athlete on one date.
type Workout struct {
	ID        int64
	AthleteID int64
	Date      string // DATE as string (YYYY-MM-DD)
	Notes     sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time

	// Joined fields populated by list queries.
	AthleteName  string
	SetCount     int
	ReviewStatus sql.NullString // NULL = unreviewed, "approved", "needs_work"
}

// CreateWorkout starts a new workout for an athlete on a date.
func CreateWorkout(db *sql.DB, athleteID int64, date, notes string) (*Workout, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO workouts (athlete_id, date, notes) VALUES (?, ?, ?) RETURNING id`,
		athleteID, date, notesVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWorkoutExists
		}
		return nil, fmt.Errorf("models: create workout for athlete %d on %s: %w", athleteID, date, err)
	}

	return GetWorkoutByID(db, id)
}

// GetWorkoutByID retrieves a workout by primary key.
func GetWorkoutByID(db *sql.DB, id int64) (*Workout, error) {
	w := &Workout{}
	err := db.QueryRow(
		`SELECT w.id, w.athlete_id, w.date, w.notes, w.created_at, w.updated_at, a.name,
		        (SELECT COUNT(*) FROM workout_sets ws WHERE ws.workout_id = w.id)
		 FROM workouts w
		 JOIN athletes a ON a.id = w.athlete_id
		 WHERE w.id = ?`, id,
	).Scan(&w.ID, &w.AthleteID, &w.Date, &w.Notes, &w.CreatedAt, &w.UpdatedAt, &w.AthleteName, &w.SetCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get workout %d: %w", id, err)
	}
	return w, nil
}

// GetWorkoutByAthleteDate retrieves a workout for an athlete on a specific date.
func GetWorkoutByAthleteDate(db *sql.DB, athleteID int64, date string) (*Workout, error) {
	w := &Workout{}
	err := db.QueryRow(
		`SELECT w.id, w.athlete_id, w.date, w.notes, w.created_at, w.updated_at, a.name,
		        (SELECT COUNT(*) FROM workout_sets ws WHERE ws.workout_id = w.id)
		 FROM workouts w
		 JOIN athletes a ON a.id = w.athlete_id
		 WHERE w.athlete_id = ? AND w.date = ?`, athleteID, date,
	).Scan(&w.ID, &w.AthleteID, &w.Date, &w.Notes, &w.CreatedAt, &w.UpdatedAt, &w.AthleteName, &w.SetCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get workout for athlete %d on %s: %w", athleteID, date, err)
	}
	return w, nil
}

// UpdateWorkoutNotes updates the notes on an existing workout.
func UpdateWorkoutNotes(db *sql.DB, id int64, notes string) error {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(`UPDATE workouts SET notes = ? WHERE id = ?`, notesVal, id)
	if err != nil {
		return fmt.Errorf("models: update workout %d notes: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteWorkout removes a workout and all its sets (CASCADE).
func DeleteWorkout(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM workouts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete workout %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// WorkoutPageSize is the number of workouts returned per page.
const WorkoutPageSize = 50

// WorkoutPage holds a page of workouts and whether more rows exist.
type WorkoutPage struct {
	Workouts []*Workout
	HasMore  bool
}

// ListWorkouts returns workouts for an athlete, ordered by date descending.
// Pass offset=0 for the first page. Returns up to WorkoutPageSize rows and
// sets HasMore if additional rows exist beyond the current page.
func ListWorkouts(db *sql.DB, athleteID int64, offset int) (*WorkoutPage, error) {
	rows, err := db.Query(`
		SELECT w.id, w.athlete_id, w.date, w.notes, w.created_at, w.updated_at, a.name,
		       (SELECT COUNT(*) FROM workout_sets ws WHERE ws.workout_id = w.id),
		       wr.status
		FROM workouts w
		JOIN athletes a ON a.id = w.athlete_id
		LEFT JOIN workout_reviews wr ON wr.workout_id = w.id
		WHERE w.athlete_id = ?
		ORDER BY w.date DESC
		LIMIT ? OFFSET ?`, athleteID, WorkoutPageSize+1, offset)
	if err != nil {
		return nil, fmt.Errorf("models: list workouts for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var workouts []*Workout
	for rows.Next() {
		w := &Workout{}
		if err := rows.Scan(&w.ID, &w.AthleteID, &w.Date, &w.Notes, &w.CreatedAt, &w.UpdatedAt, &w.AthleteName, &w.SetCount, &w.ReviewStatus); err != nil {
			return nil, fmt.Errorf("models: scan workout: %w", err)
		}
		workouts = append(workouts, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(workouts) > WorkoutPageSize
	if hasMore {
		workouts = workouts[:WorkoutPageSize]
	}
	return &WorkoutPage{Workouts: workouts, HasMore: hasMore}, nil
}
