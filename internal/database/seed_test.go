package database

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/carpenike/replog/internal/importers"
)

func TestSeedCatalog_NotEmpty(t *testing.T) {
	data := SeedCatalog()
	if len(data) == 0 {
		t.Fatal("SeedCatalog() returned empty data")
	}
}

func TestSeedCatalog_ValidJSON(t *testing.T) {
	data := SeedCatalog()
	if !json.Valid(data) {
		t.Fatal("SeedCatalog() returned invalid JSON")
	}
}

func TestSeedCatalog_ParsesCatalogJSON(t *testing.T) {
	data := SeedCatalog()
	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCatalogJSON failed: %v", err)
	}

	if len(parsed.Equipment) == 0 {
		t.Error("expected equipment in seed catalog, got none")
	}
	if len(parsed.Exercises) == 0 {
		t.Error("expected exercises in seed catalog, got none")
	}
	if len(parsed.Programs) == 0 {
		t.Error("expected programs in seed catalog, got none")
	}
}

func TestSeedCatalog_ExerciseReferences(t *testing.T) {
	data := SeedCatalog()
	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCatalogJSON failed: %v", err)
	}

	// Build a set of known exercise names.
	exerciseNames := make(map[string]bool, len(parsed.Exercises))
	for _, ex := range parsed.Exercises {
		exerciseNames[ex.Name] = true
	}

	// Verify all prescribed sets and progression rules reference valid exercises.
	for _, prog := range parsed.Programs {
		for _, ps := range prog.Template.PrescribedSets {
			if !exerciseNames[ps.Exercise] {
				t.Errorf("program %q prescribed set references unknown exercise %q",
					prog.Template.Name, ps.Exercise)
			}
		}
		for _, pr := range prog.Template.ProgressionRules {
			if !exerciseNames[pr.Exercise] {
				t.Errorf("program %q progression rule references unknown exercise %q",
					prog.Template.Name, pr.Exercise)
			}
		}
	}
}

func TestSeedCatalog_EquipmentReferences(t *testing.T) {
	data := SeedCatalog()
	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseCatalogJSON failed: %v", err)
	}

	// Build a set of known equipment names.
	equipmentNames := make(map[string]bool, len(parsed.Equipment))
	for _, eq := range parsed.Equipment {
		equipmentNames[eq.Name] = true
	}

	// Verify all exercise equipment references are valid.
	for _, ex := range parsed.Exercises {
		for _, eq := range ex.Equipment {
			if !equipmentNames[eq.Name] {
				t.Errorf("exercise %q references unknown equipment %q",
					ex.Name, eq.Name)
			}
		}
	}
}
