package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrProgramAlreadyActive is returned when assigning a primary program to an athlete who already has one.
var ErrProgramAlreadyActive = errors.New("athlete already has an active primary program")

// ErrScheduleConflict is returned when a supplemental's schedule overlaps with an existing assignment.
var ErrScheduleConflict = errors.New("schedule conflicts with an existing active program")

// AthleteProgram links an athlete to a program template.
type AthleteProgram struct {
	ID         int64
	AthleteID  int64
	TemplateID int64
	StartDate  string // DATE as YYYY-MM-DD
	Active     bool
	Role       string         // "primary" or "supplemental"
	Schedule   sql.NullString // JSON array of ISO weekday numbers, e.g. "[2,4]"
	Notes      sql.NullString
	Goal       sql.NullString // short-term cycle goal
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Joined fields.
	TemplateName string
	NumWeeks     int
	NumDays      int
	IsLoop       bool
}

// ScheduleDays parses the Schedule JSON into a slice of weekday numbers (1=Mon..7=Sun).
// Returns nil if Schedule is NULL (catch-all primary).
func (ap *AthleteProgram) ScheduleDays() []int {
	if !ap.Schedule.Valid || ap.Schedule.String == "" {
		return nil
	}
	var days []int
	if err := json.Unmarshal([]byte(ap.Schedule.String), &days); err != nil {
		return nil
	}
	return days
}

// ScheduleLabel returns a human-readable label for the schedule (e.g., "Mon, Wed, Fri").
func (ap *AthleteProgram) ScheduleLabel() string {
	days := ap.ScheduleDays()
	if days == nil {
		return "Any day"
	}
	names := map[int]string{1: "Mon", 2: "Tue", 3: "Wed", 4: "Thu", 5: "Fri", 6: "Sat", 7: "Sun"}
	label := ""
	for i, d := range days {
		if i > 0 {
			label += ", "
		}
		if n, ok := names[d]; ok {
			label += n
		}
	}
	return label
}

// AssignProgram assigns a program template to an athlete.
// role must be "primary" or "supplemental". schedule is a JSON weekday array (e.g. "[2,4]") or empty.
// Only one active primary is allowed. Supplemental schedules are validated against existing assignments.
func AssignProgram(db *sql.DB, athleteID, templateID int64, startDate, notes, goal, role, schedule string) (*AthleteProgram, error) {
	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var goalVal sql.NullString
	if goal != "" {
		goalVal = sql.NullString{String: goal, Valid: true}
	}
	if role == "" {
		role = "primary"
	}
	var scheduleVal sql.NullString
	if schedule != "" {
		scheduleVal = sql.NullString{String: schedule, Valid: true}
	}

	// Validate schedule conflicts for supplemental programs.
	if role == "supplemental" && schedule != "" {
		if err := validateScheduleConflict(db, athleteID, schedule, 0); err != nil {
			return nil, err
		}
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO athlete_programs (athlete_id, template_id, start_date, role, schedule, notes, goal) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, templateID, startDate, role, scheduleVal, notesVal, goalVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProgramAlreadyActive
		}
		return nil, fmt.Errorf("models: assign program to athlete %d: %w", athleteID, err)
	}

	return GetAthleteProgramByID(db, id)
}

// validateScheduleConflict checks that the proposed schedule doesn't overlap with any
// existing active assignment. excludeID is an assignment ID to skip (0 to skip none).
func validateScheduleConflict(db *sql.DB, athleteID int64, schedule string, excludeID int64) error {
	var proposedDays []int
	if err := json.Unmarshal([]byte(schedule), &proposedDays); err != nil {
		return fmt.Errorf("models: invalid schedule JSON: %w", err)
	}

	existing, err := ListActiveProgramAssignments(db, athleteID)
	if err != nil {
		return err
	}

	for _, ap := range existing {
		if ap.ID == excludeID {
			continue
		}
		existingDays := ap.ScheduleDays()
		for _, ed := range existingDays {
			for _, pd := range proposedDays {
				if ed == pd {
					return ErrScheduleConflict
				}
			}
		}
	}
	return nil
}

// scanAthleteProgram scans a row into an AthleteProgram (shared by all query functions).
func scanAthleteProgram(scanner interface{ Scan(...any) error }) (*AthleteProgram, error) {
	ap := &AthleteProgram{}
	err := scanner.Scan(&ap.ID, &ap.AthleteID, &ap.TemplateID, &ap.StartDate, &ap.Active,
		&ap.Role, &ap.Schedule, &ap.Notes, &ap.Goal,
		&ap.CreatedAt, &ap.UpdatedAt, &ap.TemplateName, &ap.NumWeeks, &ap.NumDays, &ap.IsLoop)
	return ap, err
}

// athleteProgramColumns is the shared SELECT list for athlete_programs queries.
const athleteProgramColumns = `ap.id, ap.athlete_id, ap.template_id, ap.start_date, ap.active,
		        ap.role, ap.schedule, ap.notes, ap.goal,
		        ap.created_at, ap.updated_at, pt.name, pt.num_weeks, pt.num_days, pt.is_loop`

// GetAthleteProgramByID retrieves an athlete program by primary key.
func GetAthleteProgramByID(db *sql.DB, id int64) (*AthleteProgram, error) {
	row := db.QueryRow(
		`SELECT `+athleteProgramColumns+`
		 FROM athlete_programs ap
		 JOIN program_templates pt ON pt.id = ap.template_id
		 WHERE ap.id = ?`,
		id,
	)
	ap, err := scanAthleteProgram(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("models: athlete program %d not found", id)
		}
		return nil, fmt.Errorf("models: get athlete program %d: %w", id, err)
	}
	return ap, nil
}

// GetActiveProgram returns the current active primary program for an athlete, or nil if none.
func GetActiveProgram(db *sql.DB, athleteID int64) (*AthleteProgram, error) {
	row := db.QueryRow(
		`SELECT `+athleteProgramColumns+`
		 FROM athlete_programs ap
		 JOIN program_templates pt ON pt.id = ap.template_id
		 WHERE ap.athlete_id = ? AND ap.active = 1 AND ap.role = 'primary'`,
		athleteID,
	)
	ap, err := scanAthleteProgram(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No active primary program is not an error.
		}
		return nil, fmt.Errorf("models: get active program for athlete %d: %w", athleteID, err)
	}
	return ap, nil
}

// ListActiveProgramAssignments returns all active program assignments for an athlete
// (primary + supplementals), ordered by role then created_at.
func ListActiveProgramAssignments(db *sql.DB, athleteID int64) ([]*AthleteProgram, error) {
	rows, err := db.Query(
		`SELECT `+athleteProgramColumns+`
		 FROM athlete_programs ap
		 JOIN program_templates pt ON pt.id = ap.template_id
		 WHERE ap.athlete_id = ? AND ap.active = 1
		 ORDER BY CASE ap.role WHEN 'primary' THEN 0 ELSE 1 END, ap.created_at`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list active assignments for %d: %w", athleteID, err)
	}
	defer rows.Close()

	var programs []*AthleteProgram
	for rows.Next() {
		ap, err := scanAthleteProgram(rows)
		if err != nil {
			return nil, fmt.Errorf("models: scan active assignment: %w", err)
		}
		programs = append(programs, ap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate active assignments: %w", err)
	}
	return programs, nil
}

// ListAthletePrograms returns all program assignments for an athlete,
// ordered by start_date descending (most recent first). Includes both
// active and deactivated programs.
func ListAthletePrograms(db *sql.DB, athleteID int64) ([]*AthleteProgram, error) {
	rows, err := db.Query(
		`SELECT `+athleteProgramColumns+`
		 FROM athlete_programs ap
		 JOIN program_templates pt ON pt.id = ap.template_id
		 WHERE ap.athlete_id = ?
		 ORDER BY ap.start_date DESC, ap.created_at DESC`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list athlete programs for %d: %w", athleteID, err)
	}
	defer rows.Close()

	var programs []*AthleteProgram
	for rows.Next() {
		ap, err := scanAthleteProgram(rows)
		if err != nil {
			return nil, fmt.Errorf("models: scan athlete program: %w", err)
		}
		programs = append(programs, ap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate athlete programs: %w", err)
	}
	return programs, nil
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
