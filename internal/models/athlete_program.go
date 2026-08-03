package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrProgramAlreadyActive is returned when assigning a primary program to an athlete who already has one.
	ErrProgramAlreadyActive = errors.New("athlete already has an active primary program")
	// ErrScheduleConflict is returned when a supplemental's schedule overlaps with an existing assignment.
	ErrScheduleConflict = errors.New("schedule conflicts with an existing active program")
	// ErrInvalidSchedule is returned for malformed or unusable weekday schedules.
	ErrInvalidSchedule = errors.New("invalid program schedule")
	// ErrInvalidProgramRole is returned for values outside the schema's role enum.
	ErrInvalidProgramRole = errors.New("invalid program role")
)

// AthleteProgram links an athlete to a program template.
type AthleteProgram struct {
	ID            int64
	AthleteID     int64
	TemplateID    int64
	StartDate     string // DATE as YYYY-MM-DD
	Active        bool
	DeactivatedAt sql.NullTime
	Role          string         // "primary" or "supplemental"
	Schedule      sql.NullString // JSON array of ISO weekday numbers, e.g. "[2,4]"
	Notes         sql.NullString
	Goal          sql.NullString // short-term cycle goal
	CreatedAt     time.Time
	UpdatedAt     time.Time

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

// parseScheduleDays validates a JSON array of distinct ISO weekdays.
func parseScheduleDays(schedule string) ([]int, error) {
	var days []int
	if err := json.Unmarshal([]byte(schedule), &days); err != nil {
		return nil, fmt.Errorf("%w: must be a JSON array of ISO weekdays: %v", ErrInvalidSchedule, err)
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("%w: select at least one weekday or omit schedule", ErrInvalidSchedule)
	}

	seen := make(map[int]struct{}, len(days))
	for _, day := range days {
		if day < 1 || day > 7 {
			return nil, fmt.Errorf("%w: weekdays must be between 1 (Monday) and 7 (Sunday)", ErrInvalidSchedule)
		}
		if _, ok := seen[day]; ok {
			return nil, fmt.Errorf("%w: weekdays must not repeat", ErrInvalidSchedule)
		}
		seen[day] = struct{}{}
	}

	return days, nil
}

// validateProgramAssignment validates the role and optional weekday schedule.
// An omitted schedule leaves a primary program flexible, preserving existing
// assignment semantics.
func validateProgramAssignment(role, schedule string) ([]int, error) {
	if role != "primary" && role != "supplemental" {
		return nil, fmt.Errorf("%w: must be primary or supplemental", ErrInvalidProgramRole)
	}
	if schedule == "" {
		return nil, nil
	}
	return parseScheduleDays(schedule)
}

// AssignProgram assigns a program template to an athlete.
// role must be "primary" or "supplemental". schedule is a JSON weekday array (e.g. "[2,4]") or empty.
// Only one active primary is allowed. Supplemental schedules are validated against existing assignments.
func AssignProgram(ctx context.Context, db *sql.DB, athleteID, templateID int64, startDate, notes, goal, role, schedule string) (*AthleteProgram, error) {
	if role == "" {
		role = "primary"
	}
	if _, err := validateProgramAssignment(role, schedule); err != nil {
		return nil, err
	}

	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var goalVal sql.NullString
	if goal != "" {
		goalVal = sql.NullString{String: goal, Valid: true}
	}
	var scheduleVal sql.NullString
	if schedule != "" {
		scheduleVal = sql.NullString{String: schedule, Valid: true}
	}

	// Validate schedule conflicts for supplemental programs.
	if role == "supplemental" && schedule != "" {
		if err := validateScheduleConflict(ctx, db, athleteID, schedule, 0); err != nil {
			return nil, err
		}
	}

	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO athlete_programs (athlete_id, template_id, start_date, role, schedule, notes, goal) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, templateID, startDate, role, scheduleVal, notesVal, goalVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProgramAlreadyActive
		}
		return nil, fmt.Errorf("models: assign program to athlete %d: %w", athleteID, err)
	}

	return GetAthleteProgramByID(ctx, db, id)
}

// validateScheduleConflict checks that the proposed schedule doesn't overlap with any
// existing active assignment. excludeID is an assignment ID to skip (0 to skip none).
func validateScheduleConflict(ctx context.Context, db *sql.DB, athleteID int64, schedule string, excludeID int64) error {
	proposedDays, err := parseScheduleDays(schedule)
	if err != nil {
		return err
	}

	existing, err := ListActiveProgramAssignments(ctx, db, athleteID)
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

// ReplaceProgram atomically deactivates active assignments in the requested
// role and creates the replacement. Validation runs before the transaction so
// a malformed request can never strand an athlete without their current plan.
func ReplaceProgram(ctx context.Context, db *sql.DB, athleteID, templateID int64, startDate, notes, goal, role, schedule string) (*AthleteProgram, error) {
	if role == "" {
		role = "primary"
	}
	proposedDays, err := validateProgramAssignment(role, schedule)
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: replace program begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE athlete_programs
		 SET active = 0, deactivated_at = CURRENT_TIMESTAMP
		 WHERE athlete_id = ? AND active = 1 AND role = ?`,
		athleteID, role,
	); err != nil {
		return nil, fmt.Errorf("models: deactivate active %s program for athlete %d: %w", role, athleteID, err)
	}

	if role == "supplemental" && len(proposedDays) > 0 {
		if err := validateScheduleConflictInTx(ctx, tx, athleteID, proposedDays); err != nil {
			return nil, err
		}
	}

	var notesVal sql.NullString
	if notes != "" {
		notesVal = sql.NullString{String: notes, Valid: true}
	}
	var goalVal sql.NullString
	if goal != "" {
		goalVal = sql.NullString{String: goal, Valid: true}
	}
	var scheduleVal sql.NullString
	if schedule != "" {
		scheduleVal = sql.NullString{String: schedule, Valid: true}
	}

	var id int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO athlete_programs (athlete_id, template_id, start_date, role, schedule, notes, goal)
		 VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, templateID, startDate, role, scheduleVal, notesVal, goalVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProgramAlreadyActive
		}
		return nil, fmt.Errorf("models: replace program for athlete %d: %w", athleteID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: replace program commit: %w", err)
	}
	return GetAthleteProgramByID(ctx, db, id)
}

func validateScheduleConflictInTx(ctx context.Context, tx *sql.Tx, athleteID int64, proposedDays []int) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT schedule
		 FROM athlete_programs
		 WHERE athlete_id = ? AND active = 1 AND schedule IS NOT NULL`,
		athleteID,
	)
	if err != nil {
		return fmt.Errorf("models: list active schedules for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var schedule sql.NullString
		if err := rows.Scan(&schedule); err != nil {
			return fmt.Errorf("models: scan active schedule: %w", err)
		}
		existingDays, err := parseScheduleDays(schedule.String)
		if err != nil {
			// Preserve the existing resolver's behavior for legacy malformed
			// schedules: they do not claim a weekday.
			continue
		}
		for _, existingDay := range existingDays {
			for _, proposedDay := range proposedDays {
				if existingDay == proposedDay {
					return ErrScheduleConflict
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("models: iterate active schedules: %w", err)
	}
	return nil
}

// scanAthleteProgram scans a row into an AthleteProgram (shared by all query functions).
func scanAthleteProgram(scanner interface{ Scan(...any) error }) (*AthleteProgram, error) {
	ap := &AthleteProgram{}
	err := scanner.Scan(&ap.ID, &ap.AthleteID, &ap.TemplateID, &ap.StartDate, &ap.Active,
		&ap.DeactivatedAt, &ap.Role, &ap.Schedule, &ap.Notes, &ap.Goal,
		&ap.CreatedAt, &ap.UpdatedAt, &ap.TemplateName, &ap.NumWeeks, &ap.NumDays, &ap.IsLoop)
	return ap, err
}

// athleteProgramColumns is the shared SELECT list for athlete_programs queries.
const athleteProgramColumns = `ap.id, ap.athlete_id, ap.template_id, ap.start_date, ap.active,
		        ap.deactivated_at, ap.role, ap.schedule, ap.notes, ap.goal,
		        ap.created_at, ap.updated_at, pt.name, pt.num_weeks, pt.num_days, pt.is_loop`

// GetAthleteProgramByID retrieves an athlete program by primary key.
func GetAthleteProgramByID(ctx context.Context, db *sql.DB, id int64) (*AthleteProgram, error) {
	row := db.QueryRowContext(ctx,
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
func GetActiveProgram(ctx context.Context, db *sql.DB, athleteID int64) (*AthleteProgram, error) {
	row := db.QueryRowContext(ctx,
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
func ListActiveProgramAssignments(ctx context.Context, db *sql.DB, athleteID int64) ([]*AthleteProgram, error) {
	rows, err := db.QueryContext(ctx,
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
func ListAthletePrograms(ctx context.Context, db *sql.DB, athleteID int64) ([]*AthleteProgram, error) {
	rows, err := db.QueryContext(ctx,
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
func DeactivateProgram(ctx context.Context, db *sql.DB, athleteProgramID int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE athlete_programs SET active = 0, deactivated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		athleteProgramID,
	)
	if err != nil {
		return fmt.Errorf("models: deactivate athlete program %d: %w", athleteProgramID, err)
	}
	return nil
}

// ReactivateProgram reactivates a deactivated athlete program assignment.
func ReactivateProgram(ctx context.Context, db *sql.DB, athleteProgramID int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE athlete_programs SET active = 1, deactivated_at = NULL WHERE id = ?`,
		athleteProgramID,
	)
	if err != nil {
		return fmt.Errorf("models: reactivate athlete program %d: %w", athleteProgramID, err)
	}
	return nil
}

// DeleteAthleteProgram removes an athlete program assignment entirely.
func DeleteAthleteProgram(ctx context.Context, db *sql.DB, athleteProgramID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM athlete_programs WHERE id = ?`, athleteProgramID)
	if err != nil {
		return fmt.Errorf("models: delete athlete program %d: %w", athleteProgramID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
