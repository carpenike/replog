package models

import (
	"bytes"
	"database/sql"
	"testing"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/importers"
)

func TestSeedCatalogImport(t *testing.T) {
	db := testDB(t)

	// Parse the embedded seed catalog.
	data := database.SeedCatalog()
	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse seed catalog: %v", err)
	}

	// Build mappings — empty DB, so all entities are created.
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}

	result, err := ExecuteCatalogImport(db, ms)
	if err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}

	// Verify all entities were created.
	if result.EquipmentCreated != len(parsed.Equipment) {
		t.Errorf("equipment: got %d created, want %d", result.EquipmentCreated, len(parsed.Equipment))
	}
	if result.ExercisesCreated != len(parsed.Exercises) {
		t.Errorf("exercises: got %d created, want %d", result.ExercisesCreated, len(parsed.Exercises))
	}
	if result.ProgramsCreated != len(parsed.Programs) {
		t.Errorf("programs: got %d created, want %d", result.ProgramsCreated, len(parsed.Programs))
	}
	if result.PrescribedSets == 0 {
		t.Error("expected prescribed sets to be created, got 0")
	}
	if result.ProgressionRules == 0 {
		t.Error("expected progression rules to be created, got 0")
	}

	// Verify data is queryable.
	exercises, err := ListExercises(db, "")
	if err != nil {
		t.Fatalf("ListExercises: %v", err)
	}
	if len(exercises) != len(parsed.Exercises) {
		t.Errorf("ListExercises: got %d, want %d", len(exercises), len(parsed.Exercises))
	}

	equipment, err := ListEquipment(db)
	if err != nil {
		t.Fatalf("ListEquipment: %v", err)
	}
	if len(equipment) != len(parsed.Equipment) {
		t.Errorf("ListEquipment: got %d, want %d", len(equipment), len(parsed.Equipment))
	}

	programs, err := ListProgramTemplates(db)
	if err != nil {
		t.Fatalf("ListProgramTemplates: %v", err)
	}
	if len(programs) != len(parsed.Programs) {
		t.Errorf("ListProgramTemplates: got %d, want %d", len(programs), len(parsed.Programs))
	}
}

func TestSeedCatalogImport_Idempotent(t *testing.T) {
	db := testDB(t)

	// Parse the embedded seed catalog.
	data := database.SeedCatalog()
	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse seed catalog: %v", err)
	}

	// First import — all created.
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}
	first, err := ExecuteCatalogImport(db, ms)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if first.ExercisesCreated == 0 {
		t.Fatal("first import should have created exercises")
	}

	// Second import — re-parse and build mappings against existing entities.
	parsed2, _ := importers.ParseCatalogJSON(bytes.NewReader(data))
	existingEx := listEntityExercises(t, db)
	existingEq := listEntityEquipment(t, db)
	existingPr := listEntityPrograms(t, db)

	ms2 := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed2.Exercises, existingEx),
		Equipment: importers.BuildEquipmentMappings(parsed2.Equipment, existingEq),
		Programs:  importers.BuildProgramMappings(parsed2.Programs, existingPr),
		Parsed:    parsed2,
	}
	second, err := ExecuteCatalogImport(db, ms2)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}

	// Nothing new should be created — everything mapped to existing.
	if second.ExercisesCreated != 0 {
		t.Errorf("second import: got %d exercises created, want 0", second.ExercisesCreated)
	}
	if second.EquipmentCreated != 0 {
		t.Errorf("second import: got %d equipment created, want 0", second.EquipmentCreated)
	}
	if second.ProgramsCreated != 0 {
		t.Errorf("second import: got %d programs created, want 0", second.ProgramsCreated)
	}
}

// listEntityExercises returns exercises as ExistingEntity for mapping tests.
func listEntityExercises(t testing.TB, db *sql.DB) []importers.ExistingEntity {
	t.Helper()
	exercises, err := ListExercises(db, "")
	if err != nil {
		t.Fatalf("list exercises: %v", err)
	}
	result := make([]importers.ExistingEntity, len(exercises))
	for i, e := range exercises {
		result[i] = importers.ExistingEntity{ID: e.ID, Name: e.Name}
	}
	return result
}

// listEntityEquipment returns equipment as ExistingEntity for mapping tests.
func listEntityEquipment(t testing.TB, db *sql.DB) []importers.ExistingEntity {
	t.Helper()
	equipment, err := ListEquipment(db)
	if err != nil {
		t.Fatalf("list equipment: %v", err)
	}
	result := make([]importers.ExistingEntity, len(equipment))
	for i, e := range equipment {
		result[i] = importers.ExistingEntity{ID: e.ID, Name: e.Name}
	}
	return result
}

// listEntityPrograms returns programs as ExistingEntity for mapping tests.
func listEntityPrograms(t testing.TB, db *sql.DB) []importers.ExistingEntity {
	t.Helper()
	programs, err := ListProgramTemplates(db)
	if err != nil {
		t.Fatalf("list programs: %v", err)
	}
	result := make([]importers.ExistingEntity, len(programs))
	for i, p := range programs {
		result[i] = importers.ExistingEntity{ID: p.ID, Name: p.Name}
	}
	return result
}
