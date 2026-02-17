package models

import (
	"database/sql"
	"fmt"
)

// ProgressionRule defines a per-exercise TM increment for a program template.
// After a cycle completes, the app suggests bumping the training max by this amount.
type ProgressionRule struct {
	ID           int64
	TemplateID   int64
	ExerciseID   int64
	Increment    float64
	ExerciseName string // joined field
}

// IncrementLabel returns a formatted increment string (e.g. "5", "10", "2.5").
func (pr *ProgressionRule) IncrementLabel() string {
	if pr.Increment == float64(int(pr.Increment)) {
		return fmt.Sprintf("%.0f", pr.Increment)
	}
	return fmt.Sprintf("%.1f", pr.Increment)
}

// SetProgressionRule creates or updates a progression rule for a template+exercise.
// Uses INSERT OR REPLACE to handle upserts cleanly.
func SetProgressionRule(db *sql.DB, templateID, exerciseID int64, increment float64) (*ProgressionRule, error) {
	_, err := db.Exec(
		`INSERT INTO progression_rules (template_id, exercise_id, increment)
		 VALUES (?, ?, ?)
		 ON CONFLICT(template_id, exercise_id) DO UPDATE SET increment = excluded.increment`,
		templateID, exerciseID, increment,
	)
	if err != nil {
		return nil, fmt.Errorf("models: set progression rule for template %d exercise %d: %w", templateID, exerciseID, err)
	}

	return GetProgressionRule(db, templateID, exerciseID)
}

// GetProgressionRule retrieves a single progression rule for a template+exercise.
func GetProgressionRule(db *sql.DB, templateID, exerciseID int64) (*ProgressionRule, error) {
	pr := &ProgressionRule{}
	err := db.QueryRow(
		`SELECT pr.id, pr.template_id, pr.exercise_id, pr.increment, e.name
		 FROM progression_rules pr
		 JOIN exercises e ON e.id = pr.exercise_id
		 WHERE pr.template_id = ? AND pr.exercise_id = ?`,
		templateID, exerciseID,
	).Scan(&pr.ID, &pr.TemplateID, &pr.ExerciseID, &pr.Increment, &pr.ExerciseName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("models: get progression rule: %w", err)
	}
	return pr, nil
}

// ListProgressionRules returns all progression rules for a program template.
func ListProgressionRules(db *sql.DB, templateID int64) ([]*ProgressionRule, error) {
	rows, err := db.Query(
		`SELECT pr.id, pr.template_id, pr.exercise_id, pr.increment, e.name
		 FROM progression_rules pr
		 JOIN exercises e ON e.id = pr.exercise_id
		 WHERE pr.template_id = ?
		 ORDER BY e.name COLLATE NOCASE`,
		templateID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list progression rules for template %d: %w", templateID, err)
	}
	defer rows.Close()

	var rules []*ProgressionRule
	for rows.Next() {
		pr := &ProgressionRule{}
		if err := rows.Scan(&pr.ID, &pr.TemplateID, &pr.ExerciseID, &pr.Increment, &pr.ExerciseName); err != nil {
			return nil, fmt.Errorf("models: scan progression rule: %w", err)
		}
		rules = append(rules, pr)
	}
	return rules, rows.Err()
}

// DeleteProgressionRule removes a progression rule by ID.
func DeleteProgressionRule(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM progression_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete progression rule %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
