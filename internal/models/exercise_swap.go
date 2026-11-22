package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoSharedPattern is returned when a swap names a substitute that does
// not share a movement pattern with the prescribed exercise.
var ErrNoSharedPattern = errors.New("substitute does not share a movement pattern")

// ErrSwapExists is returned when the prescribed exercise was already
// swapped in this workout.
var ErrSwapExists = errors.New("exercise already swapped in this workout")

// MaxSwapReasonLen caps the free-text reason on a swap.
const MaxSwapReasonLen = 500

// ExerciseSwap records an athlete swapping a prescribed exercise for a
// substitute with the same movement pattern during one workout (gap 11).
// It is a note for the coach's review; the program is never changed.
type ExerciseSwap struct {
	ID             int64
	WorkoutID      int64
	FromExerciseID int64
	FromName       string
	ToExerciseID   int64
	ToName         string
	Reason         sql.NullString
	CreatedAt      time.Time
}

// ListExerciseSubstitutes returns the exercises that share at least one
// movement pattern with exerciseID, excluding the exercise itself, sorted
// by how many patterns they share and then by name. Exercises with no
// pattern tags have no substitutes.
func ListExerciseSubstitutes(ctx context.Context, db *sql.DB, exerciseID int64) ([]*Exercise, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT e.id, e.name, e.tier, e.form_notes, e.demo_url, e.rest_seconds, e.featured,
		        e.created_at, e.updated_at
		   FROM exercises e
		   JOIN exercise_movement_patterns mp ON mp.exercise_id = e.id
		  WHERE e.id <> ?
		    AND mp.pattern IN (SELECT pattern FROM exercise_movement_patterns WHERE exercise_id = ?)
		  GROUP BY e.id
		  ORDER BY COUNT(*) DESC, e.name COLLATE NOCASE
		  LIMIT 100`, exerciseID, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("models: list substitutes for exercise %d: %w", exerciseID, err)
	}
	defer rows.Close()
	out := []*Exercise{}
	for rows.Next() {
		e := &Exercise{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Tier, &e.FormNotes, &e.DemoURL, &e.RestSeconds,
			&e.Featured, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("models: scan substitute: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate substitutes: %w", err)
	}
	return out, nil
}

// ExercisesSharePattern reports whether two exercises share a movement pattern.
func ExercisesSharePattern(ctx context.Context, db *sql.DB, a, b int64) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM exercise_movement_patterns x
		   JOIN exercise_movement_patterns y ON y.pattern = x.pattern
		  WHERE x.exercise_id = ? AND y.exercise_id = ?`, a, b).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("models: shared pattern %d/%d: %w", a, b, err)
	}
	return n > 0, nil
}

// CreateExerciseSwap logs a swap on a workout. The substitute must share a
// movement pattern with the prescribed exercise (ErrNoSharedPattern), and
// each prescribed exercise can be swapped once per workout (ErrSwapExists).
// The caller scopes workoutID to the athlete.
func CreateExerciseSwap(ctx context.Context, db *sql.DB, workoutID, fromID, toID int64, reason string) (*ExerciseSwap, error) {
	if fromID == toID {
		return nil, fmt.Errorf("%w: substitute must differ from the prescribed exercise", ErrInvalidInput)
	}
	if len(reason) > MaxSwapReasonLen {
		return nil, fmt.Errorf("%w: reason must be at most %d characters", ErrInvalidInput, MaxSwapReasonLen)
	}
	for _, id := range []int64{fromID, toID} {
		if _, err := GetExerciseByID(ctx, db, id); err != nil {
			return nil, err
		}
	}
	ok, err := ExercisesSharePattern(ctx, db, fromID, toID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoSharedPattern
	}
	var id int64
	err = db.QueryRowContext(ctx,
		`INSERT INTO workout_exercise_swaps (workout_id, from_exercise_id, to_exercise_id, reason)
		 VALUES (?, ?, ?, ?) RETURNING id`,
		workoutID, fromID, toID, nullableString(reason)).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSwapExists
		}
		return nil, fmt.Errorf("models: create swap on workout %d: %w", workoutID, err)
	}
	return getExerciseSwap(ctx, db, workoutID, id)
}

const swapSelect = `SELECT s.id, s.workout_id, s.from_exercise_id, f.name, s.to_exercise_id, t.name,
	       s.reason, s.created_at
	  FROM workout_exercise_swaps s
	  JOIN exercises f ON f.id = s.from_exercise_id
	  JOIN exercises t ON t.id = s.to_exercise_id`

func scanSwap(sc interface{ Scan(...any) error }) (*ExerciseSwap, error) {
	s := &ExerciseSwap{}
	err := sc.Scan(&s.ID, &s.WorkoutID, &s.FromExerciseID, &s.FromName, &s.ToExerciseID, &s.ToName,
		&s.Reason, &s.CreatedAt)
	return s, err
}

func getExerciseSwap(ctx context.Context, db *sql.DB, workoutID, id int64) (*ExerciseSwap, error) {
	s, err := scanSwap(db.QueryRowContext(ctx, swapSelect+` WHERE s.id = ? AND s.workout_id = ?`, id, workoutID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get swap %d: %w", id, err)
	}
	return s, nil
}

// ListExerciseSwaps returns a workout's swaps in the order they were made.
func ListExerciseSwaps(ctx context.Context, db *sql.DB, workoutID int64) ([]*ExerciseSwap, error) {
	rows, err := db.QueryContext(ctx, swapSelect+` WHERE s.workout_id = ? ORDER BY s.id`, workoutID)
	if err != nil {
		return nil, fmt.Errorf("models: list swaps for workout %d: %w", workoutID, err)
	}
	defer rows.Close()
	out := []*ExerciseSwap{}
	for rows.Next() {
		s, err := scanSwap(rows)
		if err != nil {
			return nil, fmt.Errorf("models: scan swap: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate swaps: %w", err)
	}
	return out, nil
}

// DeleteExerciseSwap undoes a swap. It is scoped to the workout, so a swap
// id from another workout returns ErrNotFound. Sets already logged against
// the substitute stay in the log.
func DeleteExerciseSwap(ctx context.Context, db *sql.DB, workoutID, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM workout_exercise_swaps WHERE id = ? AND workout_id = ?`, id, workoutID)
	if err != nil {
		return fmt.Errorf("models: delete swap %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
