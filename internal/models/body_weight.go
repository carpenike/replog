package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrDuplicateBodyWeight is returned when a body weight entry already exists
// for the same athlete and date.
var ErrDuplicateBodyWeight = errors.New("body weight entry already exists for this date")

// BodyWeight represents a single body weight entry for an athlete.
type BodyWeight struct {
	ID        int64
	AthleteID int64
	Date      string
	Weight    float64
	Notes     sql.NullString
	CreatedAt time.Time
}

// CreateBodyWeight inserts a new body weight record.
func CreateBodyWeight(db *sql.DB, athleteID int64, date string, weight float64, notes string) (*BodyWeight, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO body_weights (athlete_id, date, weight, notes) VALUES (?, ?, ?, ?)`,
		athleteID, date, weight, notesVal,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateBodyWeight
		}
		return nil, fmt.Errorf("models: create body weight for athlete %d: %w", athleteID, err)
	}

	id, _ := result.LastInsertId()
	return GetBodyWeightByID(db, id)
}

// GetBodyWeightByID retrieves a body weight entry by primary key.
func GetBodyWeightByID(db *sql.DB, id int64) (*BodyWeight, error) {
	bw := &BodyWeight{}
	err := db.QueryRow(
		`SELECT id, athlete_id, date, weight, notes, created_at FROM body_weights WHERE id = ?`, id,
	).Scan(&bw.ID, &bw.AthleteID, &bw.Date, &bw.Weight, &bw.Notes, &bw.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get body weight %d: %w", id, err)
	}
	return bw, nil
}

// DeleteBodyWeight removes a body weight entry by ID.
func DeleteBodyWeight(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM body_weights WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete body weight %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// BodyWeightPageSize is the max number of entries returned per page.
const BodyWeightPageSize = 30

// BodyWeightPage holds a page of body weight entries and whether more exist.
type BodyWeightPage struct {
	Entries []*BodyWeight
	HasMore bool
}

// ListBodyWeights returns body weight entries for an athlete, ordered by date
// descending. Uses offset-based pagination.
func ListBodyWeights(db *sql.DB, athleteID int64, offset int) (*BodyWeightPage, error) {
	rows, err := db.Query(`
		SELECT id, athlete_id, date, weight, notes, created_at
		FROM body_weights
		WHERE athlete_id = ?
		ORDER BY date DESC
		LIMIT ? OFFSET ?`, athleteID, BodyWeightPageSize+1, offset)
	if err != nil {
		return nil, fmt.Errorf("models: list body weights for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var entries []*BodyWeight
	for rows.Next() {
		bw := &BodyWeight{}
		if err := rows.Scan(&bw.ID, &bw.AthleteID, &bw.Date, &bw.Weight, &bw.Notes, &bw.CreatedAt); err != nil {
			return nil, fmt.Errorf("models: scan body weight: %w", err)
		}
		entries = append(entries, bw)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(entries) > BodyWeightPageSize
	if hasMore {
		entries = entries[:BodyWeightPageSize]
	}

	return &BodyWeightPage{Entries: entries, HasMore: hasMore}, nil
}

// LatestBodyWeight returns the most recent body weight for an athlete, or nil.
func LatestBodyWeight(db *sql.DB, athleteID int64) (*BodyWeight, error) {
	bw := &BodyWeight{}
	err := db.QueryRow(`
		SELECT id, athlete_id, date, weight, notes, created_at
		FROM body_weights
		WHERE athlete_id = ?
		ORDER BY date DESC
		LIMIT 1`, athleteID,
	).Scan(&bw.ID, &bw.AthleteID, &bw.Date, &bw.Weight, &bw.Notes, &bw.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models: latest body weight for athlete %d: %w", athleteID, err)
	}
	return bw, nil
}
