package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrProgramAlreadyActive is returned when assigning a program to an athlete who already has one.
var ErrProgramAlreadyActive = errors.New("athlete already has an active program")

// AthleteProgram links an athlete to a program template.
type AthleteProgram struct {
	ID         int64
	AthleteID  int64
	TemplateID int64
	StartDate  string // DATE as YYYY-MM-DD
	Active     bool
	Notes      sql.NullString
	Goal       sql.NullString // short-term cycle goal
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Joined fields.
	TemplateName string
	NumWeeks     int
	NumDays      int
}

// AssignProgram assigns a program template to an athlete. Only one active program per athlete.
func AssignProgram(db *sql.DB, athleteID, templateID int64, startDate, notes, goal string) (*AthleteProgram, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var goalVal sql.NullString
	if goal != "" {
		goalVal = sql.NullString{String: goal, Valid: true}
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO athlete_programs (athlete_id, template_id, start_date, notes, goal) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		athleteID, templateID, startDate, notesVal, goalVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProgramAlreadyActive
		}
		return nil, fmt.Errorf("models: assign program to athlete %d: %w", athleteID, err)
	}

	return GetAthleteProgramByID(db, id)
}

// GetAthleteProgramByID retrieves an athlete program by primary key.
func GetAthleteProgramByID(db *sql.DB, id int64) (*AthleteProgram, error) {
	ap := &AthleteProgram{}
	err := db.QueryRow(
		`SELECT ap.id, ap.athlete_id, ap.template_id, ap.start_date, ap.active, ap.notes, ap.goal,
		        ap.created_at, ap.updated_at, pt.name, pt.num_weeks, pt.num_days
		 FROM athlete_programs ap
		 JOIN program_templates pt ON pt.id = ap.template_id
		 WHERE ap.id = ?`,
		id,
	).Scan(&ap.ID, &ap.AthleteID, &ap.TemplateID, &ap.StartDate, &ap.Active, &ap.Notes, &ap.Goal,
		&ap.CreatedAt, &ap.UpdatedAt, &ap.TemplateName, &ap.NumWeeks, &ap.NumDays)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("models: athlete program %d not found", id)
		}
		return nil, fmt.Errorf("models: get athlete program %d: %w", id, err)
	}
	return ap, nil
}

// GetActiveProgram returns the current active program for an athlete, or nil if none.
func GetActiveProgram(db *sql.DB, athleteID int64) (*AthleteProgram, error) {
	ap := &AthleteProgram{}
	err := db.QueryRow(
		`SELECT ap.id, ap.athlete_id, ap.template_id, ap.start_date, ap.active, ap.notes, ap.goal,
		        ap.created_at, ap.updated_at, pt.name, pt.num_weeks, pt.num_days
		 FROM athlete_programs ap
		 JOIN program_templates pt ON pt.id = ap.template_id
		 WHERE ap.athlete_id = ? AND ap.active = 1`,
		athleteID,
	).Scan(&ap.ID, &ap.AthleteID, &ap.TemplateID, &ap.StartDate, &ap.Active, &ap.Notes, &ap.Goal,
		&ap.CreatedAt, &ap.UpdatedAt, &ap.TemplateName, &ap.NumWeeks, &ap.NumDays)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No active program is not an error.
		}
		return nil, fmt.Errorf("models: get active program for athlete %d: %w", athleteID, err)
	}
	return ap, nil
}

// DeactivateProgram deactivates an athlete's program.
func DeactivateProgram(db *sql.DB, athleteProgramID int64) error {
	_, err := db.Exec(
		`UPDATE athlete_programs SET active = 0 WHERE id = ?`,
		athleteProgramID,
	)
	if err != nil {
		return fmt.Errorf("models: deactivate athlete program %d: %w", athleteProgramID, err)
	}
	return nil
}
