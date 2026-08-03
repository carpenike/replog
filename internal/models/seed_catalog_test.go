package models

import (
	"bytes"
	"context"
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

	result, err := ExecuteCatalogImport(context.Background(), db, ms, nil, false)
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
	exercises, err := ListExercises(context.Background(), db, "")
	if err != nil {
		t.Fatalf("ListExercises: %v", err)
	}
	if len(exercises) != len(parsed.Exercises) {
		t.Errorf("ListExercises: got %d, want %d", len(exercises), len(parsed.Exercises))
	}

	equipment, err := ListEquipment(context.Background(), db)
	if err != nil {
		t.Fatalf("ListEquipment: %v", err)
	}
	if len(equipment) != len(parsed.Equipment) {
		t.Errorf("ListEquipment: got %d, want %d", len(equipment), len(parsed.Equipment))
	}

	programs, err := ListProgramTemplates(context.Background(), db)
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
	first, err := ExecuteCatalogImport(context.Background(), db, ms, nil, false)
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
	second, err := ExecuteCatalogImport(context.Background(), db, ms2, nil, false)
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

func TestCatalogExportImport_RoundTripsPrescribedSetRestSeconds(t *testing.T) {
	source := testDB(t)
	template, err := CreateProgramTemplate(context.Background(), source, nil, "Galpin 3-to-5", "", 1, 3, true, "adult")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	exercise, err := CreateExercise(context.Background(), source, "Squat", "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	reps := 5
	restSeconds := 180
	if _, err := CreatePrescribedSetWithRest(context.Background(), source, template.ID, exercise.ID, 1, 1, 1, &reps, nil, nil, &restSeconds, 0, "reps", ""); err != nil {
		t.Fatalf("create prescribed set: %v", err)
	}

	catalog, err := BuildCatalogExportJSON(context.Background(), source)
	if err != nil {
		t.Fatalf("build catalog export: %v", err)
	}
	var encoded bytes.Buffer
	if err := WriteCatalogJSON(&encoded, catalog); err != nil {
		t.Fatalf("write catalog export: %v", err)
	}
	parsed, err := importers.ParseCatalogJSON(&encoded)
	if err != nil {
		t.Fatalf("parse catalog export: %v", err)
	}
	if len(parsed.Programs) != 1 || len(parsed.Programs[0].Template.PrescribedSets) != 1 {
		t.Fatalf("unexpected exported programs: %+v", parsed.Programs)
	}
	if got := parsed.Programs[0].Template.PrescribedSets[0].RestSeconds; got == nil || *got != 180 {
		t.Fatalf("exported rest_seconds = %v, want 180", got)
	}

	target := testDB(t)
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}
	if _, err := ExecuteCatalogImport(context.Background(), target, ms, nil, false); err != nil {
		t.Fatalf("import catalog export: %v", err)
	}
	templates, err := ListProgramTemplates(context.Background(), target)
	if err != nil || len(templates) != 1 {
		t.Fatalf("list imported templates: %v, %d", err, len(templates))
	}
	sets, err := ListPrescribedSets(context.Background(), target, templates[0].ID)
	if err != nil || len(sets) != 1 {
		t.Fatalf("list imported sets: %v, %d", err, len(sets))
	}
	if !sets[0].RestSeconds.Valid || sets[0].RestSeconds.Int64 != 180 {
		t.Errorf("imported rest_seconds = %+v, want 180", sets[0].RestSeconds)
	}
}

// TestSeedCatalogImport_MovementPatterns confirms the additive movement-pattern
// extension persists tags from seed-catalog.json into exercise_movement_patterns
// via the import tx (ADR 016 Phase 1).
func TestSeedCatalogImport_MovementPatterns(t *testing.T) {
	db := testDB(t)

	data := database.SeedCatalog()
	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse seed catalog: %v", err)
	}
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}
	if _, err := ExecuteCatalogImport(context.Background(), db, ms, nil, false); err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}

	// Spot-check a handful of well-known exercises across pattern families.
	cases := []struct {
		name string
		want []string
	}{
		{"Squat", []string{"squat"}},
		{"Bench Press", []string{"push"}},
		{"Deadlift", []string{"hinge"}},
		{"Pull-up", []string{"pull"}},
		{"Farmer's Carry", []string{"carry"}},
		{"Plank", []string{"ground"}},
		{"Trap Bar Deadlift", []string{"hinge", "squat"}},
		{"Track Sprint", []string{"conditioning"}},
		{"Hamstring Stretch", []string{"mobility"}},
	}
	for _, tc := range cases {
		ex, err := getExerciseByName(context.Background(), db, tc.name)
		if err != nil {
			t.Errorf("look up %q: %v", tc.name, err)
			continue
		}
		got, err := ListExerciseMovementPatterns(context.Background(), db, ex.ID)
		if err != nil {
			t.Errorf("list patterns for %q: %v", tc.name, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %d patterns %v, want %v", tc.name, len(got), got, tc.want)
			continue
		}
		for i, p := range tc.want {
			if got[i] != p {
				t.Errorf("%s pattern[%d] = %q, want %q (got=%v)", tc.name, i, got[i], p, got)
			}
		}
	}

	// And that at least some exercises were tagged (catches a silent regression
	// if the importer ever drops the field).
	var tagged int
	if err := db.QueryRow(`SELECT COUNT(DISTINCT exercise_id) FROM exercise_movement_patterns`).Scan(&tagged); err != nil {
		t.Fatalf("count tagged: %v", err)
	}
	if tagged < 100 {
		t.Errorf("only %d exercises tagged; expected the seed catalog to tag most of them", tagged)
	}
}

func getExerciseByName(ctx context.Context, db *sql.DB, name string) (*Exercise, error) {
	rows, err := db.Query(`SELECT id, name, tier, form_notes, demo_url, rest_seconds, featured, created_at, updated_at FROM exercises WHERE name = ? COLLATE NOCASE`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	e := &Exercise{}
	if err := rows.Scan(&e.ID, &e.Name, &e.Tier, &e.FormNotes, &e.DemoURL, &e.RestSeconds, &e.Featured, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	return e, nil
}

// listEntityExercises returns exercises as ExistingEntity for mapping tests.
func listEntityExercises(t testing.TB, db *sql.DB) []importers.ExistingEntity {
	t.Helper()
	exercises, err := ListExercises(context.Background(), db, "")
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
	equipment, err := ListEquipment(context.Background(), db)
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
	programs, err := ListProgramTemplates(context.Background(), db)
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
	athlete, err := CreateAthlete(context.Background(), db, "Import Test", "foundational", "", "", "", "", "", sql.NullInt64{}, true)
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
	result, err := ExecuteCatalogImport(context.Background(), db, ms, &athlete.ID, true)
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
	active, err := GetActiveProgram(context.Background(), db, athlete.ID)
	if err != nil {
		t.Fatalf("GetActiveProgram: %v", err)
	}
	if active == nil {
		t.Fatal("expected active program, got nil")
		return
	}
	if active.TemplateName != "Test Program" {
		t.Errorf("active program name: got %q, want %q", active.TemplateName, "Test Program")
	}
	if !active.Active {
		t.Error("expected active = true")
	}
}

// TestCatalogImport_ReferencesExistingExercise reproduces the production bug
// where an AI-coach generation produced a catalog with an EMPTY exercises[]
// array while referencing existing catalog exercises by name inside
// prescribed_sets / progression_rules. The importer must resolve those names
// against the existing catalog instead of silently dropping every set (which
// previously left the athlete with an empty program template).
func TestCatalogImport_ReferencesExistingExercise(t *testing.T) {
	db := testDB(t)

	// Seed an exercise into the catalog (as a prior import would have).
	seedJSON := `{
		"version": "1.0",
		"type": "catalog",
		"exercises": [
			{"name": "Bench Press", "tier": "foundational"}
		]
	}`
	seedParsed, err := importers.ParseCatalogJSON(bytes.NewBufferString(seedJSON))
	if err != nil {
		t.Fatalf("parse seed catalog: %v", err)
	}
	seedMS := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(seedParsed.Exercises, nil),
		Parsed:    seedParsed,
	}
	if _, err := ExecuteCatalogImport(context.Background(), db, seedMS, nil, false); err != nil {
		t.Fatalf("seed import: %v", err)
	}

	// Now import a program-only catalog with an EMPTY exercises[] array that
	// references "Bench Press" purely by name — exactly what the AI coach
	// produced for the Caydan generation.
	catalogJSON := `{
		"version": "1.0",
		"type": "catalog",
		"exercises": [],
		"programs": [
			{
				"name": "Block 1",
				"num_weeks": 4,
				"num_days": 3,
				"is_loop": false,
				"prescribed_sets": [
					{"exercise": "Bench Press", "week": 1, "day": 1, "set_number": 1, "reps": 5, "rep_type": "reps", "absolute_weight": 95, "sort_order": 1}
				],
				"progression_rules": [
					{"exercise": "Bench Press", "increment": 5}
				]
			}
		]
	}`
	parsed, err := importers.ParseCatalogJSON(bytes.NewBufferString(catalogJSON))
	if err != nil {
		t.Fatalf("parse catalog JSON: %v", err)
	}

	// Build exercise mappings against the empty exercises[] array — this is
	// what the generation-execute handler does, so ms.Exercises is empty.
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, listEntityExercises(t, db)),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}

	result, err := ExecuteCatalogImport(context.Background(), db, ms, nil, false)
	if err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}

	if result.ProgramsCreated != 1 {
		t.Fatalf("ProgramsCreated: got %d, want 1", result.ProgramsCreated)
	}
	if result.PrescribedSets != 1 {
		t.Errorf("PrescribedSets: got %d, want 1 (prescribed set was silently dropped)", result.PrescribedSets)
	}
	if result.ProgressionRules != 1 {
		t.Errorf("ProgressionRules: got %d, want 1 (progression rule was silently dropped)", result.ProgressionRules)
	}

	// The created template must actually carry the prescribed set.
	if len(result.CreatedTemplateIDs) != 1 {
		t.Fatalf("CreatedTemplateIDs: got %d, want 1", len(result.CreatedTemplateIDs))
	}
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM prescribed_sets WHERE template_id = ?`,
		result.CreatedTemplateIDs[0],
	).Scan(&count); err != nil {
		t.Fatalf("count prescribed sets: %v", err)
	}
	if count != 1 {
		t.Errorf("prescribed_sets rows for template: got %d, want 1", count)
	}
}

func TestCatalogImport_DeactivatesPriorProgram(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(context.Background(), db, "Deact Test", "foundational", "", "", "", "", "", sql.NullInt64{}, true)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// Create and assign a program manually first.
	tmpl, err := CreateProgramTemplate(context.Background(), db, nil, "Old Program", "", 4, 3, false, "")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	_, err = AssignProgram(context.Background(), db, athlete.ID, tmpl.ID, "2025-01-01", "", "", "primary", "")
	if err != nil {
		t.Fatalf("assign program: %v", err)
	}

	// Verify old program is active.
	activeBefore, _ := GetActiveProgram(context.Background(), db, athlete.ID)
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

	result, err := ExecuteCatalogImport(context.Background(), db, ms, &athlete.ID, true)
	if err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}
	if result.ProgramsAssigned != 1 {
		t.Errorf("ProgramsAssigned: got %d, want 1", result.ProgramsAssigned)
	}

	// Verify the new program is now active and old is deactivated.
	activeAfter, err := GetActiveProgram(context.Background(), db, athlete.ID)
	if err != nil {
		t.Fatalf("GetActiveProgram after import: %v", err)
	}
	if activeAfter == nil {
		t.Fatal("expected active program after import, got nil")
		return
	}
	if activeAfter.TemplateName != "New Program" {
		t.Errorf("active program name: got %q, want %q", activeAfter.TemplateName, "New Program")
	}

	// Verify old program is deactivated.
	allPrograms, err := ListAthletePrograms(context.Background(), db, athlete.ID)
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

	result, err := ExecuteCatalogImport(context.Background(), db, ms, nil, false)
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
