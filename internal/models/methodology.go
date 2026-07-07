package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrDuplicateMethodologyKey is returned when a methodology key collides
// with an existing row on insert.
var ErrDuplicateMethodologyKey = errors.New("duplicate methodology key")

// Methodology audience values. Mirrors the CHECK constraint on
// methodologies.audience in migration 0004 — keep in sync.
const (
	MethodologyAudienceYouth = "youth"
	MethodologyAudienceAdult = "adult"
)

// Methodology is a stored, coach-selectable program-design philosophy +
// prescription block (ADR 016). The Definition field carries ONLY the
// methodology-specific per-tier prompt block — the shared youth-rules
// preamble and the safety-floor invariants STAY IN CODE (see ADR 016
// Decision #4 and internal/llm/generate.go:148-174).
//
// Phase 1 is data-only: nothing in buildSystemPrompt reads this struct
// yet. Phase 2 wires generation to consume the selected methodology's
// Definition + allow-lists.
type Methodology struct {
	ID              int64
	Key             string
	Name            string
	Audience        sql.NullString
	ApplicableTiers sql.NullString
	Philosophy      sql.NullString
	Definition      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MethodologyWithLinks is a Methodology eagerly loaded with all of its
// link tables. The slices are non-nil but may be empty.
type MethodologyWithLinks struct {
	Methodology

	// ReferenceProgramIDs are the program_templates rows the LLM should
	// treat as primary structural exemplars when generating against this
	// methodology. Sorted by template_id for stable test snapshots.
	ReferenceProgramIDs []int64

	// AllowedEquipmentIDs is the equipment allow-list. The Phase-2 catalog
	// filter intersects this with the athlete's available equipment.
	AllowedEquipmentIDs []int64

	// AllowedPatterns is the Dan John movement-pattern allow-list
	// (broad scoping rule).
	AllowedPatterns []string

	// AllowedExerciseIDs is an explicit-list override on top of the pattern
	// allow-list (e.g. 5/3/1 barbell mains, the Sarge bespoke list).
	AllowedExerciseIDs []int64
}

// CreateMethodology inserts a new methodology. Returns ErrDuplicateMethodologyKey
// when the key is already taken.
func CreateMethodology(ctx context.Context, db *sql.DB, m *Methodology) (*Methodology, error) {
	if m == nil {
		return nil, fmt.Errorf("models: create methodology with nil input")
	}
	if m.Key == "" {
		return nil, fmt.Errorf("models: methodology key is required")
	}
	if m.Name == "" {
		return nil, fmt.Errorf("models: methodology name is required")
	}
	if m.Definition == "" {
		return nil, fmt.Errorf("models: methodology definition is required")
	}
	if m.Audience.Valid && m.Audience.String != MethodologyAudienceYouth && m.Audience.String != MethodologyAudienceAdult {
		return nil, fmt.Errorf("models: methodology audience %q invalid (allowed: youth, adult, or null)", m.Audience.String)
	}

	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO methodologies (key, name, audience, applicable_tiers, philosophy, definition)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		m.Key, m.Name, m.Audience, m.ApplicableTiers, m.Philosophy, m.Definition,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateMethodologyKey
		}
		return nil, fmt.Errorf("models: create methodology %q: %w", m.Key, err)
	}
	return GetMethodologyByID(ctx, db, id)
}

// GetMethodologyByID retrieves a methodology by primary key.
func GetMethodologyByID(ctx context.Context, db *sql.DB, id int64) (*Methodology, error) {
	m := &Methodology{}
	err := db.QueryRowContext(ctx,
		`SELECT id, key, name, audience, applicable_tiers, philosophy, definition, created_at, updated_at
		 FROM methodologies WHERE id = ?`, id,
	).Scan(&m.ID, &m.Key, &m.Name, &m.Audience, &m.ApplicableTiers, &m.Philosophy, &m.Definition, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get methodology %d: %w", id, err)
	}
	return m, nil
}

// GetMethodologyByKey retrieves a methodology by its stable key (the form
// callers use to look up the Yessis-1×20 row, for example).
func GetMethodologyByKey(ctx context.Context, db *sql.DB, key string) (*Methodology, error) {
	m := &Methodology{}
	err := db.QueryRowContext(ctx,
		`SELECT id, key, name, audience, applicable_tiers, philosophy, definition, created_at, updated_at
		 FROM methodologies WHERE key = ?`, key,
	).Scan(&m.ID, &m.Key, &m.Name, &m.Audience, &m.ApplicableTiers, &m.Philosophy, &m.Definition, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get methodology key %q: %w", key, err)
	}
	return m, nil
}

// ListMethodologies returns all methodologies, optionally filtered by audience.
// Pass empty string for audience to list all. Ordering is by name COLLATE NOCASE
// for stable UI display.
func ListMethodologies(ctx context.Context, db *sql.DB, audienceFilter string) ([]*Methodology, error) {
	var query string
	var args []any
	if audienceFilter == "" {
		query = `SELECT id, key, name, audience, applicable_tiers, philosophy, definition, created_at, updated_at
		         FROM methodologies ORDER BY name COLLATE NOCASE`
	} else {
		if audienceFilter != MethodologyAudienceYouth && audienceFilter != MethodologyAudienceAdult {
			return nil, fmt.Errorf("models: list methodologies: invalid audience filter %q", audienceFilter)
		}
		query = `SELECT id, key, name, audience, applicable_tiers, philosophy, definition, created_at, updated_at
		         FROM methodologies WHERE audience = ? ORDER BY name COLLATE NOCASE`
		args = append(args, audienceFilter)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("models: list methodologies: %w", err)
	}
	defer rows.Close()

	var out []*Methodology
	for rows.Next() {
		m := &Methodology{}
		if err := rows.Scan(&m.ID, &m.Key, &m.Name, &m.Audience, &m.ApplicableTiers, &m.Philosophy, &m.Definition, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("models: scan methodology: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate methodologies: %w", err)
	}
	return out, nil
}

// LoadMethodologyWithLinks returns a methodology along with all its
// link-table rows in one call. Used by Phase-2 generation wiring.
func LoadMethodologyWithLinks(ctx context.Context, db *sql.DB, id int64) (*MethodologyWithLinks, error) {
	base, err := GetMethodologyByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	out := &MethodologyWithLinks{Methodology: *base}

	refs, err := scanInt64Column(ctx, db,
		`SELECT template_id FROM methodology_reference_programs WHERE methodology_id = ? ORDER BY template_id`, id)
	if err != nil {
		return nil, fmt.Errorf("models: load methodology reference programs %d: %w", id, err)
	}
	out.ReferenceProgramIDs = refs

	equip, err := scanInt64Column(ctx, db,
		`SELECT equipment_id FROM methodology_allowed_equipment WHERE methodology_id = ? ORDER BY equipment_id`, id)
	if err != nil {
		return nil, fmt.Errorf("models: load methodology allowed equipment %d: %w", id, err)
	}
	out.AllowedEquipmentIDs = equip

	patterns, err := scanStringColumn(ctx, db,
		`SELECT pattern FROM methodology_allowed_patterns WHERE methodology_id = ? ORDER BY pattern`, id)
	if err != nil {
		return nil, fmt.Errorf("models: load methodology allowed patterns %d: %w", id, err)
	}
	out.AllowedPatterns = patterns

	exIDs, err := scanInt64Column(ctx, db,
		`SELECT exercise_id FROM methodology_allowed_exercises WHERE methodology_id = ? ORDER BY exercise_id`, id)
	if err != nil {
		return nil, fmt.Errorf("models: load methodology allowed exercises %d: %w", id, err)
	}
	out.AllowedExerciseIDs = exIDs

	return out, nil
}

// AddMethodologyReferencePrograms links the given program_templates to the
// methodology. Idempotent (INSERT OR IGNORE).
func AddMethodologyReferencePrograms(ctx context.Context, db *sql.DB, methodologyID int64, templateIDs []int64) error {
	return addMethodologyLinks(ctx, db,
		`INSERT OR IGNORE INTO methodology_reference_programs (methodology_id, template_id) VALUES (?, ?)`,
		methodologyID, dedupInt64(templateIDs))
}

// AddMethodologyAllowedEquipment links equipment to the methodology allow-list.
// Idempotent.
func AddMethodologyAllowedEquipment(ctx context.Context, db *sql.DB, methodologyID int64, equipmentIDs []int64) error {
	return addMethodologyLinks(ctx, db,
		`INSERT OR IGNORE INTO methodology_allowed_equipment (methodology_id, equipment_id) VALUES (?, ?)`,
		methodologyID, dedupInt64(equipmentIDs))
}

// AddMethodologyAllowedExercises links exercises to the methodology
// explicit-list allow-list (the override surface on top of pattern scoping).
// Idempotent.
func AddMethodologyAllowedExercises(ctx context.Context, db *sql.DB, methodologyID int64, exerciseIDs []int64) error {
	return addMethodologyLinks(ctx, db,
		`INSERT OR IGNORE INTO methodology_allowed_exercises (methodology_id, exercise_id) VALUES (?, ?)`,
		methodologyID, dedupInt64(exerciseIDs))
}

// AddMethodologyAllowedPatterns links Dan John pattern strings to the
// methodology pattern allow-list. All pattern values are validated against
// IsValidMovementPattern before any rows are written.
func AddMethodologyAllowedPatterns(ctx context.Context, db *sql.DB, methodologyID int64, patterns []string) error {
	for _, p := range patterns {
		if !IsValidMovementPattern(p) {
			return fmt.Errorf("models: invalid movement pattern %q for methodology %d (allowed: push, pull, hinge, squat, carry, ground)", p, methodologyID)
		}
	}

	uniq := dedupString(patterns)
	for _, p := range uniq {
		if _, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO methodology_allowed_patterns (methodology_id, pattern) VALUES (?, ?)`,
			methodologyID, p,
		); err != nil {
			return fmt.Errorf("models: add methodology pattern %q for %d: %w", p, methodologyID, err)
		}
	}
	return nil
}

// DeleteMethodology removes a methodology and (via ON DELETE CASCADE) all
// its link-table rows. Returns ErrNotFound if no row matches.
func DeleteMethodology(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM methodologies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete methodology %d: %w", id, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// addMethodologyLinks is the shared INSERT OR IGNORE loop for the int64-id
// link tables. The caller supplies the SQL with two `?` placeholders
// (methodology_id, link_id).
func addMethodologyLinks(ctx context.Context, db *sql.DB, query string, methodologyID int64, ids []int64) error {
	for _, lid := range ids {
		if _, err := db.ExecContext(ctx, query, methodologyID, lid); err != nil {
			return fmt.Errorf("models: add methodology link (%d, %d): %w", methodologyID, lid, err)
		}
	}
	return nil
}

func scanInt64Column(ctx context.Context, db *sql.DB, query string, args ...any) ([]int64, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanStringColumn(ctx context.Context, db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func dedupInt64(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func dedupString(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
