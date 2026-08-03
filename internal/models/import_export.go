package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// --- Catalog Export Types ---
//
// These types are used by the alive catalog export (admin Settings →
// "Export catalog" / GET /api/catalog/export). The per-athlete export
// types and code that previously lived here were unreachable and were
// removed in d17xxxx (issue #11). If we ever want per-athlete export
// back, reach for git history.

// ExportEquipment is an equipment item in a JSON export.
type ExportEquipment struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// ExportExercise is an exercise in a JSON export, including equipment deps.
type ExportExercise struct {
	Name        string                    `json:"name"`
	Tier        *string                   `json:"tier"`
	FormNotes   *string                   `json:"form_notes"`
	DemoURL     *string                   `json:"demo_url"`
	RestSeconds *int                      `json:"rest_seconds"`
	Featured    bool                      `json:"featured"`
	Equipment   []ExportExerciseEquipment `json:"equipment"`
}

// ExportExerciseEquipment is an equipment link for an exercise in a JSON export.
type ExportExerciseEquipment struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional"`
}

// ExportProgramTemplate is a program template in a JSON export.
type ExportProgramTemplate struct {
	Name             string                  `json:"name"`
	Description      *string                 `json:"description"`
	NumWeeks         int                     `json:"num_weeks"`
	NumDays          int                     `json:"num_days"`
	IsLoop           bool                    `json:"is_loop"`
	PrescribedSets   []ExportPrescribedSet   `json:"prescribed_sets"`
	ProgressionRules []ExportProgressionRule `json:"progression_rules"`
}

// ExportPrescribedSet is a prescribed set in a JSON export.
type ExportPrescribedSet struct {
	Exercise       string   `json:"exercise"`
	Week           int      `json:"week"`
	Day            int      `json:"day"`
	SetNumber      int      `json:"set_number"`
	Reps           *int     `json:"reps"`
	RepType        string   `json:"rep_type"`
	Percentage     *float64 `json:"percentage"`
	AbsoluteWeight *float64 `json:"absolute_weight"`
	RestSeconds    *int     `json:"rest_seconds"`
	SortOrder      int      `json:"sort_order"`
	Notes          *string  `json:"notes"`
}

// ExportProgressionRule is a progression rule in a JSON export.
type ExportProgressionRule struct {
	Exercise  string  `json:"exercise"`
	Increment float64 `json:"increment"`
}

// nullStringPtr converts a sql.NullString to *string for JSON serialization.
func nullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// --- Catalog Export ---

// CatalogJSON is the top-level structure for a global catalog export
// (exercises, equipment, and program templates — no athlete-specific data).
type CatalogJSON struct {
	Version    string                  `json:"version"`
	ExportedAt string                  `json:"exported_at"`
	Type       string                  `json:"type"` // "catalog"
	Equipment  []ExportEquipment       `json:"equipment"`
	Exercises  []ExportExercise        `json:"exercises"`
	Programs   []ExportProgramTemplate `json:"programs"`
}

// BuildCatalogExportJSON gathers all exercises, equipment, and program templates.
func BuildCatalogExportJSON(ctx context.Context, db *sql.DB) (*CatalogJSON, error) {
	catalog := &CatalogJSON{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Type:       "catalog",
	}

	// Equipment — all.
	allEquipment, err := ListEquipment(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("models: catalog export equipment: %w", err)
	}
	for _, eq := range allEquipment {
		catalog.Equipment = append(catalog.Equipment, ExportEquipment{
			Name:        eq.Name,
			Description: nullStringPtr(eq.Description),
		})
	}

	// Exercises — all, with equipment dependencies.
	allExercises, err := ListExercises(ctx, db, "")
	if err != nil {
		return nil, fmt.Errorf("models: catalog export exercises: %w", err)
	}
	for _, ex := range allExercises {
		ee := ExportExercise{
			Name:      ex.Name,
			Tier:      nullStringPtr(ex.Tier),
			FormNotes: nullStringPtr(ex.FormNotes),
			DemoURL:   nullStringPtr(ex.DemoURL),
			Featured:  ex.Featured,
		}
		if ex.RestSeconds.Valid {
			rs := int(ex.RestSeconds.Int64)
			ee.RestSeconds = &rs
		}

		eqLinks, err := ListExerciseEquipment(ctx, db, ex.ID)
		if err != nil {
			return nil, fmt.Errorf("models: catalog export exercise equipment for %d: %w", ex.ID, err)
		}
		for _, link := range eqLinks {
			ee.Equipment = append(ee.Equipment, ExportExerciseEquipment{
				Name:     link.EquipmentName,
				Optional: link.Optional,
			})
		}
		catalog.Exercises = append(catalog.Exercises, ee)
	}

	// Program templates — all, with prescribed sets and progression rules.
	allTemplates, err := ListProgramTemplates(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("models: catalog export program templates: %w", err)
	}
	for _, pt := range allTemplates {
		ept := ExportProgramTemplate{
			Name:        pt.Name,
			Description: nullStringPtr(pt.Description),
			NumWeeks:    pt.NumWeeks,
			NumDays:     pt.NumDays,
			IsLoop:      pt.IsLoop,
		}

		pSets, err := ListPrescribedSets(ctx, db, pt.ID)
		if err != nil {
			return nil, fmt.Errorf("models: catalog export prescribed sets for template %d: %w", pt.ID, err)
		}
		for _, ps := range pSets {
			eps := ExportPrescribedSet{
				Exercise:  ps.ExerciseName,
				Week:      ps.Week,
				Day:       ps.Day,
				SetNumber: ps.SetNumber,
				RepType:   ps.RepType,
				SortOrder: ps.SortOrder,
			}
			if ps.Reps.Valid {
				r := int(ps.Reps.Int64)
				eps.Reps = &r
			}
			if ps.Percentage.Valid {
				p := ps.Percentage.Float64
				eps.Percentage = &p
			}
			if ps.AbsoluteWeight.Valid {
				w := ps.AbsoluteWeight.Float64
				eps.AbsoluteWeight = &w
			}
			if ps.RestSeconds.Valid {
				rest := int(ps.RestSeconds.Int64)
				eps.RestSeconds = &rest
			}
			eps.Notes = nullStringPtr(ps.Notes)
			ept.PrescribedSets = append(ept.PrescribedSets, eps)
		}

		rules, err := ListProgressionRules(ctx, db, pt.ID)
		if err != nil {
			return nil, fmt.Errorf("models: catalog export progression rules for template %d: %w", pt.ID, err)
		}
		for _, r := range rules {
			ept.ProgressionRules = append(ept.ProgressionRules, ExportProgressionRule{
				Exercise:  r.ExerciseName,
				Increment: r.Increment,
			})
		}

		catalog.Programs = append(catalog.Programs, ept)
	}

	return catalog, nil
}

// WriteCatalogJSON serializes the catalog export to JSON and writes it.
func WriteCatalogJSON(w io.Writer, catalog *CatalogJSON) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(catalog)
}

// --- Import Types ---

// ValidationWarning represents a non-blocking data quality issue found during
// import upload. Warnings are surfaced to the user via the upload response
// but do not prevent commit.
type ValidationWarning struct {
	Entity  string // "workout", "set", "training_max", "body_weight"
	Field   string // "weight", "reps", "rpe", "date", "rep_type"
	Message string
}

// ImportResult summarizes what was imported after execution.
type ImportResult struct {
	WorkoutsCreated      int
	SetsCreated          int
	ExercisesCreated     int
	ExercisesSkipped     int
	EquipmentCreated     int
	EquipmentSkipped     int
	AssignmentsCreated   int
	AssignmentsSkipped   int
	TrainingMaxesCreated int
	TrainingMaxesSkipped int
	BodyWeightsCreated   int
	BodyWeightsSkipped   int
	ReviewsCreated       int
	ProgramsCreated      int
	ProgramsSkipped      int
	WorkoutsSkipped      int // existing date conflicts
}
