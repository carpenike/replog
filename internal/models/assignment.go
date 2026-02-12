package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrAlreadyAssigned is returned when an exercise is already actively assigned.
var ErrAlreadyAssigned = errors.New("exercise already assigned")

// AthleteExercise represents an assignment of an exercise to an athlete.
type AthleteExercise struct {
	ID            int64
	AthleteID     int64
	ExerciseID    int64
	Active        bool
	AssignedAt    time.Time
	DeactivatedAt sql.NullTime

	// Joined fields populated by list queries.
	ExerciseName string
	ExerciseTier sql.NullString
	TargetReps   sql.NullInt64
}

// AssignExercise creates an active assignment for an athlete+exercise pair.
// Returns ErrAlreadyAssigned if there is already an active assignment.
func AssignExercise(db *sql.DB, athleteID, exerciseID int64) (*AthleteExercise, error) {
	result, err := db.Exec(
		`INSERT INTO athlete_exercises (athlete_id, exercise_id, active) VALUES (?, ?, 1)`,
		athleteID, exerciseID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyAssigned
		}
		return nil, fmt.Errorf("models: assign exercise %d to athlete %d: %w", exerciseID, athleteID, err)
	}

	id, _ := result.LastInsertId()
	return GetAssignmentByID(db, id)
}

// DeactivateAssignment sets an assignment to inactive.
func DeactivateAssignment(db *sql.DB, id int64) error {
	result, err := db.Exec(
		`UPDATE athlete_exercises SET active = 0, deactivated_at = CURRENT_TIMESTAMP WHERE id = ? AND active = 1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("models: deactivate assignment %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ReactivateAssignment creates a new active assignment row for an athlete+exercise
// that was previously deactivated. This preserves the audit trail.
func ReactivateAssignment(db *sql.DB, athleteID, exerciseID int64) (*AthleteExercise, error) {
	return AssignExercise(db, athleteID, exerciseID)
}

// GetAssignmentByID retrieves an assignment by primary key.
func GetAssignmentByID(db *sql.DB, id int64) (*AthleteExercise, error) {
	ae := &AthleteExercise{}
	err := db.QueryRow(
		`SELECT ae.id, ae.athlete_id, ae.exercise_id, ae.active, ae.assigned_at, ae.deactivated_at,
		        e.name, e.tier, e.target_reps
		 FROM athlete_exercises ae
		 JOIN exercises e ON e.id = ae.exercise_id
		 WHERE ae.id = ?`, id,
	).Scan(&ae.ID, &ae.AthleteID, &ae.ExerciseID, &ae.Active, &ae.AssignedAt, &ae.DeactivatedAt,
		&ae.ExerciseName, &ae.ExerciseTier, &ae.TargetReps)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get assignment %d: %w", id, err)
	}
	return ae, nil
}

// ListActiveAssignments returns all active assignments for an athlete.
func ListActiveAssignments(db *sql.DB, athleteID int64) ([]*AthleteExercise, error) {
	rows, err := db.Query(`
		SELECT ae.id, ae.athlete_id, ae.exercise_id, ae.active, ae.assigned_at, ae.deactivated_at,
		       e.name, e.tier, e.target_reps
		FROM athlete_exercises ae
		JOIN exercises e ON e.id = ae.exercise_id
		WHERE ae.athlete_id = ? AND ae.active = 1
		ORDER BY e.name COLLATE NOCASE
		LIMIT 100`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list active assignments for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var assignments []*AthleteExercise
	for rows.Next() {
		ae := &AthleteExercise{}
		if err := rows.Scan(&ae.ID, &ae.AthleteID, &ae.ExerciseID, &ae.Active, &ae.AssignedAt, &ae.DeactivatedAt,
			&ae.ExerciseName, &ae.ExerciseTier, &ae.TargetReps); err != nil {
			return nil, fmt.Errorf("models: scan assignment: %w", err)
		}
		assignments = append(assignments, ae)
	}
	return assignments, rows.Err()
}

// ListUnassignedExercises returns exercises not actively assigned to an athlete.
func ListUnassignedExercises(db *sql.DB, athleteID int64) ([]*Exercise, error) {
	rows, err := db.Query(`
		SELECT e.id, e.name, e.tier, e.target_reps, e.form_notes, e.created_at, e.updated_at
		FROM exercises e
		WHERE e.id NOT IN (
			SELECT exercise_id FROM athlete_exercises
			WHERE athlete_id = ? AND active = 1
		)
		ORDER BY e.name COLLATE NOCASE`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list unassigned exercises for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var exercises []*Exercise
	for rows.Next() {
		e := &Exercise{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Tier, &e.TargetReps, &e.FormNotes, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("models: scan exercise: %w", err)
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}

// ListDeactivatedAssignments returns previously deactivated (but not re-activated)
// assignments for an athlete. These are exercises that were once assigned and could
// be reactivated.
func ListDeactivatedAssignments(db *sql.DB, athleteID int64) ([]*AthleteExercise, error) {
	rows, err := db.Query(`
		SELECT ae.id, ae.athlete_id, ae.exercise_id, ae.active, ae.assigned_at, ae.deactivated_at,
		       e.name, e.tier, e.target_reps
		FROM athlete_exercises ae
		JOIN exercises e ON e.id = ae.exercise_id
		WHERE ae.athlete_id = ? AND ae.active = 0
		  AND ae.exercise_id NOT IN (
		      SELECT exercise_id FROM athlete_exercises
		      WHERE athlete_id = ? AND active = 1
		  )
		GROUP BY ae.exercise_id
		HAVING ae.deactivated_at = MAX(ae.deactivated_at)
		ORDER BY e.name COLLATE NOCASE
		LIMIT 100`, athleteID, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list deactivated assignments for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var assignments []*AthleteExercise
	for rows.Next() {
		ae := &AthleteExercise{}
		if err := rows.Scan(&ae.ID, &ae.AthleteID, &ae.ExerciseID, &ae.Active, &ae.AssignedAt, &ae.DeactivatedAt,
			&ae.ExerciseName, &ae.ExerciseTier, &ae.TargetReps); err != nil {
			return nil, fmt.Errorf("models: scan deactivated assignment: %w", err)
		}
		assignments = append(assignments, ae)
	}
	return assignments, rows.Err()
}

// AssignedAthlete represents an athlete with an active assignment for a specific exercise.
type AssignedAthlete struct {
	AthleteID   int64
	AthleteName string
	AssignedAt  time.Time
}

// ListAssignedAthletes returns athletes with an active assignment for the given exercise.
func ListAssignedAthletes(db *sql.DB, exerciseID int64) ([]*AssignedAthlete, error) {
	rows, err := db.Query(`
		SELECT a.id, a.name, ae.assigned_at
		FROM athlete_exercises ae
		JOIN athletes a ON a.id = ae.athlete_id
		WHERE ae.exercise_id = ? AND ae.active = 1
		ORDER BY a.name COLLATE NOCASE
		LIMIT 100`, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("models: list assigned athletes for exercise %d: %w", exerciseID, err)
	}
	defer rows.Close()

	var athletes []*AssignedAthlete
	for rows.Next() {
		a := &AssignedAthlete{}
		if err := rows.Scan(&a.AthleteID, &a.AthleteName, &a.AssignedAt); err != nil {
			return nil, fmt.Errorf("models: scan assigned athlete: %w", err)
		}
		athletes = append(athletes, a)
	}
	return athletes, rows.Err()
}