package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Athlete represents a training subject in the system.
type Athlete struct {
	ID               int64
	Name             string
	Tier             sql.NullString
	Notes            sql.NullString
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ActiveAssignments int // populated by list queries
}

// CreateAthlete inserts a new athlete.
func CreateAthlete(db *sql.DB, name, tier, notes string) (*Athlete, error) {
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO athletes (name, tier, notes) VALUES (?, ?, ?)`,
		name, tierVal, notesVal,
	)
	if err != nil {
		return nil, fmt.Errorf("models: create athlete %q: %w", name, err)
	}

	id, _ := result.LastInsertId()
	return GetAthleteByID(db, id)
}

// GetAthleteByID retrieves an athlete by primary key.
func GetAthleteByID(db *sql.DB, id int64) (*Athlete, error) {
	a := &Athlete{}
	err := db.QueryRow(
		`SELECT id, name, tier, notes, created_at, updated_at
		 FROM athletes WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.Tier, &a.Notes, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get athlete %d: %w", id, err)
	}
	return a, nil
}

// UpdateAthlete modifies an existing athlete's fields.
func UpdateAthlete(db *sql.DB, id int64, name, tier, notes string) (*Athlete, error) {
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE athletes SET name = ?, tier = ?, notes = ? WHERE id = ?`,
		name, tierVal, notesVal, id,
	)
	if err != nil {
		return nil, fmt.Errorf("models: update athlete %d: %w", id, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, ErrNotFound
	}

	return GetAthleteByID(db, id)
}

// DeleteAthlete removes an athlete by ID. CASCADE deletes their workouts,
// assignments, and training maxes.
func DeleteAthlete(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM athletes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete athlete %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAthletes returns all athletes with their active assignment count.
func ListAthletes(db *sql.DB) ([]*Athlete, error) {
	rows, err := db.Query(`
		SELECT a.id, a.name, a.tier, a.notes, a.created_at, a.updated_at,
		       COALESCE((SELECT COUNT(*) FROM athlete_exercises ae
		                 WHERE ae.athlete_id = a.id AND ae.active = 1), 0) AS active_assignments
		FROM athletes a
		ORDER BY a.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("models: list athletes: %w", err)
	}
	defer rows.Close()

	var athletes []*Athlete
	for rows.Next() {
		a := &Athlete{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Tier, &a.Notes, &a.CreatedAt, &a.UpdatedAt, &a.ActiveAssignments); err != nil {
			return nil, fmt.Errorf("models: scan athlete: %w", err)
		}
		athletes = append(athletes, a)
	}
	return athletes, rows.Err()
}
