package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// WorkoutReview represents a coach's review of an athlete's workout.
type WorkoutReview struct {
	ID        int64
	WorkoutID int64
	CoachID   int64
	Status    string // "approved" or "needs_work"
	Notes     sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time

	// Joined fields populated by queries.
	CoachUsername string
}

// ReviewStatusApproved indicates the coach approved the workout.
const ReviewStatusApproved = "approved"

// ReviewStatusNeedsWork indicates the coach wants the athlete to address feedback.
const ReviewStatusNeedsWork = "needs_work"

// UnreviewedWorkout represents a workout pending coach review, with joined fields
// for display in the review dashboard.
type UnreviewedWorkout struct {
	WorkoutID   int64
	AthleteID   int64
	AthleteName string
	Date        string
	SetCount    int
	Notes       sql.NullString
}

// ReviewStats holds aggregate counts for the coach dashboard.
type ReviewStats struct {
	PendingCount  int
	ApprovedCount int
	NeedsWork     int
}

// CreateWorkoutReview inserts a new review for a workout. Returns ErrWorkoutExists
// semantically if a review already exists (unique constraint on workout_id).
func CreateWorkoutReview(db *sql.DB, workoutID, coachID int64, status, notes string) (*WorkoutReview, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO workout_reviews (workout_id, coach_id, status, notes)
		 VALUES (?, ?, ?, ?)`,
		workoutID, coachID, status, notesVal,
	)
	if err != nil {
		if isUniqueViolation(err) {
			// Review already exists — caller should use Update instead.
			return nil, fmt.Errorf("models: review already exists for workout %d: %w", workoutID, ErrInvalidInput)
		}
		return nil, fmt.Errorf("models: create workout review for workout %d: %w", workoutID, err)
	}

	id, _ := result.LastInsertId()
	return GetWorkoutReviewByID(db, id)
}

// UpdateWorkoutReview updates an existing review's status, notes, and coach.
func UpdateWorkoutReview(db *sql.DB, id, coachID int64, status, notes string) (*WorkoutReview, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE workout_reviews SET coach_id = ?, status = ?, notes = ?
		 WHERE id = ?`,
		coachID, status, notesVal, id,
	)
	if err != nil {
		return nil, fmt.Errorf("models: update workout review %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, ErrNotFound
	}

	return GetWorkoutReviewByID(db, id)
}

// GetWorkoutReviewByID retrieves a review by primary key.
func GetWorkoutReviewByID(db *sql.DB, id int64) (*WorkoutReview, error) {
	rev := &WorkoutReview{}
	err := db.QueryRow(
		`SELECT wr.id, wr.workout_id, wr.coach_id, wr.status, wr.notes,
		        wr.created_at, wr.updated_at, u.username
		 FROM workout_reviews wr
		 JOIN users u ON u.id = wr.coach_id
		 WHERE wr.id = ?`, id,
	).Scan(&rev.ID, &rev.WorkoutID, &rev.CoachID, &rev.Status, &rev.Notes,
		&rev.CreatedAt, &rev.UpdatedAt, &rev.CoachUsername)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get workout review %d: %w", id, err)
	}
	return rev, nil
}

// GetWorkoutReviewByWorkoutID retrieves the review for a specific workout.
// Returns ErrNotFound if no review exists.
func GetWorkoutReviewByWorkoutID(db *sql.DB, workoutID int64) (*WorkoutReview, error) {
	rev := &WorkoutReview{}
	err := db.QueryRow(
		`SELECT wr.id, wr.workout_id, wr.coach_id, wr.status, wr.notes,
		        wr.created_at, wr.updated_at, u.username
		 FROM workout_reviews wr
		 JOIN users u ON u.id = wr.coach_id
		 WHERE wr.workout_id = ?`, workoutID,
	).Scan(&rev.ID, &rev.WorkoutID, &rev.CoachID, &rev.Status, &rev.Notes,
		&rev.CreatedAt, &rev.UpdatedAt, &rev.CoachUsername)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get workout review for workout %d: %w", workoutID, err)
	}
	return rev, nil
}

// DeleteWorkoutReview removes a review by primary key.
func DeleteWorkoutReview(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM workout_reviews WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete workout review %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUnreviewedWorkouts returns all workouts that have not been reviewed,
// ordered by date descending. Useful for the coach review dashboard.
func ListUnreviewedWorkouts(db *sql.DB) ([]*UnreviewedWorkout, error) {
	rows, err := db.Query(`
		SELECT w.id, w.athlete_id, a.name, w.date, w.notes,
		       (SELECT COUNT(*) FROM workout_sets ws WHERE ws.workout_id = w.id)
		FROM workouts w
		JOIN athletes a ON a.id = w.athlete_id
		LEFT JOIN workout_reviews wr ON wr.workout_id = w.id
		WHERE wr.id IS NULL
		ORDER BY w.date DESC
		LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("models: list unreviewed workouts: %w", err)
	}
	defer rows.Close()

	var workouts []*UnreviewedWorkout
	for rows.Next() {
		uw := &UnreviewedWorkout{}
		if err := rows.Scan(&uw.WorkoutID, &uw.AthleteID, &uw.AthleteName,
			&uw.Date, &uw.Notes, &uw.SetCount); err != nil {
			return nil, fmt.Errorf("models: scan unreviewed workout: %w", err)
		}
		workouts = append(workouts, uw)
	}
	return workouts, rows.Err()
}

// GetReviewStats returns aggregate counts of review statuses for the coach dashboard.
func GetReviewStats(db *sql.DB) (*ReviewStats, error) {
	stats := &ReviewStats{}

	// Count pending (unreviewed) workouts.
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM workouts w
		LEFT JOIN workout_reviews wr ON wr.workout_id = w.id
		WHERE wr.id IS NULL`).Scan(&stats.PendingCount)
	if err != nil {
		return nil, fmt.Errorf("models: count pending reviews: %w", err)
	}

	// Count approved and needs_work.
	rows, err := db.Query(`
		SELECT status, COUNT(*)
		FROM workout_reviews
		GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("models: count review statuses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("models: scan review status count: %w", err)
		}
		switch status {
		case ReviewStatusApproved:
			stats.ApprovedCount = count
		case ReviewStatusNeedsWork:
			stats.NeedsWork = count
		}
	}

	return stats, rows.Err()
}

// CreateOrUpdateWorkoutReview creates a review if none exists, or updates the
// existing one. This is the primary entry point for the review handler.
func CreateOrUpdateWorkoutReview(db *sql.DB, workoutID, coachID int64, status, notes string) (*WorkoutReview, error) {
	existing, err := GetWorkoutReviewByWorkoutID(db, workoutID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("models: check existing review for workout %d: %w", workoutID, err)
	}

	if existing != nil {
		return UpdateWorkoutReview(db, existing.ID, coachID, status, notes)
	}
	return CreateWorkoutReview(db, workoutID, coachID, status, notes)
}
