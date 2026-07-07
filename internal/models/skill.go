package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SkillSession is the detail row for a discipline='skill' workout (ADR 018) —
// sport-skill work such as batting, fielding, agility, or med-ball throws. The
// parent workout carries athlete + date. `load_kg` records implement load as a
// youth-safety datum; it is logged data, never a prescribed target.
type SkillSession struct {
	ID              int64
	WorkoutID       int64
	SkillType       string // batting | fielding | throwing_accuracy | agility | medball | sprint | other
	RepCount        sql.NullInt64
	LoadKg          sql.NullFloat64
	Velocity        sql.NullFloat64
	DurationSeconds sql.NullInt64
	Notes           sql.NullString
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Joined from the parent workout.
	AthleteID int64
	Date      string
}

// SkillSessionInput carries the fields for logging a skill session.
type SkillSessionInput struct {
	Date            string
	SkillType       string
	RepCount        *int64
	LoadKg          *float64
	Velocity        *float64
	DurationSeconds *int64
	Notes           string
}

// validSkillTypes mirrors the CHECK on skill_sessions.skill_type.
var validSkillTypes = map[string]bool{
	"batting": true, "fielding": true, "throwing_accuracy": true,
	"agility": true, "medball": true, "sprint": true, "other": true,
}

// CreateSkillSession logs a skill session for an athlete, creating the parent
// workout (discipline='skill') and the detail row in one transaction.
func CreateSkillSession(ctx context.Context, db *sql.DB, athleteID int64, in SkillSessionInput) (*SkillSession, error) {
	if !validSkillTypes[in.SkillType] {
		return nil, fmt.Errorf("models: invalid skill_type %q: %w", in.SkillType, ErrInvalidInput)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: begin skill session tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var workoutID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO workouts (athlete_id, date, discipline) VALUES (?, ?, 'skill') RETURNING id`,
		athleteID, in.Date,
	).Scan(&workoutID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWorkoutExists
		}
		return nil, fmt.Errorf("models: create skill workout for athlete %d on %s: %w", athleteID, in.Date, err)
	}

	var id int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO skill_sessions
		    (workout_id, skill_type, rep_count, load_kg, velocity, duration_seconds, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		workoutID, in.SkillType,
		nullableInt64(in.RepCount), nullableFloat64(in.LoadKg), nullableFloat64(in.Velocity),
		nullableInt64(in.DurationSeconds), nullableString(in.Notes),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create skill session for athlete %d: %w", athleteID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit skill session: %w", err)
	}

	return GetSkillSessionByID(ctx, db, id)
}

// GetSkillSessionByID retrieves a skill session by ID.
func GetSkillSessionByID(ctx context.Context, db *sql.DB, id int64) (*SkillSession, error) {
	ss := &SkillSession{}
	err := db.QueryRowContext(ctx,
		`SELECT ss.id, ss.workout_id, ss.skill_type, ss.rep_count, ss.load_kg,
		        ss.velocity, ss.duration_seconds, ss.notes, ss.created_at, ss.updated_at,
		        w.athlete_id, w.date
		 FROM skill_sessions ss
		 JOIN workouts w ON w.id = ss.workout_id
		 WHERE ss.id = ?`, id,
	).Scan(&ss.ID, &ss.WorkoutID, &ss.SkillType, &ss.RepCount, &ss.LoadKg,
		&ss.Velocity, &ss.DurationSeconds, &ss.Notes, &ss.CreatedAt, &ss.UpdatedAt,
		&ss.AthleteID, &ss.Date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get skill session %d: %w", id, err)
	}
	return ss, nil
}

// ListSkillSessions returns an athlete's skill sessions, newest first.
func ListSkillSessions(ctx context.Context, db *sql.DB, athleteID int64, limit int) ([]*SkillSession, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx,
		`SELECT ss.id, ss.workout_id, ss.skill_type, ss.rep_count, ss.load_kg,
		        ss.velocity, ss.duration_seconds, ss.notes, ss.created_at, ss.updated_at,
		        w.athlete_id, w.date
		 FROM skill_sessions ss
		 JOIN workouts w ON w.id = ss.workout_id
		 WHERE w.athlete_id = ?
		 ORDER BY w.date DESC, ss.id DESC
		 LIMIT ?`, athleteID, limit)
	if err != nil {
		return nil, fmt.Errorf("models: list skill sessions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var sessions []*SkillSession
	for rows.Next() {
		ss := &SkillSession{}
		if err := rows.Scan(&ss.ID, &ss.WorkoutID, &ss.SkillType, &ss.RepCount, &ss.LoadKg,
			&ss.Velocity, &ss.DurationSeconds, &ss.Notes, &ss.CreatedAt, &ss.UpdatedAt,
			&ss.AthleteID, &ss.Date); err != nil {
			return nil, fmt.Errorf("models: scan skill session: %w", err)
		}
		sessions = append(sessions, ss)
	}
	return sessions, rows.Err()
}

// DeleteSkillSession removes a skill session and its parent workout.
func DeleteSkillSession(ctx context.Context, db *sql.DB, id int64) error {
	ss, err := GetSkillSessionByID(ctx, db, id)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM workouts WHERE id = ?`, ss.WorkoutID)
	if err != nil {
		return fmt.Errorf("models: delete skill session %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
