package models

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// MethodologySeed is the JSON schema for the embedded methodology seed file
// (internal/database/seed-methodologies.json). Each entry becomes a
// methodologies row plus the link-table rows resolvable via the names
// referenced (program template name, equipment name, exercise name).
//
// Methodologies are seeded via a DEDICATED path (ApplyMethodologySeed below) —
// NOT routed through importers.ParseCatalogJSON / ExecuteCatalogImport. They
// are app configuration, not user-importable program content (see ADR 016
// HOF-003 [fix]).
type MethodologySeed struct {
	Version        string                `json:"version"`
	Type           string                `json:"type"` // "methodologies"
	ExportedAt     string                `json:"exported_at"`
	Methodologies  []MethodologySeedItem `json:"methodologies"`
}

// MethodologySeedItem is one row of methodology seed data plus the names
// of dependent rows to link.
type MethodologySeedItem struct {
	Key               string   `json:"key"`
	Name              string   `json:"name"`
	Audience          *string  `json:"audience"`
	ApplicableTiers   *string  `json:"applicable_tiers"`
	Philosophy        *string  `json:"philosophy"`
	Definition        string   `json:"definition"`
	ReferencePrograms []string `json:"reference_programs"`
	AllowedEquipment  []string `json:"allowed_equipment"`
	AllowedPatterns   []string `json:"allowed_patterns"`
	AllowedExercises  []string `json:"allowed_exercises"`
}

// ParseMethodologySeed decodes the embedded methodology seed JSON.
func ParseMethodologySeed(r io.Reader) (*MethodologySeed, error) {
	var s MethodologySeed
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return nil, fmt.Errorf("models: decode methodology seed: %w", err)
	}
	if s.Type != "methodologies" {
		return nil, fmt.Errorf("models: expected methodology seed type \"methodologies\", got %q", s.Type)
	}
	return &s, nil
}

// MethodologySeedResult summarizes an ApplyMethodologySeed run.
type MethodologySeedResult struct {
	MethodologiesCreated int
	MethodologiesSkipped int // already-present (matched by key)

	ReferenceProgramLinks int
	EquipmentLinks        int
	PatternLinks          int
	ExerciseLinks         int

	// MissingProgramRefs / MissingEquipment / MissingExercises are names from
	// the seed file that did not resolve to a row in the catalog. They are
	// surfaced (not failed) — the catalog may legitimately have evolved.
	MissingProgramRefs []string
	MissingEquipment   []string
	MissingExercises   []string
}

// ApplyMethodologySeed inserts methodologies from the seed file along with
// their link rows. Idempotent: matches existing methodologies by key, skips
// the row, and updates link tables to the union of existing + seed.
//
// All work runs in a single transaction. Names that don't resolve to a
// program / equipment / exercise are reported in the result but do NOT
// fail the import — the catalog may have evolved since the seed was
// authored.
func ApplyMethodologySeed(db *sql.DB, seed *MethodologySeed) (*MethodologySeedResult, error) {
	if seed == nil {
		return nil, fmt.Errorf("models: ApplyMethodologySeed called with nil seed")
	}
	result := &MethodologySeedResult{}

	// Build name → id lookup maps from the live DB. Done once outside the
	// loop so we don't query per-link.
	programIDs, err := nameToIDMapPrograms(db)
	if err != nil {
		return nil, fmt.Errorf("models: methodology seed: load program template ids: %w", err)
	}
	equipmentIDs, err := nameToIDMapEquipment(db)
	if err != nil {
		return nil, fmt.Errorf("models: methodology seed: load equipment ids: %w", err)
	}
	exerciseIDs, err := nameToIDMapExercises(db)
	if err != nil {
		return nil, fmt.Errorf("models: methodology seed: load exercise ids: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin methodology seed tx: %w", err)
	}
	defer tx.Rollback()

	for _, m := range seed.Methodologies {
		if m.Key == "" || m.Name == "" || m.Definition == "" {
			return nil, fmt.Errorf("models: methodology seed: row missing required field (key=%q, name=%q)", m.Key, m.Name)
		}
		var audience sql.NullString
		if m.Audience != nil && *m.Audience != "" {
			if *m.Audience != MethodologyAudienceYouth && *m.Audience != MethodologyAudienceAdult {
				return nil, fmt.Errorf("models: methodology seed %q: invalid audience %q", m.Key, *m.Audience)
			}
			audience = sql.NullString{String: *m.Audience, Valid: true}
		}
		var tiers sql.NullString
		if m.ApplicableTiers != nil && *m.ApplicableTiers != "" {
			tiers = sql.NullString{String: *m.ApplicableTiers, Valid: true}
		}
		var philosophy sql.NullString
		if m.Philosophy != nil && *m.Philosophy != "" {
			philosophy = sql.NullString{String: *m.Philosophy, Valid: true}
		}

		// Validate pattern strings up-front so a bad seed file fails fast
		// rather than rolling back mid-transaction.
		for _, p := range m.AllowedPatterns {
			if !IsValidMovementPattern(p) {
				return nil, fmt.Errorf("models: methodology seed %q: invalid pattern %q", m.Key, p)
			}
		}

		// Insert or fetch by key.
		methodologyID, created, err := upsertMethodologyByKey(tx, m.Key, m.Name, audience, tiers, philosophy, m.Definition)
		if err != nil {
			return nil, fmt.Errorf("models: methodology seed %q: %w", m.Key, err)
		}
		if created {
			result.MethodologiesCreated++
		} else {
			result.MethodologiesSkipped++
		}

		// Reference programs.
		for _, pname := range m.ReferencePrograms {
			id, ok := programIDs[strings.ToLower(pname)]
			if !ok {
				result.MissingProgramRefs = appendUnique(result.MissingProgramRefs, pname)
				continue
			}
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO methodology_reference_programs (methodology_id, template_id) VALUES (?, ?)`,
				methodologyID, id,
			); err != nil {
				return nil, fmt.Errorf("models: methodology seed %q link program %q: %w", m.Key, pname, err)
			}
			result.ReferenceProgramLinks++
		}

		// Allowed equipment.
		for _, ename := range m.AllowedEquipment {
			id, ok := equipmentIDs[strings.ToLower(ename)]
			if !ok {
				result.MissingEquipment = appendUnique(result.MissingEquipment, ename)
				continue
			}
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO methodology_allowed_equipment (methodology_id, equipment_id) VALUES (?, ?)`,
				methodologyID, id,
			); err != nil {
				return nil, fmt.Errorf("models: methodology seed %q link equipment %q: %w", m.Key, ename, err)
			}
			result.EquipmentLinks++
		}

		// Allowed patterns.
		for _, p := range m.AllowedPatterns {
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO methodology_allowed_patterns (methodology_id, pattern) VALUES (?, ?)`,
				methodologyID, p,
			); err != nil {
				return nil, fmt.Errorf("models: methodology seed %q link pattern %q: %w", m.Key, p, err)
			}
			result.PatternLinks++
		}

		// Allowed exercises (explicit-list override).
		for _, ename := range m.AllowedExercises {
			id, ok := exerciseIDs[strings.ToLower(ename)]
			if !ok {
				result.MissingExercises = appendUnique(result.MissingExercises, ename)
				continue
			}
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO methodology_allowed_exercises (methodology_id, exercise_id) VALUES (?, ?)`,
				methodologyID, id,
			); err != nil {
				return nil, fmt.Errorf("models: methodology seed %q link exercise %q: %w", m.Key, ename, err)
			}
			result.ExerciseLinks++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit methodology seed: %w", err)
	}
	return result, nil
}

// ApplyMethodologySeedFromBytes is the convenience wrapper used by the
// first-run bootstrap path in cmd/replog/main.go.
func ApplyMethodologySeedFromBytes(db *sql.DB, data []byte) (*MethodologySeedResult, error) {
	seed, err := ParseMethodologySeed(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return ApplyMethodologySeed(db, seed)
}

// upsertMethodologyByKey returns the id of the row matching key — inserting
// it if absent. Returns (id, created, err). Existing rows are NOT updated
// (definition/name edits should go through a deliberate model write).
func upsertMethodologyByKey(tx *sql.Tx, key, name string, audience, tiers, philosophy sql.NullString, definition string) (int64, bool, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM methodologies WHERE key = ?`, key).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, fmt.Errorf("lookup by key: %w", err)
	}
	err = tx.QueryRow(
		`INSERT INTO methodologies (key, name, audience, applicable_tiers, philosophy, definition)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		key, name, audience, tiers, philosophy, definition,
	).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("insert: %w", err)
	}
	return id, true, nil
}

func nameToIDMapPrograms(db *sql.DB) (map[string]int64, error) {
	rows, err := db.Query(`SELECT id, name FROM program_templates WHERE athlete_id IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = id
	}
	return out, rows.Err()
}

func nameToIDMapEquipment(db *sql.DB) (map[string]int64, error) {
	rows, err := db.Query(`SELECT id, name FROM equipment`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = id
	}
	return out, rows.Err()
}

func nameToIDMapExercises(db *sql.DB) (map[string]int64, error) {
	rows, err := db.Query(`SELECT id, name FROM exercises`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = id
	}
	return out, rows.Err()
}

func appendUnique(in []string, v string) []string {
	for _, x := range in {
		if x == v {
			return in
		}
	}
	return append(in, v)
}

// BackfillResult summarizes a BackfillExerciseMovementPatterns run.
type BackfillResult struct {
	ExercisesConsidered int
	ExercisesTagged     int // had >=1 pattern added
	PatternsInserted    int
	SkippedAlreadyTagged int // exercise already had at least one pattern row
}

// BackfillExerciseMovementPatterns reads the embedded catalog seed and adds
// movement-pattern tags to existing exercises that match by name (case-
// insensitive). Idempotent: an exercise that already has ANY tag row is
// considered "already managed" and is skipped — manual edits via
// SetExerciseMovementPatterns are preserved.
//
// Use case: an existing database has exercises seeded before ADR 016 Phase 1
// ran. Migration 0004 created the empty table; this backfill populates it
// from the seed catalog on the next startup. New installs (where the catalog
// importer wrote the tags inline) hit the "already-tagged" branch and skip.
func BackfillExerciseMovementPatterns(db *sql.DB, catalogSeed []byte) (*BackfillResult, error) {
	parsed, err := decodeCatalogExerciseTags(catalogSeed)
	if err != nil {
		return nil, fmt.Errorf("models: backfill movement patterns: %w", err)
	}

	result := &BackfillResult{}
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("models: begin backfill tx: %w", err)
	}
	defer tx.Rollback()

	for _, item := range parsed {
		if len(item.Patterns) == 0 {
			continue
		}
		result.ExercisesConsidered++

		var exerciseID int64
		err := tx.QueryRow(`SELECT id FROM exercises WHERE name = ? COLLATE NOCASE`, item.Name).Scan(&exerciseID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("models: lookup %q: %w", item.Name, err)
		}

		// Skip if any pattern row already exists for this exercise — preserves
		// manual edits and avoids double-tagging on new-install repeats.
		var existing int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM exercise_movement_patterns WHERE exercise_id = ?`, exerciseID).Scan(&existing); err != nil {
			return nil, fmt.Errorf("models: count existing patterns for %q: %w", item.Name, err)
		}
		if existing > 0 {
			result.SkippedAlreadyTagged++
			continue
		}

		added := 0
		for _, p := range item.Patterns {
			if !IsValidMovementPattern(p) {
				return nil, fmt.Errorf("models: backfill: invalid pattern %q for %q", p, item.Name)
			}
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO exercise_movement_patterns (exercise_id, pattern) VALUES (?, ?)`,
				exerciseID, p,
			); err != nil {
				return nil, fmt.Errorf("models: backfill insert %q/%q: %w", item.Name, p, err)
			}
			added++
		}
		if added > 0 {
			result.ExercisesTagged++
			result.PatternsInserted += added
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: commit backfill: %w", err)
	}
	return result, nil
}

// catalogExerciseTagEntry is the minimal catalog-JSON shape this backfill needs.
type catalogExerciseTagEntry struct {
	Name     string   `json:"name"`
	Patterns []string `json:"movement_patterns"`
}

func decodeCatalogExerciseTags(data []byte) ([]catalogExerciseTagEntry, error) {
	var wrapper struct {
		Type      string                    `json:"type"`
		Exercises []catalogExerciseTagEntry `json:"exercises"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("decode catalog json: %w", err)
	}
	if wrapper.Type != "catalog" {
		return nil, fmt.Errorf("expected catalog type, got %q", wrapper.Type)
	}
	return wrapper.Exercises, nil
}
