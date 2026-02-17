package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrDuplicateEquipmentName is returned when an equipment name is already taken.
var ErrDuplicateEquipmentName = errors.New("duplicate equipment name")

// ErrEquipmentInUse is returned when deleting equipment that is still linked
// to exercises or athletes.
var ErrEquipmentInUse = errors.New("equipment is still in use")

// Equipment represents a piece of training equipment (e.g., "Barbell", "Squat Rack").
type Equipment struct {
	ID          int64
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ExerciseEquipment represents a required or optional equipment link for an exercise.
type ExerciseEquipment struct {
	ID            int64
	ExerciseID    int64
	EquipmentID   int64
	EquipmentName string // populated by joins
	Optional      bool
}

// AthleteEquipment represents an equipment item available to an athlete.
type AthleteEquipment struct {
	ID            int64
	AthleteID     int64
	EquipmentID   int64
	EquipmentName string // populated by joins
}

// EquipmentCompatibility summarizes whether an athlete has the equipment for an exercise.
type EquipmentCompatibility struct {
	ExerciseID   int64
	ExerciseName string
	HasRequired  bool                // true if athlete has all required equipment
	Missing      []ExerciseEquipment // required equipment the athlete lacks
	Available    []ExerciseEquipment // required equipment the athlete has
	Optional     []ExerciseEquipment // optional equipment (for display)
}

// ProgramCompatibility summarizes equipment readiness for an entire program template.
type ProgramCompatibility struct {
	TemplateID   int64
	TemplateName string
	Ready        bool                     // true if athlete has equipment for all exercises
	Exercises    []EquipmentCompatibility  // per-exercise breakdown
	ReadyCount   int                      // exercises with all required equipment
	TotalCount   int                      // total unique exercises in program
}

// CreateEquipment inserts a new equipment item.
func CreateEquipment(db *sql.DB, name, description string) (*Equipment, error) {
	var descVal sql.NullString
	if description != "" {
		descVal = sql.NullString{String: description, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO equipment (name, description) VALUES (?, ?)`,
		name, descVal,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateEquipmentName
		}
		return nil, fmt.Errorf("models: create equipment %q: %w", name, err)
	}

	id, _ := result.LastInsertId()
	return GetEquipmentByID(db, id)
}

// GetEquipmentByID retrieves an equipment item by primary key.
func GetEquipmentByID(db *sql.DB, id int64) (*Equipment, error) {
	e := &Equipment{}
	err := db.QueryRow(
		`SELECT id, name, description, created_at, updated_at
		 FROM equipment WHERE id = ?`, id,
	).Scan(&e.ID, &e.Name, &e.Description, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get equipment %d: %w", id, err)
	}
	return e, nil
}

// UpdateEquipment modifies an existing equipment item.
func UpdateEquipment(db *sql.DB, id int64, name, description string) (*Equipment, error) {
	var descVal sql.NullString
	if description != "" {
		descVal = sql.NullString{String: description, Valid: true}
	}

	result, err := db.Exec(
		`UPDATE equipment SET name = ?, description = ? WHERE id = ?`,
		name, descVal, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateEquipmentName
		}
		return nil, fmt.Errorf("models: update equipment %d: %w", id, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, ErrNotFound
	}

	return GetEquipmentByID(db, id)
}

// DeleteEquipment removes an equipment item by ID.
func DeleteEquipment(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM equipment WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete equipment %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListEquipment returns all equipment items ordered by name.
func ListEquipment(db *sql.DB) ([]*Equipment, error) {
	rows, err := db.Query(
		`SELECT id, name, description, created_at, updated_at
		 FROM equipment ORDER BY name COLLATE NOCASE LIMIT 200`,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list equipment: %w", err)
	}
	defer rows.Close()

	var items []*Equipment
	for rows.Next() {
		e := &Equipment{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Description, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("models: scan equipment: %w", err)
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

// --- Exercise Equipment (requirements) ---

// AddExerciseEquipment links an equipment item to an exercise.
func AddExerciseEquipment(db *sql.DB, exerciseID, equipmentID int64, optional bool) error {
	optVal := 0
	if optional {
		optVal = 1
	}
	_, err := db.Exec(
		`INSERT INTO exercise_equipment (exercise_id, equipment_id, optional)
		 VALUES (?, ?, ?)
		 ON CONFLICT(exercise_id, equipment_id) DO UPDATE SET optional = excluded.optional`,
		exerciseID, equipmentID, optVal,
	)
	if err != nil {
		return fmt.Errorf("models: add exercise equipment (exercise=%d, equipment=%d): %w", exerciseID, equipmentID, err)
	}
	return nil
}

// RemoveExerciseEquipment unlinks an equipment item from an exercise.
func RemoveExerciseEquipment(db *sql.DB, exerciseID, equipmentID int64) error {
	result, err := db.Exec(
		`DELETE FROM exercise_equipment WHERE exercise_id = ? AND equipment_id = ?`,
		exerciseID, equipmentID,
	)
	if err != nil {
		return fmt.Errorf("models: remove exercise equipment (exercise=%d, equipment=%d): %w", exerciseID, equipmentID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListExerciseEquipment returns all equipment linked to an exercise.
func ListExerciseEquipment(db *sql.DB, exerciseID int64) ([]ExerciseEquipment, error) {
	rows, err := db.Query(
		`SELECT ee.id, ee.exercise_id, ee.equipment_id, e.name, ee.optional
		 FROM exercise_equipment ee
		 JOIN equipment e ON e.id = ee.equipment_id
		 WHERE ee.exercise_id = ?
		 ORDER BY ee.optional, e.name COLLATE NOCASE`,
		exerciseID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list exercise equipment (exercise=%d): %w", exerciseID, err)
	}
	defer rows.Close()

	var items []ExerciseEquipment
	for rows.Next() {
		var ee ExerciseEquipment
		if err := rows.Scan(&ee.ID, &ee.ExerciseID, &ee.EquipmentID, &ee.EquipmentName, &ee.Optional); err != nil {
			return nil, fmt.Errorf("models: scan exercise equipment: %w", err)
		}
		items = append(items, ee)
	}
	return items, rows.Err()
}

// SyncExerciseEquipment replaces all equipment links for an exercise.
// required and optional are slices of equipment IDs.
func SyncExerciseEquipment(db *sql.DB, exerciseID int64, required, optional []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("models: sync exercise equipment begin: %w", err)
	}
	defer tx.Rollback()

	// Remove all existing links.
	if _, err := tx.Exec(`DELETE FROM exercise_equipment WHERE exercise_id = ?`, exerciseID); err != nil {
		return fmt.Errorf("models: sync exercise equipment delete: %w", err)
	}

	stmt, err := tx.Prepare(
		`INSERT INTO exercise_equipment (exercise_id, equipment_id, optional) VALUES (?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("models: sync exercise equipment prepare: %w", err)
	}
	defer stmt.Close()

	for _, eqID := range required {
		if _, err := stmt.Exec(exerciseID, eqID, 0); err != nil {
			return fmt.Errorf("models: sync exercise equipment insert required (equipment=%d): %w", eqID, err)
		}
	}
	for _, eqID := range optional {
		if _, err := stmt.Exec(exerciseID, eqID, 1); err != nil {
			return fmt.Errorf("models: sync exercise equipment insert optional (equipment=%d): %w", eqID, err)
		}
	}

	return tx.Commit()
}

// --- Athlete Equipment (inventory) ---

// AddAthleteEquipment adds an equipment item to an athlete's inventory.
func AddAthleteEquipment(db *sql.DB, athleteID, equipmentID int64) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO athlete_equipment (athlete_id, equipment_id) VALUES (?, ?)`,
		athleteID, equipmentID,
	)
	if err != nil {
		return fmt.Errorf("models: add athlete equipment (athlete=%d, equipment=%d): %w", athleteID, equipmentID, err)
	}
	return nil
}

// RemoveAthleteEquipment removes an equipment item from an athlete's inventory.
func RemoveAthleteEquipment(db *sql.DB, athleteID, equipmentID int64) error {
	result, err := db.Exec(
		`DELETE FROM athlete_equipment WHERE athlete_id = ? AND equipment_id = ?`,
		athleteID, equipmentID,
	)
	if err != nil {
		return fmt.Errorf("models: remove athlete equipment (athlete=%d, equipment=%d): %w", athleteID, equipmentID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAthleteEquipment returns all equipment available to an athlete.
func ListAthleteEquipment(db *sql.DB, athleteID int64) ([]AthleteEquipment, error) {
	rows, err := db.Query(
		`SELECT ae.id, ae.athlete_id, ae.equipment_id, e.name
		 FROM athlete_equipment ae
		 JOIN equipment e ON e.id = ae.equipment_id
		 WHERE ae.athlete_id = ?
		 ORDER BY e.name COLLATE NOCASE`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list athlete equipment (athlete=%d): %w", athleteID, err)
	}
	defer rows.Close()

	var items []AthleteEquipment
	for rows.Next() {
		var ae AthleteEquipment
		if err := rows.Scan(&ae.ID, &ae.AthleteID, &ae.EquipmentID, &ae.EquipmentName); err != nil {
			return nil, fmt.Errorf("models: scan athlete equipment: %w", err)
		}
		items = append(items, ae)
	}
	return items, rows.Err()
}

// AthleteEquipmentIDs returns a set of equipment IDs available to an athlete.
func AthleteEquipmentIDs(db *sql.DB, athleteID int64) (map[int64]bool, error) {
	rows, err := db.Query(
		`SELECT equipment_id FROM athlete_equipment WHERE athlete_id = ?`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: athlete equipment ids (athlete=%d): %w", athleteID, err)
	}
	defer rows.Close()

	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("models: scan equipment id: %w", err)
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// --- Compatibility Checks ---

// CheckExerciseCompatibility checks whether an athlete has the required equipment
// for a specific exercise.
func CheckExerciseCompatibility(db *sql.DB, athleteID, exerciseID int64) (*EquipmentCompatibility, error) {
	// Get exercise info.
	exercise, err := GetExerciseByID(db, exerciseID)
	if err != nil {
		return nil, err
	}

	// Get all equipment requirements for this exercise.
	eqList, err := ListExerciseEquipment(db, exerciseID)
	if err != nil {
		return nil, err
	}

	// Get athlete's available equipment.
	athleteIDs, err := AthleteEquipmentIDs(db, athleteID)
	if err != nil {
		return nil, err
	}

	result := &EquipmentCompatibility{
		ExerciseID:   exerciseID,
		ExerciseName: exercise.Name,
		HasRequired:  true,
	}

	for _, eq := range eqList {
		if eq.Optional {
			result.Optional = append(result.Optional, eq)
			continue
		}
		if athleteIDs[eq.EquipmentID] {
			result.Available = append(result.Available, eq)
		} else {
			result.Missing = append(result.Missing, eq)
			result.HasRequired = false
		}
	}

	return result, nil
}

// CheckAthleteExerciseCompatibility checks equipment compatibility for all
// actively assigned exercises for an athlete. Returns a slice of compatibility
// results, one per assigned exercise.
func CheckAthleteExerciseCompatibility(db *sql.DB, athleteID int64) ([]EquipmentCompatibility, error) {
	// Get active assignments.
	rows, err := db.Query(
		`SELECT DISTINCT ae.exercise_id, e.name
		 FROM athlete_exercises ae
		 JOIN exercises e ON e.id = ae.exercise_id
		 WHERE ae.athlete_id = ? AND ae.active = 1
		 ORDER BY e.name COLLATE NOCASE`,
		athleteID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list assigned exercises for compatibility (athlete=%d): %w", athleteID, err)
	}
	defer rows.Close()

	type exerciseRef struct {
		id   int64
		name string
	}
	var exercises []exerciseRef
	for rows.Next() {
		var ref exerciseRef
		if err := rows.Scan(&ref.id, &ref.name); err != nil {
			return nil, fmt.Errorf("models: scan exercise ref: %w", err)
		}
		exercises = append(exercises, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Get athlete's equipment set once.
	athleteIDs, err := AthleteEquipmentIDs(db, athleteID)
	if err != nil {
		return nil, err
	}

	var results []EquipmentCompatibility
	for _, ex := range exercises {
		eqList, err := ListExerciseEquipment(db, ex.id)
		if err != nil {
			return nil, err
		}

		compat := EquipmentCompatibility{
			ExerciseID:   ex.id,
			ExerciseName: ex.name,
			HasRequired:  true,
		}

		for _, eq := range eqList {
			if eq.Optional {
				compat.Optional = append(compat.Optional, eq)
				continue
			}
			if athleteIDs[eq.EquipmentID] {
				compat.Available = append(compat.Available, eq)
			} else {
				compat.Missing = append(compat.Missing, eq)
				compat.HasRequired = false
			}
		}

		results = append(results, compat)
	}

	return results, nil
}

// CheckProgramCompatibility checks whether an athlete has the required equipment
// for every exercise in a program template. It examines all unique exercises
// referenced by the template's prescribed sets.
func CheckProgramCompatibility(db *sql.DB, athleteID, templateID int64) (*ProgramCompatibility, error) {
	// Get program template info.
	tmpl, err := GetProgramTemplateByID(db, templateID)
	if err != nil {
		return nil, err
	}

	// Get all unique exercises referenced by this template's prescribed sets.
	rows, err := db.Query(
		`SELECT DISTINCT ps.exercise_id, e.name
		 FROM prescribed_sets ps
		 JOIN exercises e ON e.id = ps.exercise_id
		 WHERE ps.template_id = ?
		 ORDER BY e.name COLLATE NOCASE`,
		templateID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list program exercises for compatibility (template=%d): %w", templateID, err)
	}
	defer rows.Close()

	type exerciseRef struct {
		id   int64
		name string
	}
	var exercises []exerciseRef
	for rows.Next() {
		var ref exerciseRef
		if err := rows.Scan(&ref.id, &ref.name); err != nil {
			return nil, fmt.Errorf("models: scan exercise ref: %w", err)
		}
		exercises = append(exercises, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Get athlete's equipment set once.
	athleteIDs, err := AthleteEquipmentIDs(db, athleteID)
	if err != nil {
		return nil, err
	}

	result := &ProgramCompatibility{
		TemplateID:   templateID,
		TemplateName: tmpl.Name,
		Ready:        true,
		TotalCount:   len(exercises),
	}

	for _, ex := range exercises {
		eqList, err := ListExerciseEquipment(db, ex.id)
		if err != nil {
			return nil, err
		}

		compat := EquipmentCompatibility{
			ExerciseID:   ex.id,
			ExerciseName: ex.name,
			HasRequired:  true,
		}

		for _, eq := range eqList {
			if eq.Optional {
				compat.Optional = append(compat.Optional, eq)
				continue
			}
			if athleteIDs[eq.EquipmentID] {
				compat.Available = append(compat.Available, eq)
			} else {
				compat.Missing = append(compat.Missing, eq)
				compat.HasRequired = false
			}
		}

		if compat.HasRequired {
			result.ReadyCount++
		} else {
			result.Ready = false
		}

		result.Exercises = append(result.Exercises, compat)
	}

	return result, nil
}
