package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RecoveryCheckin is the detail row for a discipline='recovery' workout (ADR
// 018) — a subjective manual check-in (sleep/soreness/energy). Objective
// wearable sleep lives in bio_samples; the two are never summed, and the
// weekly load view excludes recovery entirely (it is a recovery signal, not
// training load).
type RecoveryCheckin struct {
	ID         int64
	WorkoutID  int64
	SleepHours sql.NullFloat64
	Soreness   sql.NullInt64
	Energy     sql.NullInt64
	Notes      sql.NullString
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Joined from the parent workout.
	AthleteID int64
	Date      string
}

// RecoveryCheckinInput carries the fields for logging a recovery check-in.
type RecoveryCheckinInput struct {
	Date       string
	SleepHours *float64
	Soreness   *int64
	Energy     *int64
	Notes      string
}

// CreateRecoveryCheckin logs a recovery check-in for an athlete, creating the
// parent workout (discipline='recovery') and the detail row in one transaction.
func CreateRecoveryCheckin(db *sql.DB, athleteID int64, in RecoveryCheckinInput) (*RecoveryCheckin, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin recovery checkin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var workoutID int64
	err = tx.QueryRow(
		`INSERT INTO workouts (athlete_id, date, discipline) VALUES (?, ?, 'recovery') RETURNING id`,
		athleteID, in.Date,
	).Scan(&workoutID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWorkoutExists
		}
		return nil, fmt.Errorf("models: create recovery workout for athlete %d on %s: %w", athleteID, in.Date, err)
	}

	var id int64
	err = tx.QueryRow(
		`INSERT INTO recovery_checkins
		    (workout_id, sleep_hours, soreness, energy, notes)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		workoutID,
		nullableFloat64(in.SleepHours), nullableInt64(in.Soreness),
		nullableInt64(in.Energy), nullableString(in.Notes),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create recovery checkin for athlete %d: %w", athleteID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit recovery checkin: %w", err)
	}

	return GetRecoveryCheckinByID(db, id)
}

// GetRecoveryCheckinByID retrieves a recovery check-in by ID.
func GetRecoveryCheckinByID(db *sql.DB, id int64) (*RecoveryCheckin, error) {
	rc := &RecoveryCheckin{}
	err := db.QueryRow(
		`SELECT rc.id, rc.workout_id, rc.sleep_hours, rc.soreness, rc.energy,
		        rc.notes, rc.created_at, rc.updated_at, w.athlete_id, w.date
		 FROM recovery_checkins rc
		 JOIN workouts w ON w.id = rc.workout_id
		 WHERE rc.id = ?`, id,
	).Scan(&rc.ID, &rc.WorkoutID, &rc.SleepHours, &rc.Soreness, &rc.Energy,
		&rc.Notes, &rc.CreatedAt, &rc.UpdatedAt, &rc.AthleteID, &rc.Date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get recovery checkin %d: %w", id, err)
	}
	return rc, nil
}

// ListRecoveryCheckins returns an athlete's recovery check-ins, newest first.
func ListRecoveryCheckins(db *sql.DB, athleteID int64, limit int) ([]*RecoveryCheckin, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(
		`SELECT rc.id, rc.workout_id, rc.sleep_hours, rc.soreness, rc.energy,
		        rc.notes, rc.created_at, rc.updated_at, w.athlete_id, w.date
		 FROM recovery_checkins rc
		 JOIN workouts w ON w.id = rc.workout_id
		 WHERE w.athlete_id = ?
		 ORDER BY w.date DESC, rc.id DESC
		 LIMIT ?`, athleteID, limit)
	if err != nil {
		return nil, fmt.Errorf("models: list recovery checkins for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var checkins []*RecoveryCheckin
	for rows.Next() {
		rc := &RecoveryCheckin{}
		if err := rows.Scan(&rc.ID, &rc.WorkoutID, &rc.SleepHours, &rc.Soreness, &rc.Energy,
			&rc.Notes, &rc.CreatedAt, &rc.UpdatedAt, &rc.AthleteID, &rc.Date); err != nil {
			return nil, fmt.Errorf("models: scan recovery checkin: %w", err)
		}
		checkins = append(checkins, rc)
	}
	return checkins, rows.Err()
}

// DeleteRecoveryCheckin removes a recovery check-in and its parent workout.
func DeleteRecoveryCheckin(db *sql.DB, id int64) error {
	rc, err := GetRecoveryCheckinByID(db, id)
	if err != nil {
		return err
	}
	result, err := db.Exec(`DELETE FROM workouts WHERE id = ?`, rc.WorkoutID)
	if err != nil {
		return fmt.Errorf("models: delete recovery checkin %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
