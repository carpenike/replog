package models

import (
	"context"
	"database/sql"
	"testing"
)

func TestCreateEquipment(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		e, err := CreateEquipment(context.Background(), db, "Barbell", "Standard 45lb barbell")
		if err != nil {
			t.Fatalf("create equipment: %v", err)
		}
		if e.Name != "Barbell" {
			t.Errorf("name = %q, want Barbell", e.Name)
		}
		if !e.Description.Valid || e.Description.String != "Standard 45lb barbell" {
			t.Errorf("description = %v, want Standard 45lb barbell", e.Description)
		}
	})

	t.Run("no description", func(t *testing.T) {
		e, err := CreateEquipment(context.Background(), db, "Dumbbells", "")
		if err != nil {
			t.Fatalf("create equipment: %v", err)
		}
		if e.Description.Valid {
			t.Errorf("description should be null, got %q", e.Description.String)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := CreateEquipment(context.Background(), db, "Barbell", "")
		if err != ErrDuplicateEquipmentName {
			t.Errorf("err = %v, want ErrDuplicateEquipmentName", err)
		}
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		_, err := CreateEquipment(context.Background(), db, "barbell", "")
		if err != ErrDuplicateEquipmentName {
			t.Errorf("err = %v, want ErrDuplicateEquipmentName", err)
		}
	})
}

func TestGetEquipmentByID(t *testing.T) {
	db := testDB(t)

	e, _ := CreateEquipment(context.Background(), db, "Squat Rack", "Full rack with safeties")

	t.Run("found", func(t *testing.T) {
		got, err := GetEquipmentByID(context.Background(), db, e.ID)
		if err != nil {
			t.Fatalf("get equipment: %v", err)
		}
		if got.Name != "Squat Rack" {
			t.Errorf("name = %q, want Squat Rack", got.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetEquipmentByID(context.Background(), db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestUpdateEquipment(t *testing.T) {
	db := testDB(t)

	e, _ := CreateEquipment(context.Background(), db, "Bench", "Flat bench")

	t.Run("basic update", func(t *testing.T) {
		updated, err := UpdateEquipment(context.Background(), db, e.ID, "Flat Bench", "Adjustable flat bench")
		if err != nil {
			t.Fatalf("update equipment: %v", err)
		}
		if updated.Name != "Flat Bench" {
			t.Errorf("name = %q, want Flat Bench", updated.Name)
		}
		if !updated.Description.Valid || updated.Description.String != "Adjustable flat bench" {
			t.Errorf("description = %v, want Adjustable flat bench", updated.Description)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		CreateEquipment(context.Background(), db, "Pull-up Bar", "")
		_, err := UpdateEquipment(context.Background(), db, e.ID, "Pull-up Bar", "")
		if err != ErrDuplicateEquipmentName {
			t.Errorf("err = %v, want ErrDuplicateEquipmentName", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := UpdateEquipment(context.Background(), db, 99999, "Whatever", "")
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteEquipment(t *testing.T) {
	db := testDB(t)

	t.Run("delete existing", func(t *testing.T) {
		e, _ := CreateEquipment(context.Background(), db, "Kettlebell", "")
		if err := DeleteEquipment(context.Background(), db, e.ID); err != nil {
			t.Fatalf("delete equipment: %v", err)
		}
		_, err := GetEquipmentByID(context.Background(), db, e.ID)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound after delete", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := DeleteEquipment(context.Background(), db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestListEquipment(t *testing.T) {
	db := testDB(t)

	CreateEquipment(context.Background(), db, "Barbell", "")
	CreateEquipment(context.Background(), db, "Dumbbells", "")
	CreateEquipment(context.Background(), db, "Squat Rack", "")

	items, err := ListEquipment(context.Background(), db)
	if err != nil {
		t.Fatalf("list equipment: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("count = %d, want 3", len(items))
	}
	// Should be sorted by name.
	if items[0].Name != "Barbell" {
		t.Errorf("first item = %q, want Barbell", items[0].Name)
	}
}

func TestExerciseEquipment(t *testing.T) {
	db := testDB(t)

	exercise, _ := CreateExercise(context.Background(), db, "Bench Press", "", "", "", 0)
	barbell, _ := CreateEquipment(context.Background(), db, "Barbell", "")
	bench, _ := CreateEquipment(context.Background(), db, "Flat Bench", "")

	t.Run("add required", func(t *testing.T) {
		if err := AddExerciseEquipment(context.Background(), db, exercise.ID, barbell.ID, false); err != nil {
			t.Fatalf("add exercise equipment: %v", err)
		}
	})

	t.Run("add optional", func(t *testing.T) {
		if err := AddExerciseEquipment(context.Background(), db, exercise.ID, bench.ID, true); err != nil {
			t.Fatalf("add exercise equipment: %v", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		items, err := ListExerciseEquipment(context.Background(), db, exercise.ID)
		if err != nil {
			t.Fatalf("list exercise equipment: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("count = %d, want 2", len(items))
		}
		// Required items come first (sorted by optional, then name).
		if items[0].EquipmentName != "Barbell" || items[0].Optional {
			t.Errorf("first item = %q (optional=%v), want Barbell (optional=false)", items[0].EquipmentName, items[0].Optional)
		}
		if items[1].EquipmentName != "Flat Bench" || !items[1].Optional {
			t.Errorf("second item = %q (optional=%v), want Flat Bench (optional=true)", items[1].EquipmentName, items[1].Optional)
		}
	})

	t.Run("upsert changes optional flag", func(t *testing.T) {
		// Change bench from optional to required.
		if err := AddExerciseEquipment(context.Background(), db, exercise.ID, bench.ID, false); err != nil {
			t.Fatalf("upsert exercise equipment: %v", err)
		}
		items, _ := ListExerciseEquipment(context.Background(), db, exercise.ID)
		for _, item := range items {
			if item.EquipmentID == bench.ID && item.Optional {
				t.Error("bench should now be required, not optional")
			}
		}
	})

	t.Run("remove", func(t *testing.T) {
		if err := RemoveExerciseEquipment(context.Background(), db, exercise.ID, bench.ID); err != nil {
			t.Fatalf("remove exercise equipment: %v", err)
		}
		items, _ := ListExerciseEquipment(context.Background(), db, exercise.ID)
		if len(items) != 1 {
			t.Errorf("count = %d, want 1 after remove", len(items))
		}
	})

	t.Run("remove not found", func(t *testing.T) {
		err := RemoveExerciseEquipment(context.Background(), db, exercise.ID, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestAthleteEquipment(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(context.Background(), db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	barbell, _ := CreateEquipment(context.Background(), db, "Barbell", "")
	rack, _ := CreateEquipment(context.Background(), db, "Squat Rack", "")

	t.Run("add equipment", func(t *testing.T) {
		if err := AddAthleteEquipment(context.Background(), db, athlete.ID, barbell.ID); err != nil {
			t.Fatalf("add athlete equipment: %v", err)
		}
		if err := AddAthleteEquipment(context.Background(), db, athlete.ID, rack.ID); err != nil {
			t.Fatalf("add athlete equipment: %v", err)
		}
	})

	t.Run("add duplicate ignored", func(t *testing.T) {
		// INSERT OR IGNORE should not error.
		if err := AddAthleteEquipment(context.Background(), db, athlete.ID, barbell.ID); err != nil {
			t.Fatalf("add duplicate athlete equipment: %v", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		items, err := ListAthleteEquipment(context.Background(), db, athlete.ID)
		if err != nil {
			t.Fatalf("list athlete equipment: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("count = %d, want 2", len(items))
		}
	})

	t.Run("athlete equipment ids", func(t *testing.T) {
		ids, err := AthleteEquipmentIDs(context.Background(), db, athlete.ID)
		if err != nil {
			t.Fatalf("athlete equipment ids: %v", err)
		}
		if !ids[barbell.ID] {
			t.Error("barbell should be in athlete's equipment")
		}
		if !ids[rack.ID] {
			t.Error("rack should be in athlete's equipment")
		}
	})

	t.Run("remove", func(t *testing.T) {
		if err := RemoveAthleteEquipment(context.Background(), db, athlete.ID, rack.ID); err != nil {
			t.Fatalf("remove athlete equipment: %v", err)
		}
		items, _ := ListAthleteEquipment(context.Background(), db, athlete.ID)
		if len(items) != 1 {
			t.Errorf("count = %d, want 1 after remove", len(items))
		}
	})

	t.Run("remove not found", func(t *testing.T) {
		err := RemoveAthleteEquipment(context.Background(), db, athlete.ID, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestCheckExerciseCompatibility(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(context.Background(), db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	benchPress, _ := CreateExercise(context.Background(), db, "Bench Press", "", "", "", 0)
	barbell, _ := CreateEquipment(context.Background(), db, "Barbell", "")
	bench, _ := CreateEquipment(context.Background(), db, "Flat Bench", "")
	bands, _ := CreateEquipment(context.Background(), db, "Resistance Bands", "")

	// Set up exercise requirements: barbell required, bench required, bands optional.
	AddExerciseEquipment(context.Background(), db, benchPress.ID, barbell.ID, false)
	AddExerciseEquipment(context.Background(), db, benchPress.ID, bench.ID, false)
	AddExerciseEquipment(context.Background(), db, benchPress.ID, bands.ID, true)

	t.Run("athlete has no equipment", func(t *testing.T) {
		compat, err := CheckExerciseCompatibility(context.Background(), db, athlete.ID, benchPress.ID)
		if err != nil {
			t.Fatalf("check compatibility: %v", err)
		}
		if compat.HasRequired {
			t.Error("should not have required equipment")
		}
		if len(compat.Missing) != 2 {
			t.Errorf("missing = %d, want 2", len(compat.Missing))
		}
		if len(compat.Optional) != 1 {
			t.Errorf("optional = %d, want 1", len(compat.Optional))
		}
	})

	t.Run("athlete has partial equipment", func(t *testing.T) {
		AddAthleteEquipment(context.Background(), db, athlete.ID, barbell.ID)
		compat, err := CheckExerciseCompatibility(context.Background(), db, athlete.ID, benchPress.ID)
		if err != nil {
			t.Fatalf("check compatibility: %v", err)
		}
		if compat.HasRequired {
			t.Error("should not have all required equipment")
		}
		if len(compat.Missing) != 1 {
			t.Errorf("missing = %d, want 1", len(compat.Missing))
		}
		if len(compat.Available) != 1 {
			t.Errorf("available = %d, want 1", len(compat.Available))
		}
	})

	t.Run("athlete has all required equipment", func(t *testing.T) {
		AddAthleteEquipment(context.Background(), db, athlete.ID, bench.ID)
		compat, err := CheckExerciseCompatibility(context.Background(), db, athlete.ID, benchPress.ID)
		if err != nil {
			t.Fatalf("check compatibility: %v", err)
		}
		if !compat.HasRequired {
			t.Error("should have all required equipment")
		}
		if len(compat.Missing) != 0 {
			t.Errorf("missing = %d, want 0", len(compat.Missing))
		}
	})

	t.Run("exercise with no requirements", func(t *testing.T) {
		pushUps, _ := CreateExercise(context.Background(), db, "Push-ups", "", "", "", 0)
		compat, err := CheckExerciseCompatibility(context.Background(), db, athlete.ID, pushUps.ID)
		if err != nil {
			t.Fatalf("check compatibility: %v", err)
		}
		if !compat.HasRequired {
			t.Error("should be compatible with no requirements")
		}
	})
}

func TestCheckAthleteExerciseCompatibility(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(context.Background(), db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	benchPress, _ := CreateExercise(context.Background(), db, "Bench Press", "", "", "", 0)
	pushUps, _ := CreateExercise(context.Background(), db, "Push-ups", "", "", "", 0)

	barbell, _ := CreateEquipment(context.Background(), db, "Barbell", "")
	bench, _ := CreateEquipment(context.Background(), db, "Flat Bench", "")

	// Assign both exercises.
	AssignExercise(context.Background(), db, athlete.ID, benchPress.ID, 0)
	AssignExercise(context.Background(), db, athlete.ID, pushUps.ID, 0)

	// Set up bench press requirements.
	AddExerciseEquipment(context.Background(), db, benchPress.ID, barbell.ID, false)
	AddExerciseEquipment(context.Background(), db, benchPress.ID, bench.ID, false)

	t.Run("no equipment", func(t *testing.T) {
		results, err := CheckAthleteExerciseCompatibility(context.Background(), db, athlete.ID)
		if err != nil {
			t.Fatalf("check compatibility: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("results = %d, want 2", len(results))
		}

		// Find bench press result.
		var benchResult *EquipmentCompatibility
		var pushResult *EquipmentCompatibility
		for i := range results {
			if results[i].ExerciseID == benchPress.ID {
				benchResult = &results[i]
			}
			if results[i].ExerciseID == pushUps.ID {
				pushResult = &results[i]
			}
		}

		if benchResult == nil {
			t.Fatal("bench press not in results")
		}
		if benchResult.HasRequired {
			t.Error("bench press should not be compatible without equipment")
		}

		if pushResult == nil {
			t.Fatal("push-ups not in results")
		}
		if !pushResult.HasRequired {
			t.Error("push-ups should be compatible (no requirements)")
		}
	})

	t.Run("with all equipment", func(t *testing.T) {
		AddAthleteEquipment(context.Background(), db, athlete.ID, barbell.ID)
		AddAthleteEquipment(context.Background(), db, athlete.ID, bench.ID)

		results, err := CheckAthleteExerciseCompatibility(context.Background(), db, athlete.ID)
		if err != nil {
			t.Fatalf("check compatibility: %v", err)
		}

		for _, r := range results {
			if !r.HasRequired {
				t.Errorf("exercise %q should be compatible", r.ExerciseName)
			}
		}
	})
}

func TestEquipmentCascadeOnAthleteDelete(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(context.Background(), db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	eq, _ := CreateEquipment(context.Background(), db, "Barbell", "")
	AddAthleteEquipment(context.Background(), db, athlete.ID, eq.ID)

	// Delete athlete — cascade should remove athlete_equipment.
	DeleteAthlete(context.Background(), db, athlete.ID)

	items, err := ListAthleteEquipment(context.Background(), db, athlete.ID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("athlete equipment count = %d, want 0 after cascade", len(items))
	}
}

func TestCheckProgramCompatibility(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(context.Background(), db, "Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	barbell, _ := CreateEquipment(context.Background(), db, "Barbell", "")
	rack, _ := CreateEquipment(context.Background(), db, "Squat Rack", "")
	bench, _ := CreateEquipment(context.Background(), db, "Flat Bench", "")

	squat, _ := CreateExercise(context.Background(), db, "Squat", "", "", "", 0)
	AddExerciseEquipment(context.Background(), db, squat.ID, barbell.ID, false)
	AddExerciseEquipment(context.Background(), db, squat.ID, rack.ID, false)

	benchPress, _ := CreateExercise(context.Background(), db, "Bench Press", "", "", "", 0)
	AddExerciseEquipment(context.Background(), db, benchPress.ID, barbell.ID, false)
	AddExerciseEquipment(context.Background(), db, benchPress.ID, bench.ID, false)

	pushUps, _ := CreateExercise(context.Background(), db, "Push-ups", "", "", "", 0)

	tmpl, _ := CreateProgramTemplate(context.Background(), db, nil, "Test Program", "", 4, 3, false, "")
	reps := 5
	pct := 80.0
	CreatePrescribedSet(context.Background(), db, tmpl.ID, squat.ID, 1, 1, 1, &reps, &pct, nil, 0, "reps", "")
	CreatePrescribedSet(context.Background(), db, tmpl.ID, benchPress.ID, 1, 2, 1, &reps, &pct, nil, 0, "reps", "")
	CreatePrescribedSet(context.Background(), db, tmpl.ID, pushUps.ID, 1, 3, 1, &reps, nil, nil, 0, "reps", "")

	t.Run("no equipment — partial readiness", func(t *testing.T) {
		result, err := CheckProgramCompatibility(context.Background(), db, athlete.ID, tmpl.ID)
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if result.Ready {
			t.Error("should not be ready with no equipment")
		}
		if result.TotalCount != 3 {
			t.Errorf("total = %d, want 3", result.TotalCount)
		}
		// Push-ups has no requirements, so should be ready.
		if result.ReadyCount != 1 {
			t.Errorf("ready = %d, want 1 (push-ups only)", result.ReadyCount)
		}
	})

	t.Run("partial equipment", func(t *testing.T) {
		AddAthleteEquipment(context.Background(), db, athlete.ID, barbell.ID)

		result, err := CheckProgramCompatibility(context.Background(), db, athlete.ID, tmpl.ID)
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if result.Ready {
			t.Error("should not be fully ready — missing rack and bench")
		}
		// Push-ups ready, squat missing rack, bench press missing bench.
		if result.ReadyCount != 1 {
			t.Errorf("ready = %d, want 1", result.ReadyCount)
		}
	})

	t.Run("all equipment — fully ready", func(t *testing.T) {
		AddAthleteEquipment(context.Background(), db, athlete.ID, rack.ID)
		AddAthleteEquipment(context.Background(), db, athlete.ID, bench.ID)

		result, err := CheckProgramCompatibility(context.Background(), db, athlete.ID, tmpl.ID)
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if !result.Ready {
			t.Error("should be fully ready with all equipment")
		}
		if result.ReadyCount != result.TotalCount {
			t.Errorf("ready/total = %d/%d, want equal", result.ReadyCount, result.TotalCount)
		}
	})

	t.Run("empty program — ready by default", func(t *testing.T) {
		emptyTmpl, _ := CreateProgramTemplate(context.Background(), db, nil, "Empty", "", 1, 1, false, "")
		result, err := CheckProgramCompatibility(context.Background(), db, athlete.ID, emptyTmpl.ID)
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if !result.Ready {
			t.Error("empty program should be ready")
		}
		if result.TotalCount != 0 {
			t.Errorf("total = %d, want 0", result.TotalCount)
		}
	})
}

func TestEquipmentCascadeOnExerciseDelete(t *testing.T) {
	db := testDB(t)

	exercise, _ := CreateExercise(context.Background(), db, "Test Exercise", "", "", "", 0)
	eq, _ := CreateEquipment(context.Background(), db, "Barbell", "")
	AddExerciseEquipment(context.Background(), db, exercise.ID, eq.ID, false)

	// Delete exercise — cascade should remove exercise_equipment.
	DeleteExercise(context.Background(), db, exercise.ID)

	items, err := ListExerciseEquipment(context.Background(), db, exercise.ID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("exercise equipment count = %d, want 0 after cascade", len(items))
	}
}

func TestEquipmentCascadeOnEquipmentDelete(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(context.Background(), db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	exercise, _ := CreateExercise(context.Background(), db, "Test Exercise", "", "", "", 0)
	eq, _ := CreateEquipment(context.Background(), db, "Barbell", "")

	AddAthleteEquipment(context.Background(), db, athlete.ID, eq.ID)
	AddExerciseEquipment(context.Background(), db, exercise.ID, eq.ID, false)

	// Delete equipment — cascade should remove both join table entries.
	DeleteEquipment(context.Background(), db, eq.ID)

	athleteItems, _ := ListAthleteEquipment(context.Background(), db, athlete.ID)
	if len(athleteItems) != 0 {
		t.Errorf("athlete equipment count = %d, want 0 after cascade", len(athleteItems))
	}

	exerciseItems, _ := ListExerciseEquipment(context.Background(), db, exercise.ID)
	if len(exerciseItems) != 0 {
		t.Errorf("exercise equipment count = %d, want 0 after cascade", len(exerciseItems))
	}
}
