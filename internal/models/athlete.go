package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Athlete represents a training subject in the system.
type Athlete struct {
	ID                int64
	Name              string
	Tier              sql.NullString
	Notes             sql.NullString
	Goal              sql.NullString // long-term athlete goal
	CoachID           sql.NullInt64
	TrackBodyWeight   bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ActiveAssignments int // populated by list queries
}

// AthleteCardInfo holds enriched data for displaying athlete cards.
type AthleteCardInfo struct {
	ID                int64
	Name              string
	Tier              sql.NullString
	ActiveAssignments int
	LastWorkoutDate   sql.NullString // most recent workout date, null if none
	WeekStreak        int            // consecutive weeks with a workout
	BWTrend           string         // "up", "down", "flat", or "" if insufficient data
	TrackBodyWeight   bool           // whether body weight tracking is enabled
}

// CreateAthlete inserts a new athlete. coachID links the athlete to a coach.
func CreateAthlete(db *sql.DB, name, tier, notes, goal string, coachID sql.NullInt64) (*Athlete, error) {
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var goalVal sql.NullString
	if goal != "" {
		goalVal = sql.NullString{String: goal, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO athletes (name, tier, notes, goal, coach_id, track_body_weight) VALUES (?, ?, ?, ?, ?, 1)`,
		name, tierVal, notesVal, goalVal, coachID,
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
		`SELECT a.id, a.name, a.tier, a.notes, a.goal, a.coach_id, a.track_body_weight,
		        a.created_at, a.updated_at,
		        COALESCE((SELECT COUNT(*) FROM athlete_exercises ae
		                  WHERE ae.athlete_id = a.id AND ae.active = 1), 0)
		 FROM athletes a WHERE a.id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.Tier, &a.Notes, &a.Goal, &a.CoachID, &a.TrackBodyWeight,
		&a.CreatedAt, &a.UpdatedAt, &a.ActiveAssignments)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get athlete %d: %w", id, err)
	}
	return a, nil
}

// UpdateAthlete modifies an existing athlete's fields.
func UpdateAthlete(db *sql.DB, id int64, name, tier, notes, goal string, coachID sql.NullInt64, trackBodyWeight bool) (*Athlete, error) {
	var tierVal sql.NullString
	if tier != "" {
		tierVal = sql.NullString{String: tier, Valid: true}
	}
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var goalVal sql.NullString
	if goal != "" {
		goalVal = sql.NullString{String: goal, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE athletes SET name = ?, tier = ?, notes = ?, goal = ?, coach_id = ?, track_body_weight = ? WHERE id = ?`,
		name, tierVal, notesVal, goalVal, coachID, trackBodyWeight, id,
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

// NextTier returns the tier that follows the given tier value.
// Returns ("", false) if the athlete is already at the highest tier or has no tier.
func NextTier(current string) (string, bool) {
	switch current {
	case "foundational":
		return "intermediate", true
	case "intermediate":
		return "sport_performance", true
	default:
		return "", false
	}
}

// PromoteAthlete advances an athlete to the next tier. Returns ErrNotFound
// if the athlete doesn't exist, or an error if the athlete can't be promoted.
func PromoteAthlete(db *sql.DB, id int64) (*Athlete, error) {
	athlete, err := GetAthleteByID(db, id)
	if err != nil {
		return nil, err
	}

	if !athlete.Tier.Valid {
		return nil, fmt.Errorf("models: promote athlete %d: %w (no tier set)", id, ErrInvalidInput)
	}

	next, ok := NextTier(athlete.Tier.String)
	if !ok {
		return nil, fmt.Errorf("models: promote athlete %d: %w (already at highest tier)", id, ErrInvalidInput)
	}

	_, err = db.Exec(`UPDATE athletes SET tier = ? WHERE id = ?`, next, id)
	if err != nil {
		return nil, fmt.Errorf("models: promote athlete %d: %w", id, err)
	}

	return GetAthleteByID(db, id)
}

// ListAthletes returns athletes with their active assignment count.
// If coachID is valid, only returns athletes belonging to that coach.
// Pass sql.NullInt64{} (invalid) to return all athletes (admin view).
func ListAthletes(db *sql.DB, coachID sql.NullInt64) ([]*Athlete, error) {
	var rows *sql.Rows
	var err error
	if coachID.Valid {
		rows, err = db.Query(`
			SELECT a.id, a.name, a.tier, a.notes, a.goal, a.coach_id, a.track_body_weight,
			       a.created_at, a.updated_at,
			       COALESCE((SELECT COUNT(*) FROM athlete_exercises ae
			                 WHERE ae.athlete_id = a.id AND ae.active = 1), 0) AS active_assignments
			FROM athletes a
			WHERE a.coach_id = ?
			ORDER BY a.name COLLATE NOCASE
			LIMIT 100`, coachID.Int64)
	} else {
		rows, err = db.Query(`
			SELECT a.id, a.name, a.tier, a.notes, a.goal, a.coach_id, a.track_body_weight,
			       a.created_at, a.updated_at,
			       COALESCE((SELECT COUNT(*) FROM athlete_exercises ae
			                 WHERE ae.athlete_id = a.id AND ae.active = 1), 0) AS active_assignments
			FROM athletes a
			ORDER BY a.name COLLATE NOCASE
			LIMIT 100`)
	}
	if err != nil {
		return nil, fmt.Errorf("models: list athletes: %w", err)
	}
	defer rows.Close()

	var athletes []*Athlete
	for rows.Next() {
		a := &Athlete{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Tier, &a.Notes, &a.Goal, &a.CoachID, &a.TrackBodyWeight,
			&a.CreatedAt, &a.UpdatedAt, &a.ActiveAssignments); err != nil {
			return nil, fmt.Errorf("models: scan athlete: %w", err)
		}
		athletes = append(athletes, a)
	}
	return athletes, rows.Err()
}

// ListAvailableAthletes returns athletes not yet linked to any user, plus the
// athlete with exceptAthleteID (so the current link shows in an edit form).
// Pass 0 for exceptAthleteID to exclude no one extra.
func ListAvailableAthletes(db *sql.DB, exceptAthleteID int64) ([]*Athlete, error) {
	rows, err := db.Query(`
		SELECT a.id, a.name, a.tier, a.notes, a.goal, a.coach_id, a.track_body_weight,
		       a.created_at, a.updated_at,
		       COALESCE((SELECT COUNT(*) FROM athlete_exercises ae
		                 WHERE ae.athlete_id = a.id AND ae.active = 1), 0) AS active_assignments
		FROM athletes a
		WHERE a.id NOT IN (SELECT u.athlete_id FROM users u WHERE u.athlete_id IS NOT NULL)
		   OR a.id = ?
		ORDER BY a.name COLLATE NOCASE
		LIMIT 100`, exceptAthleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list available athletes: %w", err)
	}
	defer rows.Close()

	var athletes []*Athlete
	for rows.Next() {
		a := &Athlete{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Tier, &a.Notes, &a.Goal, &a.CoachID, &a.TrackBodyWeight,
			&a.CreatedAt, &a.UpdatedAt, &a.ActiveAssignments); err != nil {
			return nil, fmt.Errorf("models: scan available athlete: %w", err)
		}
		athletes = append(athletes, a)
	}
	return athletes, rows.Err()
}

// ListAthleteCards returns enriched athlete data for the athlete list view.
// Includes last workout date, week streak, and body weight trend.
// If coachID is valid, only returns athletes belonging to that coach.
// Pass sql.NullInt64{} (invalid) to return all athletes (admin view).
func ListAthleteCards(db *sql.DB, coachID sql.NullInt64) ([]*AthleteCardInfo, error) {
	var rows *sql.Rows
	var err error
	if coachID.Valid {
		rows, err = db.Query(`
			SELECT a.id, a.name, a.tier,
			       COALESCE((SELECT COUNT(*) FROM athlete_exercises ae
			                 WHERE ae.athlete_id = a.id AND ae.active = 1), 0) AS active_assignments,
			       (SELECT date(w.date) FROM workouts w WHERE w.athlete_id = a.id ORDER BY w.date DESC LIMIT 1) AS last_workout,
			       a.track_body_weight
			FROM athletes a
			WHERE a.coach_id = ?
			ORDER BY a.name COLLATE NOCASE
			LIMIT 100`, coachID.Int64)
	} else {
		rows, err = db.Query(`
			SELECT a.id, a.name, a.tier,
			       COALESCE((SELECT COUNT(*) FROM athlete_exercises ae
			                 WHERE ae.athlete_id = a.id AND ae.active = 1), 0) AS active_assignments,
			       (SELECT date(w.date) FROM workouts w WHERE w.athlete_id = a.id ORDER BY w.date DESC LIMIT 1) AS last_workout,
			       a.track_body_weight
			FROM athletes a
			ORDER BY a.name COLLATE NOCASE
			LIMIT 100`)
	}
	if err != nil {
		return nil, fmt.Errorf("models: list athlete cards: %w", err)
	}
	defer rows.Close()

	var cards []*AthleteCardInfo
	for rows.Next() {
		c := &AthleteCardInfo{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Tier, &c.ActiveAssignments, &c.LastWorkoutDate, &c.TrackBodyWeight); err != nil {
			return nil, fmt.Errorf("models: scan athlete card: %w", err)
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Enrich each card with streak and BW trend.
	for _, c := range cards {
		c.WeekStreak = athleteWeekStreak(db, c.ID)
		if c.TrackBodyWeight {
			c.BWTrend = athleteBWTrend(db, c.ID)
		}
	}

	return cards, nil
}

// athleteWeekStreak counts consecutive weeks (ending this week) with at least one workout.
func athleteWeekStreak(db *sql.DB, athleteID int64) int {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -(weekday - 1))
	streak := 0
	for i := 0; i < 52; i++ {
		weekStart := monday.AddDate(0, 0, -7*i)
		weekEnd := weekStart.AddDate(0, 0, 7)
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM workouts
			WHERE athlete_id = ? AND date(date) >= date(?) AND date(date) < date(?)`,
			athleteID, weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")).Scan(&count)
		if err != nil || count == 0 {
			break
		}
		streak++
	}
	return streak
}

// athleteBWTrend returns "up", "down", or "flat" based on the last 3 body weight entries.
func athleteBWTrend(db *sql.DB, athleteID int64) string {
	rows, err := db.Query(`
		SELECT weight FROM body_weights
		WHERE athlete_id = ?
		ORDER BY date DESC
		LIMIT 3`, athleteID)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var weights []float64
	for rows.Next() {
		var w float64
		if err := rows.Scan(&w); err != nil {
			return ""
		}
		weights = append(weights, w)
	}
	if len(weights) < 2 {
		return ""
	}

	newest := weights[0]
	oldest := weights[len(weights)-1]
	diff := newest - oldest
	if diff > 0.5 {
		return "up"
	} else if diff < -0.5 {
		return "down"
	}
	return "flat"
}
