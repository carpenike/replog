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

	result, err := ExecuteCatalogImport(db, ms, nil)
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
	first, err := ExecuteCatalogImport(db, ms, nil)
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
	second, err := ExecuteCatalogImport(db, ms2, nil)
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

func TestCatalogImport_AssignsToAthlete(t *testing.T) {
	db := testDB(t)

	// Create an athlete to import for.
	athlete, err := CreateAthlete(db, "Import Test", "foundational", "", "", "", "", "", sql.NullInt64{}, true)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// Build a minimal catalog JSON.
	catalogJSON := `{
		"version": "1.0",
		"type": "catalog",
		"exercises": [
			{"name": "Squat", "tier": "foundational"}
		],
		"programs": [
			{
				"name": "Test Program",
				"num_weeks": 4,
				"num_days": 3,
				"is_loop": false,
				"prescribed_sets": [
					{"exercise": "Squat", "week": 1, "day": 1, "set_number": 1, "reps": 5, "rep_type": "reps", "percentage": 0.75, "sort_order": 1}
				]
			}
		]
	}`

	parsed, err := importers.ParseCatalogJSON(bytes.NewBufferString(catalogJSON))
	if err != nil {
		t.Fatalf("parse catalog JSON: %v", err)
	}

	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}

	// Import with athlete ID — should create template AND assign it.
	result, err := ExecuteCatalogImport(db, ms, &athlete.ID)
	if err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}

	if result.ProgramsCreated != 1 {
		t.Errorf("ProgramsCreated: got %d, want 1", result.ProgramsCreated)
	}
	if result.ProgramsAssigned != 1 {
		t.Errorf("ProgramsAssigned: got %d, want 1", result.ProgramsAssigned)
	}
	if len(result.CreatedTemplateIDs) != 1 {
		t.Errorf("CreatedTemplateIDs: got %d, want 1", len(result.CreatedTemplateIDs))
	}

	// Verify the athlete has an active program.
	active, err := GetActiveProgram(db, athlete.ID)
	if err != nil {
		t.Fatalf("GetActiveProgram: %v", err)
	}
	if active == nil {
		t.Fatal("expected active program, got nil")
	}
	if active.TemplateName != "Test Program" {
		t.Errorf("active program name: got %q, want %q", active.TemplateName, "Test Program")
	}
	if !active.Active {
		t.Error("expected active = true")
	}
}

func TestCatalogImport_DeactivatesPriorProgram(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(db, "Deact Test", "foundational", "", "", "", "", "", sql.NullInt64{}, true)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// Create and assign a program manually first.
	tmpl, err := CreateProgramTemplate(db, nil, "Old Program", "", 4, 3, false, "")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	_, err = AssignProgram(db, athlete.ID, tmpl.ID, "2025-01-01", "", "", "primary", "")
	if err != nil {
		t.Fatalf("assign program: %v", err)
	}

	// Verify old program is active.
	activeBefore, _ := GetActiveProgram(db, athlete.ID)
	if activeBefore == nil || activeBefore.TemplateName != "Old Program" {
		t.Fatal("expected Old Program to be active before import")
	}

	// Import a new catalog program scoped to the athlete.
	catalogJSON := `{
		"version": "1.0",
		"type": "catalog",
		"exercises": [
			{"name": "Bench Press", "tier": "foundational"}
		],
		"programs": [
			{
				"name": "New Program",
				"num_weeks": 4,
				"num_days": 3,
				"is_loop": false,
				"prescribed_sets": [
					{"exercise": "Bench Press", "week": 1, "day": 1, "set_number": 1, "reps": 5, "rep_type": "reps", "percentage": 0.80, "sort_order": 1}
				]
			}
		]
	}`
	parsed, err := importers.ParseCatalogJSON(bytes.NewBufferString(catalogJSON))
	if err != nil {
		t.Fatalf("parse catalog JSON: %v", err)
	}
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}

	result, err := ExecuteCatalogImport(db, ms, &athlete.ID)
	if err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}
	if result.ProgramsAssigned != 1 {
		t.Errorf("ProgramsAssigned: got %d, want 1", result.ProgramsAssigned)
	}

	// Verify the new program is now active and old is deactivated.
	activeAfter, err := GetActiveProgram(db, athlete.ID)
	if err != nil {
		t.Fatalf("GetActiveProgram after import: %v", err)
	}
	if activeAfter == nil {
		t.Fatal("expected active program after import, got nil")
	}
	if activeAfter.TemplateName != "New Program" {
		t.Errorf("active program name: got %q, want %q", activeAfter.TemplateName, "New Program")
	}

	// Verify old program is deactivated.
	allPrograms, err := ListAthletePrograms(db, athlete.ID)
	if err != nil {
		t.Fatalf("ListAthletePrograms: %v", err)
	}
	if len(allPrograms) != 2 {
		t.Fatalf("expected 2 programs, got %d", len(allPrograms))
	}
	for _, p := range allPrograms {
		if p.TemplateName == "Old Program" && p.Active {
			t.Error("expected Old Program to be deactivated")
		}
	}
}

func TestCatalogImport_NilAthlete_NoAssignment(t *testing.T) {
	db := testDB(t)

	catalogJSON := `{
		"version": "1.0",
		"type": "catalog",
		"exercises": [
			{"name": "Deadlift", "tier": "foundational"}
		],
		"programs": [
			{
				"name": "Global Program",
				"num_weeks": 4,
				"num_days": 3,
				"is_loop": false,
				"prescribed_sets": [
					{"exercise": "Deadlift", "week": 1, "day": 1, "set_number": 1, "reps": 5, "rep_type": "reps", "percentage": 0.85, "sort_order": 1}
				]
			}
		]
	}`
	parsed, err := importers.ParseCatalogJSON(bytes.NewBufferString(catalogJSON))
	if err != nil {
		t.Fatalf("parse catalog JSON: %v", err)
	}
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}

	result, err := ExecuteCatalogImport(db, ms, nil)
	if err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}
	if result.ProgramsCreated != 1 {
		t.Errorf("ProgramsCreated: got %d, want 1", result.ProgramsCreated)
	}
	if result.ProgramsAssigned != 0 {
		t.Errorf("ProgramsAssigned: got %d, want 0 (no athlete)", result.ProgramsAssigned)
	}
}
