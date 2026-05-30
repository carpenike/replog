package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ConditioningSession is the detail row for a discipline='conditioning' workout
// (ADR 018). The parent workout carries athlete + date; this row carries the
// conditioning payload (modality, type, distance/duration/HR/RPE). Optional
// per-effort intervals hang off it in conditioning_intervals.
type ConditioningSession struct {
	ID              int64
	WorkoutID       int64
	Modality        string // run | row | bike | sprint | circuit | swim | other
	SessionType     string // steady | interval | sprint | tempo
	TotalDistance   sql.NullFloat64
	DistanceUnit    sql.NullString // m | km | yd | mi
	DurationSeconds sql.NullInt64
	AvgHR           sql.NullInt64
	RPE             sql.NullFloat64
	Notes           sql.NullString
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Joined from the parent workout.
	AthleteID int64
	Date      string

	// Child interval rows, ordered by interval_number.
	Intervals []*ConditioningInterval
}

// ConditioningInterval is one work/rest effort within a conditioning session.
type ConditioningInterval struct {
	ID                    int64
	ConditioningSessionID int64
	IntervalNumber        int64
	WorkSeconds           sql.NullInt64
	WorkDistance          sql.NullFloat64
	RestSeconds           sql.NullInt64
	Notes                 sql.NullString
}

// ConditioningSessionInput carries the fields for logging a conditioning session.
type ConditioningSessionInput struct {
	Date            string
	Modality        string
	SessionType     string
	TotalDistance   *float64
	DistanceUnit    string
	DurationSeconds *int64
	AvgHR           *int64
	RPE             *float64
	Notes           string
	Intervals       []ConditioningIntervalInput
}

// ConditioningIntervalInput carries the fields for one interval.
type ConditioningIntervalInput struct {
	IntervalNumber int64
	WorkSeconds    *int64
	WorkDistance   *float64
	RestSeconds    *int64
	Notes          string
}

// validConditioningModalities mirrors the CHECK on conditioning_sessions.modality.
var validConditioningModalities = map[string]bool{
	"run": true, "row": true, "bike": true, "sprint": true,
	"circuit": true, "swim": true, "other": true,
}

// validConditioningTypes mirrors the CHECK on conditioning_sessions.session_type.
var validConditioningTypes = map[string]bool{
	"steady": true, "interval": true, "sprint": true, "tempo": true,
}

// validDistanceUnits mirrors the CHECK on conditioning_sessions.distance_unit.
var validDistanceUnits = map[string]bool{
	"m": true, "km": true, "yd": true, "mi": true,
}

// CreateConditioningSession logs a conditioning session for an athlete. It
// creates the parent workout (discipline='conditioning'), the conditioning
// detail row, and any interval rows in a single transaction.
func CreateConditioningSession(db *sql.DB, athleteID int64, in ConditioningSessionInput) (*ConditioningSession, error) {
	if !validConditioningModalities[in.Modality] {
		return nil, fmt.Errorf("models: invalid modality %q: %w", in.Modality, ErrInvalidInput)
	}
	if !validConditioningTypes[in.SessionType] {
		return nil, fmt.Errorf("models: invalid session_type %q: %w", in.SessionType, ErrInvalidInput)
	}
	if in.DistanceUnit != "" && !validDistanceUnits[in.DistanceUnit] {
		return nil, fmt.Errorf("models: invalid distance_unit %q: %w", in.DistanceUnit, ErrInvalidInput)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin conditioning session tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var workoutID int64
	err = tx.QueryRow(
		`INSERT INTO workouts (athlete_id, date, discipline) VALUES (?, ?, 'conditioning') RETURNING id`,
		athleteID, in.Date,
	).Scan(&workoutID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWorkoutExists
		}
		return nil, fmt.Errorf("models: create conditioning workout for athlete %d on %s: %w", athleteID, in.Date, err)
	}

	var id int64
	err = tx.QueryRow(
		`INSERT INTO conditioning_sessions
		    (workout_id, modality, session_type, total_distance, distance_unit, duration_seconds, avg_hr, rpe, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		workoutID, in.Modality, in.SessionType,
		nullableFloat64(in.TotalDistance), nullableString(in.DistanceUnit),
		nullableInt64(in.DurationSeconds), nullableInt64(in.AvgHR), nullableFloat64(in.RPE),
		nullableString(in.Notes),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create conditioning session for athlete %d: %w", athleteID, err)
	}

	for _, iv := range in.Intervals {
		_, err = tx.Exec(
			`INSERT INTO conditioning_intervals
			    (conditioning_session_id, interval_number, work_seconds, work_distance, rest_seconds, notes)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			id, iv.IntervalNumber,
			nullableInt64(iv.WorkSeconds), nullableFloat64(iv.WorkDistance),
			nullableInt64(iv.RestSeconds), nullableString(iv.Notes),
		)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, fmt.Errorf("models: duplicate interval_number %d: %w", iv.IntervalNumber, ErrInvalidInput)
			}
			return nil, fmt.Errorf("models: create conditioning interval %d: %w", iv.IntervalNumber, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit conditioning session: %w", err)
	}

	return GetConditioningSessionByID(db, id)
}

// GetConditioningSessionByID retrieves a conditioning session (with parent date
// and ordered intervals) by ID.
func GetConditioningSessionByID(db *sql.DB, id int64) (*ConditioningSession, error) {
	cs := &ConditioningSession{}
	err := db.QueryRow(
		`SELECT cs.id, cs.workout_id, cs.modality, cs.session_type, cs.total_distance,
		        cs.distance_unit, cs.duration_seconds, cs.avg_hr, cs.rpe, cs.notes,
		        cs.created_at, cs.updated_at, w.athlete_id, w.date
		 FROM conditioning_sessions cs
		 JOIN workouts w ON w.id = cs.workout_id
		 WHERE cs.id = ?`, id,
	).Scan(&cs.ID, &cs.WorkoutID, &cs.Modality, &cs.SessionType, &cs.TotalDistance,
		&cs.DistanceUnit, &cs.DurationSeconds, &cs.AvgHR, &cs.RPE, &cs.Notes,
		&cs.CreatedAt, &cs.UpdatedAt, &cs.AthleteID, &cs.Date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get conditioning session %d: %w", id, err)
	}

	intervals, err := listConditioningIntervals(db, id)
	if err != nil {
		return nil, err
	}
	cs.Intervals = intervals
	return cs, nil
}

// listConditioningIntervals returns a session's intervals, ordered.
func listConditioningIntervals(db *sql.DB, sessionID int64) ([]*ConditioningInterval, error) {
	rows, err := db.Query(
		`SELECT id, conditioning_session_id, interval_number, work_seconds, work_distance, rest_seconds, notes
		 FROM conditioning_intervals WHERE conditioning_session_id = ?
		 ORDER BY interval_number ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("models: list conditioning intervals for session %d: %w", sessionID, err)
	}
	defer rows.Close()

	var intervals []*ConditioningInterval
	for rows.Next() {
		iv := &ConditioningInterval{}
		if err := rows.Scan(&iv.ID, &iv.ConditioningSessionID, &iv.IntervalNumber,
			&iv.WorkSeconds, &iv.WorkDistance, &iv.RestSeconds, &iv.Notes); err != nil {
			return nil, fmt.Errorf("models: scan conditioning interval: %w", err)
		}
		intervals = append(intervals, iv)
	}
	return intervals, rows.Err()
}

// ListConditioningSessions returns an athlete's conditioning sessions, newest
// first. Intervals are populated for each.
func ListConditioningSessions(db *sql.DB, athleteID int64, limit int) ([]*ConditioningSession, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(
		`SELECT cs.id, cs.workout_id, cs.modality, cs.session_type, cs.total_distance,
		        cs.distance_unit, cs.duration_seconds, cs.avg_hr, cs.rpe, cs.notes,
		        cs.created_at, cs.updated_at, w.athlete_id, w.date
		 FROM conditioning_sessions cs
		 JOIN workouts w ON w.id = cs.workout_id
		 WHERE w.athlete_id = ?
		 ORDER BY w.date DESC, cs.id DESC
		 LIMIT ?`, athleteID, limit)
	if err != nil {
		return nil, fmt.Errorf("models: list conditioning sessions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var sessions []*ConditioningSession
	for rows.Next() {
		cs := &ConditioningSession{}
		if err := rows.Scan(&cs.ID, &cs.WorkoutID, &cs.Modality, &cs.SessionType, &cs.TotalDistance,
			&cs.DistanceUnit, &cs.DurationSeconds, &cs.AvgHR, &cs.RPE, &cs.Notes,
			&cs.CreatedAt, &cs.UpdatedAt, &cs.AthleteID, &cs.Date); err != nil {
			return nil, fmt.Errorf("models: scan conditioning session: %w", err)
		}
		sessions = append(sessions, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, cs := range sessions {
		intervals, err := listConditioningIntervals(db, cs.ID)
		if err != nil {
			return nil, err
		}
		cs.Intervals = intervals
	}
	return sessions, nil
}

// DeleteConditioningSession removes a conditioning session and its parent
// workout (cascading to the detail row and intervals).
func DeleteConditioningSession(db *sql.DB, id int64) error {
	cs, err := GetConditioningSessionByID(db, id)
	if err != nil {
		return err
	}
	result, err := db.Exec(`DELETE FROM workouts WHERE id = ?`, cs.WorkoutID)
	if err != nil {
		return fmt.Errorf("models: delete conditioning session %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
