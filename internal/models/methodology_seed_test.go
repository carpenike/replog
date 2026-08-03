package models

import (
	"bytes"
	"context"
	"testing"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/importers"
)

func TestMethodologySeed_FullSeedFile(t *testing.T) {
	db := testDB(t)
	// Seed the catalog first — methodology links resolve by name.
	dataCat := database.SeedCatalog()
	parsedCat, err := importers.ParseCatalogJSON(bytes.NewReader(dataCat))
	if err != nil {
		t.Fatalf("parse catalog: %v", err)
	}
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsedCat.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsedCat.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsedCat.Programs, nil),
		Parsed:    parsedCat,
	}
	if _, err := ExecuteCatalogImport(context.Background(), db, ms, nil, false); err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}

	// Now apply the methodology seed.
	result, err := ApplyMethodologySeedFromBytes(context.Background(), db, database.SeedMethodologies())
	if err != nil {
		t.Fatalf("ApplyMethodologySeed: %v", err)
	}

	// Every methodology in the seed should have been created on the first run.
	if result.MethodologiesCreated == 0 {
		t.Fatal("no methodologies created")
	}
	if result.MethodologiesSkipped != 0 {
		t.Errorf("MethodologiesSkipped = %d on first run, want 0", result.MethodologiesSkipped)
	}

	// Loud signal if the seed file references rows that no longer exist
	// in the catalog — would mean the seed has drifted.
	if len(result.MissingProgramRefs) > 0 {
		t.Errorf("seed references unknown program templates: %v", result.MissingProgramRefs)
	}
	if len(result.MissingEquipment) > 0 {
		t.Errorf("seed references unknown equipment items: %v", result.MissingEquipment)
	}
	if len(result.MissingExercises) > 0 {
		t.Errorf("seed references unknown exercises: %v", result.MissingExercises)
	}

	// Spot-check the well-known methodologies and their link contents.
	cases := []struct {
		key         string
		minRefs     int
		minPatterns int
		minEquip    int
	}{
		{"yessis-1x20", 1, 4, 5},
		{"yessis-1x15", 1, 4, 5},
		{"yessis-sport-performance", 3, 5, 5},
		{"int-youth-gpp", 0, 4, 5},
		{"531", 2, 3, 5},
		{"531-bbb", 1, 3, 5},
		{"greyskull-lp", 1, 3, 5},
		{"gzclp", 1, 3, 5},
		{"5x5", 0, 3, 3},
		{"galpin-3-to-5", 0, 5, 8},
		{"sarge-circuit", 3, 5, 8},
	}
	for _, tc := range cases {
		m, err := GetMethodologyByKey(context.Background(), db, tc.key)
		if err != nil {
			t.Errorf("get %q: %v", tc.key, err)
			continue
		}
		links, err := LoadMethodologyWithLinks(context.Background(), db, m.ID)
		if err != nil {
			t.Errorf("load with links %q: %v", tc.key, err)
			continue
		}
		if len(links.ReferenceProgramIDs) < tc.minRefs {
			t.Errorf("%s: reference programs = %d, want >= %d", tc.key, len(links.ReferenceProgramIDs), tc.minRefs)
		}
		if len(links.AllowedPatterns) < tc.minPatterns {
			t.Errorf("%s: allowed patterns = %d, want >= %d (%v)", tc.key, len(links.AllowedPatterns), tc.minPatterns, links.AllowedPatterns)
		}
		if len(links.AllowedEquipmentIDs) < tc.minEquip {
			t.Errorf("%s: allowed equipment = %d, want >= %d", tc.key, len(links.AllowedEquipmentIDs), tc.minEquip)
		}
		if m.Definition == "" {
			t.Errorf("%s: definition is empty", tc.key)
		}
	}

	// The youth Yessis methodologies should be tagged audience=youth.
	for _, k := range []string{"yessis-1x20", "yessis-1x15", "yessis-sport-performance", "int-youth-gpp"} {
		m, _ := GetMethodologyByKey(context.Background(), db, k)
		if !m.Audience.Valid || m.Audience.String != MethodologyAudienceYouth {
			t.Errorf("%s: Audience = %v, want youth", k, m.Audience)
		}
	}

	// And the adult ones audience=adult.
	for _, k := range []string{"531", "531-bbb", "greyskull-lp", "gzclp", "5x5", "galpin-3-to-5", "sarge-circuit"} {
		m, _ := GetMethodologyByKey(context.Background(), db, k)
		if !m.Audience.Valid || m.Audience.String != MethodologyAudienceAdult {
			t.Errorf("%s: Audience = %v, want adult", k, m.Audience)
		}
	}
}

func TestMethodologySeed_Idempotent(t *testing.T) {
	db := testDB(t)
	dataCat := database.SeedCatalog()
	parsedCat, _ := importers.ParseCatalogJSON(bytes.NewReader(dataCat))
	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsedCat.Exercises, nil),
		Equipment: importers.BuildEquipmentMappings(parsedCat.Equipment, nil),
		Programs:  importers.BuildProgramMappings(parsedCat.Programs, nil),
		Parsed:    parsedCat,
	}
	if _, err := ExecuteCatalogImport(context.Background(), db, ms, nil, false); err != nil {
		t.Fatalf("ExecuteCatalogImport: %v", err)
	}

	first, err := ApplyMethodologySeedFromBytes(context.Background(), db, database.SeedMethodologies())
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if first.MethodologiesCreated == 0 {
		t.Fatal("first apply created 0 methodologies")
	}

	second, err := ApplyMethodologySeedFromBytes(context.Background(), db, database.SeedMethodologies())
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if second.MethodologiesCreated != 0 {
		t.Errorf("second apply MethodologiesCreated = %d, want 0", second.MethodologiesCreated)
	}
	if second.MethodologiesSkipped != first.MethodologiesCreated {
		t.Errorf("second apply MethodologiesSkipped = %d, want %d", second.MethodologiesSkipped, first.MethodologiesCreated)
	}
}

func TestMethodologySeed_RejectsInvalidPattern(t *testing.T) {
	db := testDB(t)
	bad := []byte(`{
		"version": "1.0",
		"type": "methodologies",
		"methodologies": [
			{
				"key": "bad",
				"name": "Bad",
				"definition": "x",
				"allowed_patterns": ["push", "yoga"]
			}
		]
	}`)
	if _, err := ApplyMethodologySeedFromBytes(context.Background(), db, bad); err == nil {
		t.Fatal("expected error for invalid pattern")
	}
	// And that no row was inserted (transaction rolled back).
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM methodologies WHERE key = 'bad'`).Scan(&n)
	if n != 0 {
		t.Errorf("row count after rejected seed = %d, want 0 (rollback)", n)
	}
}

func TestMethodologySeed_RejectsBadType(t *testing.T) {
	db := testDB(t)
	bad := []byte(`{"version":"1.0","type":"catalog","methodologies":[]}`)
	if _, err := ApplyMethodologySeedFromBytes(context.Background(), db, bad); err == nil {
		t.Fatal("expected error for wrong top-level type")
	}
}

// TestBackfillExerciseMovementPatterns simulates the upgrade path: a DB whose
// exercises were seeded BEFORE the catalog gained movement_patterns. The
// backfill should tag them on the next startup; subsequent backfills are
// no-ops.
func TestBackfillExerciseMovementPatterns(t *testing.T) {
	db := testDB(t)

	// Seed exercises directly (bypassing the catalog importer that would
	// have written the tags inline). Simulates an older install.
	for _, name := range []string{"Squat", "Bench Press", "Plank", "Track Sprint"} {
		if _, err := CreateExercise(context.Background(), db, name, "", "", "", 0); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}

	result, err := BackfillExerciseMovementPatterns(context.Background(), db, database.SeedCatalog())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if result.ExercisesTagged != 4 {
		t.Errorf("ExercisesTagged = %d, want 4", result.ExercisesTagged)
	}
	if result.PatternsInserted < 4 {
		t.Errorf("PatternsInserted = %d, want >= 4", result.PatternsInserted)
	}

	// Confirm the tags landed — including the conditioning tag added for #33.
	for _, tc := range []struct {
		name string
		want string
	}{{"Squat", "squat"}, {"Bench Press", "push"}, {"Plank", "ground"}, {"Track Sprint", "conditioning"}} {
		ex, _ := getExerciseByName(context.Background(), db, tc.name)
		got, _ := ListExerciseMovementPatterns(context.Background(), db, ex.ID)
		if len(got) == 0 || got[0] != tc.want {
			t.Errorf("%s tags = %v, want %q first", tc.name, got, tc.want)
		}
	}

	// Second run is a no-op (already-tagged short-circuit).
	result2, err := BackfillExerciseMovementPatterns(context.Background(), db, database.SeedCatalog())
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if result2.ExercisesTagged != 0 {
		t.Errorf("second backfill tagged = %d, want 0 (should skip already-tagged)", result2.ExercisesTagged)
	}
	if result2.SkippedAlreadyTagged != 4 {
		t.Errorf("second backfill skipped = %d, want 4", result2.SkippedAlreadyTagged)
	}

	// Manual edits are preserved: tag an exercise differently, re-run, no change.
	plank, _ := getExerciseByName(context.Background(), db, "Plank")
	if err := SetExerciseMovementPatterns(context.Background(), db, plank.ID, []string{"carry"}); err != nil {
		t.Fatalf("override: %v", err)
	}
	if _, err := BackfillExerciseMovementPatterns(context.Background(), db, database.SeedCatalog()); err != nil {
		t.Fatalf("third backfill: %v", err)
	}
	got, _ := ListExerciseMovementPatterns(context.Background(), db, plank.ID)
	if len(got) != 1 || got[0] != "carry" {
		t.Errorf("manual edit lost: Plank tags = %v, want [carry]", got)
	}
}
