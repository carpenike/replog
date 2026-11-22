package models

import (
	"context"
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
	Mood       sql.NullInt64
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
	Mood       *int64
	Notes      string
}

// Readiness thresholds. A check-in at or past any of them is a low-readiness
// flag the coach sees in review. The flag only informs: it never changes
// the plan, the prescription, or a training max.
const (
	ReadinessLowSleepHours = 6 // sleep below this many hours
	ReadinessHighSoreness  = 8 // soreness at or above this (1-10)
	ReadinessLowMood       = 3 // mood or energy at or below this (1-10)
)

// ReadinessLow reports whether the check-in answers flag low readiness.
func (rc *RecoveryCheckin) ReadinessLow() bool {
	return ReadinessIsLow(rc.SleepHours, rc.Soreness, rc.Energy, rc.Mood)
}

// ReadinessIsLow applies the readiness thresholds to nullable answers.
// Unanswered questions never flag.
func ReadinessIsLow(sleep sql.NullFloat64, soreness, energy, mood sql.NullInt64) bool {
	switch {
	case sleep.Valid && sleep.Float64 < ReadinessLowSleepHours:
		return true
	case soreness.Valid && soreness.Int64 >= ReadinessHighSoreness:
		return true
	case energy.Valid && energy.Int64 <= ReadinessLowMood:
		return true
	case mood.Valid && mood.Int64 <= ReadinessLowMood:
		return true
	}
	return false
}

// GetRecoveryCheckinForDate returns the athlete's check-in on date, or
// ErrNotFound when there is none.
func GetRecoveryCheckinForDate(ctx context.Context, db *sql.DB, athleteID int64, date string) (*RecoveryCheckin, error) {
	var id int64
	err := db.QueryRowContext(ctx,
		`SELECT rc.id FROM recovery_checkins rc
		 JOIN workouts w ON w.id = rc.workout_id
		 WHERE w.athlete_id = ? AND date(w.date) = date(?) AND w.discipline = 'recovery'
		 ORDER BY rc.id DESC LIMIT 1`, athleteID, date,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: recovery checkin for athlete %d on %s: %w", athleteID, date, err)
	}
	return GetRecoveryCheckinByID(ctx, db, id)
}

// CreateRecoveryCheckin logs a recovery check-in for an athlete, creating the
// parent workout (discipline='recovery') and the detail row in one transaction.
func CreateRecoveryCheckin(ctx context.Context, db *sql.DB, athleteID int64, in RecoveryCheckinInput) (*RecoveryCheckin, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: begin recovery checkin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var workoutID int64
	err = tx.QueryRowContext(ctx,
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
	err = tx.QueryRowContext(ctx,
		`INSERT INTO recovery_checkins
		    (workout_id, sleep_hours, soreness, energy, mood, notes)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		workoutID,
		nullableFloat64(in.SleepHours), nullableInt64(in.Soreness),
		nullableInt64(in.Energy), nullableInt64(in.Mood), nullableString(in.Notes),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create recovery checkin for athlete %d: %w", athleteID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit recovery checkin: %w", err)
	}

	return GetRecoveryCheckinByID(ctx, db, id)
}

// GetRecoveryCheckinByID retrieves a recovery check-in by ID.
func GetRecoveryCheckinByID(ctx context.Context, db *sql.DB, id int64) (*RecoveryCheckin, error) {
	rc := &RecoveryCheckin{}
	err := db.QueryRowContext(ctx,
		`SELECT rc.id, rc.workout_id, rc.sleep_hours, rc.soreness, rc.energy, rc.mood,
		        rc.notes, rc.created_at, rc.updated_at, w.athlete_id, w.date
		 FROM recovery_checkins rc
		 JOIN workouts w ON w.id = rc.workout_id
		 WHERE rc.id = ?`, id,
	).Scan(&rc.ID, &rc.WorkoutID, &rc.SleepHours, &rc.Soreness, &rc.Energy, &rc.Mood,
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
func ListRecoveryCheckins(ctx context.Context, db *sql.DB, athleteID int64, limit int) ([]*RecoveryCheckin, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx,
		`SELECT rc.id, rc.workout_id, rc.sleep_hours, rc.soreness, rc.energy, rc.mood,
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
		if err := rows.Scan(&rc.ID, &rc.WorkoutID, &rc.SleepHours, &rc.Soreness, &rc.Energy, &rc.Mood,
			&rc.Notes, &rc.CreatedAt, &rc.UpdatedAt, &rc.AthleteID, &rc.Date); err != nil {
			return nil, fmt.Errorf("models: scan recovery checkin: %w", err)
		}
		checkins = append(checkins, rc)
	}
	return checkins, rows.Err()
}

// DeleteRecoveryCheckin removes a recovery check-in and its parent workout.
func DeleteRecoveryCheckin(ctx context.Context, db *sql.DB, id int64) error {
	rc, err := GetRecoveryCheckinByID(ctx, db, id)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM workouts WHERE id = ?`, rc.WorkoutID)
	if err != nil {
		return fmt.Errorf("models: delete recovery checkin %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
