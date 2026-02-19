package models

import (
	"database/sql"
	"fmt"
	"time"
)

// GoalHistory represents a single goal change event for an athlete.
type GoalHistory struct {
	ID            int64
	AthleteID     int64
	Goal          string
	PreviousGoal  sql.NullString
	SetBy         sql.NullInt64
	EffectiveDate string // DATE as string (YYYY-MM-DD)
	Notes         sql.NullString
	CreatedAt     time.Time

	// Joined fields populated by list queries.
	SetByName string
}

// RecordGoalChange inserts a goal history entry. Called when an athlete's goal
// is created or changed. previousGoal should be the old goal value (empty string
// if none). setByUserID is the user making the change.
func RecordGoalChange(db *sql.DB, athleteID int64, goal, previousGoal string, setByUserID int64, effectiveDate, notes string) (*GoalHistory, error) {
	var prevVal sql.NullString
	if previousGoal != "" {
		prevVal = sql.NullString{String: previousGoal, Valid: true}
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
		`INSERT INTO goal_history (athlete_id, goal, previous_goal, set_by, effective_date, notes)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, goal, prevVal, setByUserID, effectiveDate, notesVal,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("models: record goal change for athlete %d: %w", athleteID, err)
	}

	return GetGoalHistoryByID(db, id)
}

// GetGoalHistoryByID retrieves a single goal history entry by primary key.
func GetGoalHistoryByID(db *sql.DB, id int64) (*GoalHistory, error) {
	gh := &GoalHistory{}
	err := db.QueryRow(
		`SELECT gh.id, gh.athlete_id, gh.goal, gh.previous_goal, gh.set_by,
		        gh.effective_date, gh.notes, gh.created_at,
		        COALESCE(u.name, u.username, '') AS set_by_name
		 FROM goal_history gh
		 LEFT JOIN users u ON u.id = gh.set_by
		 WHERE gh.id = ?`, id,
	).Scan(&gh.ID, &gh.AthleteID, &gh.Goal, &gh.PreviousGoal, &gh.SetBy,
		&gh.EffectiveDate, &gh.Notes, &gh.CreatedAt, &gh.SetByName)
	if err != nil {
		return nil, fmt.Errorf("models: get goal history %d: %w", id, err)
	}
	return gh, nil
}

// ListGoalHistory returns the goal change history for an athlete, newest first.
func ListGoalHistory(db *sql.DB, athleteID int64) ([]*GoalHistory, error) {
	rows, err := db.Query(
		`SELECT gh.id, gh.athlete_id, gh.goal, gh.previous_goal, gh.set_by,
		        gh.effective_date, gh.notes, gh.created_at,
		        COALESCE(u.name, u.username, '') AS set_by_name
		 FROM goal_history gh
		 LEFT JOIN users u ON u.id = gh.set_by
		 WHERE gh.athlete_id = ?
		 ORDER BY gh.effective_date DESC, gh.created_at DESC
		 LIMIT 100`, athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list goal history for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var history []*GoalHistory
	for rows.Next() {
		gh := &GoalHistory{}
		if err := rows.Scan(&gh.ID, &gh.AthleteID, &gh.Goal, &gh.PreviousGoal, &gh.SetBy,
			&gh.EffectiveDate, &gh.Notes, &gh.CreatedAt, &gh.SetByName); err != nil {
			return nil, fmt.Errorf("models: scan goal history: %w", err)
		}
		history = append(history, gh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate goal history: %w", err)
	}
	return history, nil
}
