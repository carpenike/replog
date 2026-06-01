package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ThrowingSession is the detail row for a discipline='throwing' workout
// (ADR 018). The parent workout carries athlete + date; this row carries the
// arm-care payload (throw type/count/intent/velocity, fatigue/pain flags).
type ThrowingSession struct {
	ID         int64
	WorkoutID  int64
	ThrowType  string // game | bullpen | lesson | long_toss | catch | flat_ground | position
	ThrowCount sql.NullInt64
	MaxIntent  sql.NullInt64
	Velocity   sql.NullFloat64
	Fatigue    bool
	Pain       bool
	Source     string // program | external
	Team       sql.NullString
	Notes      sql.NullString
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Joined from the parent workout.
	AthleteID int64
	Date      string
}

// ThrowingSessionInput carries the fields for logging a throwing session.
type ThrowingSessionInput struct {
	Date       string
	ThrowType  string
	ThrowCount *int64
	MaxIntent  *int64
	Velocity   *float64
	Fatigue    bool
	Pain       bool
	Source     string
	Team       string
	Notes      string
}

// validThrowTypes mirrors the CHECK constraint on throwing_sessions.throw_type.
var validThrowTypes = map[string]bool{
	"game": true, "bullpen": true, "lesson": true,
	"long_toss": true, "catch": true, "flat_ground": true, "position": true,
}

// CreateThrowingSession logs a throwing session for an athlete. It creates the
// parent workout (discipline='throwing') and the throwing_sessions detail row
// in a single transaction. An over-Pitch-Smart-limit session still succeeds —
// limits are advisory (ADR 007/018), never a hard log-block.
func CreateThrowingSession(db *sql.DB, athleteID int64, in ThrowingSessionInput) (*ThrowingSession, error) {
	if !validThrowTypes[in.ThrowType] {
		return nil, fmt.Errorf("models: invalid throw_type %q: %w", in.ThrowType, ErrInvalidInput)
	}
	source := in.Source
	if source == "" {
		source = "program"
	}
	if source != "program" && source != "external" {
		return nil, fmt.Errorf("models: invalid source %q: %w", source, ErrInvalidInput)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin throwing session tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var workoutID int64
	err = tx.QueryRow(
		`INSERT INTO workouts (athlete_id, date, discipline) VALUES (?, ?, 'throwing') RETURNING id`,
		athleteID, in.Date,
	).Scan(&workoutID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWorkoutExists
		}
		return nil, fmt.Errorf("models: create throwing workout for athlete %d on %s: %w", athleteID, in.Date, err)
	}

	var id int64
	err = tx.QueryRow(
		`INSERT INTO throwing_sessions
		    (workout_id, throw_type, throw_count, max_intent, velocity, fatigue, pain, source, team, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		workoutID, in.ThrowType,
		nullableInt64(in.ThrowCount), nullableInt64(in.MaxIntent), nullableFloat64(in.Velocity),
		boolToInt(in.Fatigue), boolToInt(in.Pain), source,
		nullableString(in.Team), nullableString(in.Notes),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create throwing session for athlete %d: %w", athleteID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit throwing session: %w", err)
	}

	return GetThrowingSessionByID(db, id)
}

// GetThrowingSessionByID retrieves a throwing session (with parent date) by ID.
func GetThrowingSessionByID(db *sql.DB, id int64) (*ThrowingSession, error) {
	ts := &ThrowingSession{}
	err := db.QueryRow(
		`SELECT ts.id, ts.workout_id, ts.throw_type, ts.throw_count, ts.max_intent, ts.velocity,
		        ts.fatigue, ts.pain, ts.source, ts.team, ts.notes, ts.created_at, ts.updated_at,
		        w.athlete_id, w.date
		 FROM throwing_sessions ts
		 JOIN workouts w ON w.id = ts.workout_id
		 WHERE ts.id = ?`, id,
	).Scan(&ts.ID, &ts.WorkoutID, &ts.ThrowType, &ts.ThrowCount, &ts.MaxIntent, &ts.Velocity,
		&ts.Fatigue, &ts.Pain, &ts.Source, &ts.Team, &ts.Notes, &ts.CreatedAt, &ts.UpdatedAt,
		&ts.AthleteID, &ts.Date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get throwing session %d: %w", id, err)
	}
	return ts, nil
}

// ListThrowingSessions returns an athlete's throwing sessions, newest first.
func ListThrowingSessions(db *sql.DB, athleteID int64, limit int) ([]*ThrowingSession, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(
		`SELECT ts.id, ts.workout_id, ts.throw_type, ts.throw_count, ts.max_intent, ts.velocity,
		        ts.fatigue, ts.pain, ts.source, ts.team, ts.notes, ts.created_at, ts.updated_at,
		        w.athlete_id, w.date
		 FROM throwing_sessions ts
		 JOIN workouts w ON w.id = ts.workout_id
		 WHERE w.athlete_id = ?
		 ORDER BY w.date DESC, ts.id DESC
		 LIMIT ?`, athleteID, limit)
	if err != nil {
		return nil, fmt.Errorf("models: list throwing sessions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var sessions []*ThrowingSession
	for rows.Next() {
		ts := &ThrowingSession{}
		if err := rows.Scan(&ts.ID, &ts.WorkoutID, &ts.ThrowType, &ts.ThrowCount, &ts.MaxIntent, &ts.Velocity,
			&ts.Fatigue, &ts.Pain, &ts.Source, &ts.Team, &ts.Notes, &ts.CreatedAt, &ts.UpdatedAt,
			&ts.AthleteID, &ts.Date); err != nil {
			return nil, fmt.Errorf("models: scan throwing session: %w", err)
		}
		sessions = append(sessions, ts)
	}
	return sessions, rows.Err()
}

// DeleteThrowingSession removes a throwing session and its parent workout.
func DeleteThrowingSession(db *sql.DB, id int64) error {
	// Deleting the parent workout cascades to the throwing_sessions row.
	ts, err := GetThrowingSessionByID(db, id)
	if err != nil {
		return err
	}
	result, err := db.Exec(`DELETE FROM workouts WHERE id = ?`, ts.WorkoutID)
	if err != nil {
		return fmt.Errorf("models: delete throwing session %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
