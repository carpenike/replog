package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SeasonPhase is a coach-recorded off/pre/in-season window for an athlete and
// sport (ADR 018). It surfaces on the journal and contextualizes training load.
type SeasonPhase struct {
	ID        int64
	AthleteID int64
	Sport     sql.NullString
	Phase     string // off | pre | in
	StartDate string
	EndDate   sql.NullString
	Notes     sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SeasonPhaseInput carries the fields for creating a season phase.
type SeasonPhaseInput struct {
	Sport     string
	Phase     string
	StartDate string
	EndDate   string
	Notes     string
}

var validSeasonPhases = map[string]bool{"off": true, "pre": true, "in": true}

// CreateSeasonPhase records a season phase for an athlete.
func CreateSeasonPhase(db *sql.DB, athleteID int64, in SeasonPhaseInput) (*SeasonPhase, error) {
	if !validSeasonPhases[in.Phase] {
		return nil, fmt.Errorf("models: invalid phase %q: %w", in.Phase, ErrInvalidInput)
	}
	if in.StartDate == "" {
		return nil, fmt.Errorf("models: season phase start_date required: %w", ErrInvalidInput)
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO athlete_season_phases (athlete_id, sport, phase, start_date, end_date, notes)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, nullableString(in.Sport), in.Phase, in.StartDate,
		nullableString(in.EndDate), nullableString(in.Notes),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: create season phase for athlete %d: %w", athleteID, err)
	}
	return GetSeasonPhaseByID(db, id)
}

// GetSeasonPhaseByID retrieves a season phase by primary key.
func GetSeasonPhaseByID(db *sql.DB, id int64) (*SeasonPhase, error) {
	sp := &SeasonPhase{}
	err := db.QueryRow(
		`SELECT id, athlete_id, sport, phase, start_date, end_date, notes, created_at, updated_at
		 FROM athlete_season_phases WHERE id = ?`, id,
	).Scan(&sp.ID, &sp.AthleteID, &sp.Sport, &sp.Phase, &sp.StartDate, &sp.EndDate, &sp.Notes, &sp.CreatedAt, &sp.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get season phase %d: %w", id, err)
	}
	return sp, nil
}

// ListSeasonPhases returns an athlete's season phases, newest start first.
func ListSeasonPhases(db *sql.DB, athleteID int64) ([]*SeasonPhase, error) {
	rows, err := db.Query(
		`SELECT id, athlete_id, sport, phase, start_date, end_date, notes, created_at, updated_at
		 FROM athlete_season_phases WHERE athlete_id = ?
		 ORDER BY start_date DESC, id DESC`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list season phases for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var phases []*SeasonPhase
	for rows.Next() {
		sp := &SeasonPhase{}
		if err := rows.Scan(&sp.ID, &sp.AthleteID, &sp.Sport, &sp.Phase, &sp.StartDate, &sp.EndDate, &sp.Notes, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("models: scan season phase: %w", err)
		}
		phases = append(phases, sp)
	}
	return phases, rows.Err()
}

// DeleteSeasonPhase removes a season phase by ID.
func DeleteSeasonPhase(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM athlete_season_phases WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete season phase %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
