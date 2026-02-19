package models

import (
	"database/sql"
	"fmt"
	"time"
)

// TierHistory represents a single tier change event for an athlete.
type TierHistory struct {
	ID            int64
	AthleteID     int64
	Tier          string
	PreviousTier  sql.NullString
	SetBy         sql.NullInt64
	EffectiveDate string // DATE as string (YYYY-MM-DD)
	Notes         sql.NullString
	CreatedAt     time.Time

	// Joined fields populated by list queries.
	SetByName string
}

// RecordTierChange inserts a tier history entry. Called when an athlete's tier
// is changed via edit or promote. previousTier should be the old tier value
// (empty string if none). setByUserID is the user making the change.
func RecordTierChange(db *sql.DB, athleteID int64, tier, previousTier string, setByUserID int64, effectiveDate, notes string) (*TierHistory, error) {
	var prevVal sql.NullString
	if previousTier != "" {
		prevVal = sql.NullString{String: previousTier, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	if effectiveDate == "" {
		effectiveDate = time.Now().Format("2006-01-02")
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO tier_history (athlete_id, tier, previous_tier, set_by, effective_date, notes)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, tier, prevVal, setByUserID, effectiveDate, notesVal,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: record tier change for athlete %d: %w", athleteID, err)
	}

	return GetTierHistoryByID(db, id)
}

// GetTierHistoryByID retrieves a single tier history entry by primary key.
func GetTierHistoryByID(db *sql.DB, id int64) (*TierHistory, error) {
	th := &TierHistory{}
	err := db.QueryRow(
		`SELECT th.id, th.athlete_id, th.tier, th.previous_tier, th.set_by,
		        th.effective_date, th.notes, th.created_at,
		        COALESCE(u.name, u.username, '') AS set_by_name
		 FROM tier_history th
		 LEFT JOIN users u ON u.id = th.set_by
		 WHERE th.id = ?`, id,
	).Scan(&th.ID, &th.AthleteID, &th.Tier, &th.PreviousTier, &th.SetBy,
		&th.EffectiveDate, &th.Notes, &th.CreatedAt, &th.SetByName)
	if err != nil {
		return nil, fmt.Errorf("models: get tier history %d: %w", id, err)
	}
	return th, nil
}

// ListTierHistory returns the tier change history for an athlete, newest first.
func ListTierHistory(db *sql.DB, athleteID int64) ([]*TierHistory, error) {
	rows, err := db.Query(
		`SELECT th.id, th.athlete_id, th.tier, th.previous_tier, th.set_by,
		        th.effective_date, th.notes, th.created_at,
		        COALESCE(u.name, u.username, '') AS set_by_name
		 FROM tier_history th
		 LEFT JOIN users u ON u.id = th.set_by
		 WHERE th.athlete_id = ?
		 ORDER BY th.effective_date DESC, th.created_at DESC
		 LIMIT 100`, athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list tier history for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var history []*TierHistory
	for rows.Next() {
		th := &TierHistory{}
		if err := rows.Scan(&th.ID, &th.AthleteID, &th.Tier, &th.PreviousTier, &th.SetBy,
			&th.EffectiveDate, &th.Notes, &th.CreatedAt, &th.SetByName); err != nil {
			return nil, fmt.Errorf("models: scan tier history: %w", err)
		}
		history = append(history, th)
	}
	return history, rows.Err()
}
