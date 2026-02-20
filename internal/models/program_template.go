package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrTemplateInUse is returned when deleting a template that has active athlete assignments.
var ErrTemplateInUse = errors.New("program template is in use by one or more athletes")

// ProgramTemplate represents a reusable training program structure.
// AthleteID nil = global/shared template; non-nil = athlete-specific (e.g. AI-generated).
type ProgramTemplate struct {
	ID          int64
	AthleteID   *int64 // NULL = global, non-NULL = athlete-scoped
	Name        string
	Description sql.NullString
	NumWeeks    int
	NumDays     int  // training days per week
	IsLoop      bool // true = indefinite cycling (e.g. Yessis 1x20)
	Audience    sql.NullString // "youth" or "adult"; NULL = unclassified
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Joined fields populated by detail queries.
	AthleteCount int
}

// CreateProgramTemplate inserts a new program template.
// athleteID nil = global template, non-nil = athlete-scoped.
// audience is "youth", "adult", or "" (NULL).
func CreateProgramTemplate(db *sql.DB, athleteID *int64, name, description string, numWeeks, numDays int, isLoop bool, audience string) (*ProgramTemplate, error) {
	var descVal sql.NullString
	if description != "" {
		descVal = sql.NullString{String: description, Valid: true}
	}

	isLoopInt := 0
	if isLoop {
		isLoopInt = 1
	}

	var audVal sql.NullString
	if audience != "" {
		audVal = sql.NullString{String: audience, Valid: true}
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO program_templates (athlete_id, name, description, num_weeks, num_days, is_loop, audience) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		athleteID, name, descVal, numWeeks, numDays, isLoopInt, audVal,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: program template %q already exists", name)
		}
		return nil, fmt.Errorf("models: create program template: %w", err)
	}

	return GetProgramTemplateByID(db, id)
}

// GetProgramTemplateByID retrieves a program template by primary key.
func GetProgramTemplateByID(db *sql.DB, id int64) (*ProgramTemplate, error) {
	t := &ProgramTemplate{}
	err := db.QueryRow(
		`SELECT pt.id, pt.athlete_id, pt.name, pt.description, pt.num_weeks, pt.num_days, pt.is_loop, pt.audience, pt.created_at, pt.updated_at,
		        COUNT(ap.id) AS athlete_count
		 FROM program_templates pt
		 LEFT JOIN athlete_programs ap ON ap.template_id = pt.id AND ap.active = 1
		 WHERE pt.id = ?
		 GROUP BY pt.id`,
		id,
	).Scan(&t.ID, &t.AthleteID, &t.Name, &t.Description, &t.NumWeeks, &t.NumDays, &t.IsLoop, &t.Audience, &t.CreatedAt, &t.UpdatedAt, &t.AthleteCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("models: program template %d not found", id)
		}
		return nil, fmt.Errorf("models: get program template %d: %w", id, err)
	}
	return t, nil
}

// ListProgramTemplates returns all program templates ordered by name.
func ListProgramTemplates(db *sql.DB) ([]*ProgramTemplate, error) {
	rows, err := db.Query(
		`SELECT pt.id, pt.athlete_id, pt.name, pt.description, pt.num_weeks, pt.num_days, pt.is_loop, pt.audience, pt.created_at, pt.updated_at,
		        COUNT(ap.id) AS athlete_count
		 FROM program_templates pt
		 LEFT JOIN athlete_programs ap ON ap.template_id = pt.id AND ap.active = 1
		 GROUP BY pt.id
		 ORDER BY pt.name COLLATE NOCASE`,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list program templates: %w", err)
	}
	defer rows.Close()

	var templates []*ProgramTemplate
	for rows.Next() {
		t := &ProgramTemplate{}
		if err := rows.Scan(&t.ID, &t.AthleteID, &t.Name, &t.Description, &t.NumWeeks, &t.NumDays, &t.IsLoop, &t.Audience, &t.CreatedAt, &t.UpdatedAt, &t.AthleteCount); err != nil {
			return nil, fmt.Errorf("models: scan program template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate program templates: %w", err)
	}
	return templates, nil
}

// ListProgramTemplatesForAthlete returns global templates plus templates scoped
// to the given athlete, ordered by name. This is the appropriate listing for
// athlete-facing views and program assignment forms.
func ListProgramTemplatesForAthlete(db *sql.DB, athleteID int64) ([]*ProgramTemplate, error) {
	rows, err := db.Query(
		`SELECT pt.id, pt.athlete_id, pt.name, pt.description, pt.num_weeks, pt.num_days, pt.is_loop, pt.audience, pt.created_at, pt.updated_at,
		        COUNT(ap.id) AS athlete_count
		 FROM program_templates pt
		 LEFT JOIN athlete_programs ap ON ap.template_id = pt.id AND ap.active = 1
		 WHERE pt.athlete_id IS NULL OR pt.athlete_id = ?
		 GROUP BY pt.id
		 ORDER BY pt.name COLLATE NOCASE`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list program templates for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var templates []*ProgramTemplate
	for rows.Next() {
		t := &ProgramTemplate{}
		if err := rows.Scan(&t.ID, &t.AthleteID, &t.Name, &t.Description, &t.NumWeeks, &t.NumDays, &t.IsLoop, &t.Audience, &t.CreatedAt, &t.UpdatedAt, &t.AthleteCount); err != nil {
			return nil, fmt.Errorf("models: scan program template for athlete: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate program templates for athlete: %w", err)
	}
	return templates, nil
}

// ListReferenceTemplatesByAudience returns global templates (athlete_id IS NULL)
// filtered by audience. Pass "youth" or "adult" to get matching programs.
// Returns templates ordered by name.
func ListReferenceTemplatesByAudience(db *sql.DB, audience string) ([]*ProgramTemplate, error) {
	rows, err := db.Query(
		`SELECT pt.id, pt.athlete_id, pt.name, pt.description, pt.num_weeks, pt.num_days, pt.is_loop, pt.audience, pt.created_at, pt.updated_at,
		        COUNT(ap.id) AS athlete_count
		 FROM program_templates pt
		 LEFT JOIN athlete_programs ap ON ap.template_id = pt.id AND ap.active = 1
		 WHERE pt.athlete_id IS NULL AND pt.audience = ?
		 GROUP BY pt.id
		 ORDER BY pt.name COLLATE NOCASE`,
		audience,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list reference templates for audience %q: %w", audience, err)
	}
	defer rows.Close()

	var templates []*ProgramTemplate
	for rows.Next() {
		t := &ProgramTemplate{}
		if err := rows.Scan(&t.ID, &t.AthleteID, &t.Name, &t.Description, &t.NumWeeks, &t.NumDays, &t.IsLoop, &t.Audience, &t.CreatedAt, &t.UpdatedAt, &t.AthleteCount); err != nil {
			return nil, fmt.Errorf("models: scan reference template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate reference templates: %w", err)
	}
	return templates, nil
}

// UpdateProgramTemplate updates a program template's metadata.
func UpdateProgramTemplate(db *sql.DB, id int64, name, description string, numWeeks, numDays int, isLoop bool) (*ProgramTemplate, error) {
	var descVal sql.NullString
	if description != "" {
		descVal = sql.NullString{String: description, Valid: true}
	}

	isLoopInt := 0
	if isLoop {
		isLoopInt = 1
	}

	_, err := db.Exec(
		`UPDATE program_templates SET name = ?, description = ?, num_weeks = ?, num_days = ?, is_loop = ? WHERE id = ?`,
		name, descVal, numWeeks, numDays, isLoopInt, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("models: program template %q already exists", name)
		}
		return nil, fmt.Errorf("models: update program template %d: %w", id, err)
	}

	return GetProgramTemplateByID(db, id)
}

// DeleteProgramTemplate removes a program template. Fails if athletes are assigned to it.
func DeleteProgramTemplate(db *sql.DB, id int64) error {
	// Check for active athlete assignments.
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM athlete_programs WHERE template_id = ? AND active = 1`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("models: check program template usage: %w", err)
	}
	if count > 0 {
		return ErrTemplateInUse
	}

	result, err := db.Exec(`DELETE FROM program_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete program template %d: %w", id, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("models: program template %d not found", id)
	}
	return nil
}
