package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Default preference values.
const (
	DefaultWeightUnit = "lbs"
	DefaultTimezone   = "America/New_York"
	DefaultDateFormat = "Jan 2, 2006"
)

// ValidWeightUnits lists acceptable values for weight_unit.
var ValidWeightUnits = []string{"lbs", "kg"}

// ValidDateFormats maps display labels to Go format strings.
var ValidDateFormats = map[string]string{
	"Jan 2, 2006":   "Jan 2, 2006",
	"2006-01-02":    "2006-01-02",
	"02/01/2006":    "02/01/2006",
	"01/02/2006":    "01/02/2006",
	"2 Jan 2006":    "2 Jan 2006",
	"Monday, Jan 2": "Monday, Jan 2",
}

// UserPreferences represents a user's display and locale preferences.
type UserPreferences struct {
	ID         int64
	UserID     int64
	WeightUnit string
	Timezone   string
	DateFormat string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// WeightLabel returns the display label for the user's weight unit.
func (p *UserPreferences) WeightLabel() string {
	return p.WeightUnit
}

// GetUserPreferences retrieves preferences for a user. If none exist, returns defaults.
func GetUserPreferences(db *sql.DB, userID int64) (*UserPreferences, error) {
	p := &UserPreferences{}
	err := db.QueryRow(
		`SELECT id, user_id, weight_unit, timezone, date_format, created_at, updated_at
		 FROM user_preferences WHERE user_id = ?`, userID,
	).Scan(&p.ID, &p.UserID, &p.WeightUnit, &p.Timezone, &p.DateFormat, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// Return sensible defaults when no row exists.
		return &UserPreferences{
			UserID:     userID,
			WeightUnit: DefaultWeightUnit,
			Timezone:   DefaultTimezone,
			DateFormat: DefaultDateFormat,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models: get preferences for user %d: %w", userID, err)
	}
	return p, nil
}

// UpsertUserPreferences creates or updates a user's preferences.
func UpsertUserPreferences(db *sql.DB, userID int64, weightUnit, timezone, dateFormat string) (*UserPreferences, error) {
	if !isValidWeightUnit(weightUnit) {
		return nil, fmt.Errorf("models: invalid weight unit %q: %w", weightUnit, ErrInvalidInput)
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("models: invalid timezone %q: %w", timezone, ErrInvalidInput)
	}
	if !isValidDateFormat(dateFormat) {
		return nil, fmt.Errorf("models: invalid date format %q: %w", dateFormat, ErrInvalidInput)
	}

	_, err := db.Exec(
		`INSERT INTO user_preferences (user_id, weight_unit, timezone, date_format)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   weight_unit = excluded.weight_unit,
		   timezone = excluded.timezone,
		   date_format = excluded.date_format`,
		userID, weightUnit, timezone, dateFormat,
	)
	if err != nil {
		return nil, fmt.Errorf("models: upsert preferences for user %d: %w", userID, err)
	}
	return GetUserPreferences(db, userID)
}

// EnsureUserPreferences creates default preferences for a user if they don't exist.
func EnsureUserPreferences(db *sql.DB, userID int64) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO user_preferences (user_id) VALUES (?)`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("models: ensure preferences for user %d: %w", userID, err)
	}
	return nil
}

func isValidWeightUnit(unit string) bool {
	for _, v := range ValidWeightUnits {
		if v == unit {
			return true
		}
	}
	return false
}

func isValidDateFormat(format string) bool {
	for _, v := range ValidDateFormats {
		if v == format {
			return true
		}
	}
	return false
}
