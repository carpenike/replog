package models

import (
	"database/sql"
	"testing"
)

func TestCreateEquipment(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		e, err := CreateEquipment(db, "Barbell", "Standard 45lb barbell")
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
		e, err := CreateEquipment(db, "Dumbbells", "")
		if err != nil {
			t.Fatalf("create equipment: %v", err)
		}
		if e.Description.Valid {
			t.Errorf("description should be null, got %q", e.Description.String)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := CreateEquipment(db, "Barbell", "")
		if err != ErrDuplicateEquipmentName {
			t.Errorf("err = %v, want ErrDuplicateEquipmentName", err)
		}
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		_, err := CreateEquipment(db, "barbell", "")
		if err != ErrDuplicateEquipmentName {
			t.Errorf("err = %v, want ErrDuplicateEquipmentName", err)
		}
	})
}

func TestGetEquipmentByID(t *testing.T) {
	db := testDB(t)

	e, _ := CreateEquipment(db, "Squat Rack", "Full rack with safeties")

	t.Run("found", func(t *testing.T) {
		got, err := GetEquipmentByID(db, e.ID)
		if err != nil {
			t.Fatalf("get equipment: %v", err)
		}
		if got.Name != "Squat Rack" {
			t.Errorf("name = %q, want Squat Rack", got.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetEquipmentByID(db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestUpdateEquipment(t *testing.T) {
	db := testDB(t)

	e, _ := CreateEquipment(db, "Bench", "Flat bench")

	t.Run("basic update", func(t *testing.T) {
		updated, err := UpdateEquipment(db, e.ID, "Flat Bench", "Adjustable flat bench")
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
		CreateEquipment(db, "Pull-up Bar", "")
		_, err := UpdateEquipment(db, e.ID, "Pull-up Bar", "")
		if err != ErrDuplicateEquipmentName {
			t.Errorf("err = %v, want ErrDuplicateEquipmentName", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := UpdateEquipment(db, 99999, "Whatever", "")
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteEquipment(t *testing.T) {
	db := testDB(t)

	t.Run("delete existing", func(t *testing.T) {
		e, _ := CreateEquipment(db, "Kettlebell", "")
		if err := DeleteEquipment(db, e.ID); err != nil {
			t.Fatalf("delete equipment: %v", err)
		}
		_, err := GetEquipmentByID(db, e.ID)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound after delete", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := DeleteEquipment(db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestListEquipment(t *testing.T) {
	db := testDB(t)

	CreateEquipment(db, "Barbell", "")
	CreateEquipment(db, "Dumbbells", "")
	CreateEquipment(db, "Squat Rack", "")

	items, err := ListEquipment(db)
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

	exercise, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	barbell, _ := CreateEquipment(db, "Barbell", "")
	bench, _ := CreateEquipment(db, "Flat Bench", "")

	t.Run("add required", func(t *testing.T) {
		if err := AddExerciseEquipment(db, exercise.ID, barbell.ID, false); err != nil {
			t.Fatalf("add exercise equipment: %v", err)
		}
	})

	t.Run("add optional", func(t *testing.T) {
		if err := AddExerciseEquipment(db, exercise.ID, bench.ID, true); err != nil {
			t.Fatalf("add exercise equipment: %v", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		items, err := ListExerciseEquipment(db, exercise.ID)
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
		if err := AddExerciseEquipment(db, exercise.ID, bench.ID, false); err != nil {
			t.Fatalf("upsert exercise equipment: %v", err)
		}
		items, _ := ListExerciseEquipment(db, exercise.ID)
		for _, item := range items {
			if item.EquipmentID == bench.ID && item.Optional {
				t.Error("bench should now be required, not optional")
			}
		}
	})

	t.Run("remove", func(t *testing.T) {
		if err := RemoveExerciseEquipment(db, exercise.ID, bench.ID); err != nil {
			t.Fatalf("remove exercise equipment: %v", err)
		}
		items, _ := ListExerciseEquipment(db, exercise.ID)
		if len(items) != 1 {
			t.Errorf("count = %d, want 1 after remove", len(items))
		}
	})

	t.Run("remove not found", func(t *testing.T) {
		err := RemoveExerciseEquipment(db, exercise.ID, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestAthleteEquipment(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	barbell, _ := CreateEquipment(db, "Barbell", "")
	rack, _ := CreateEquipment(db, "Squat Rack", "")

	t.Run("add equipment", func(t *testing.T) {
		if err := AddAthleteEquipment(db, athlete.ID, barbell.ID); err != nil {
			t.Fatalf("add athlete equipment: %v", err)
		}
		if err := AddAthleteEquipment(db, athlete.ID, rack.ID); err != nil {
			t.Fatalf("add athlete equipment: %v", err)
		}
	})

	t.Run("add duplicate ignored", func(t *testing.T) {
		// INSERT OR IGNORE should not error.
		if err := AddAthleteEquipment(db, athlete.ID, barbell.ID); err != nil {
			t.Fatalf("add duplicate athlete equipment: %v", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		items, err := ListAthleteEquipment(db, athlete.ID)
		if err != nil {
			t.Fatalf("list athlete equipment: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("count = %d, want 2", len(items))
		}
	})

	t.Run("athlete equipment ids", func(t *testing.T) {
		ids, err := AthleteEquipmentIDs(db, athlete.ID)
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
		if err := RemoveAthleteEquipment(db, athlete.ID, rack.ID); err != nil {
			t.Fatalf("remove athlete equipment: %v", err)
		}
		items, _ := ListAthleteEquipment(db, athlete.ID)
		if len(items) != 1 {
			t.Errorf("count = %d, want 1 after remove", len(items))
		}
	})

	t.Run("remove not found", func(t *testing.T) {
		err := RemoveAthleteEquipment(db, athlete.ID, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestCheckExerciseCompatibility(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	benchPress, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	barbell, _ := CreateEquipment(db, "Barbell", "")
	bench, _ := CreateEquipment(db, "Flat Bench", "")
	bands, _ := CreateEquipment(db, "Resistance Bands", "")

	// Set up exercise requirements: barbell required, bench required, bands optional.
	AddExerciseEquipment(db, benchPress.ID, barbell.ID, false)
	AddExerciseEquipment(db, benchPress.ID, bench.ID, false)
	AddExerciseEquipment(db, benchPress.ID, bands.ID, true)

	t.Run("athlete has no equipment", func(t *testing.T) {
		compat, err := CheckExerciseCompatibility(db, athlete.ID, benchPress.ID)
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
		AddAthleteEquipment(db, athlete.ID, barbell.ID)
		compat, err := CheckExerciseCompatibility(db, athlete.ID, benchPress.ID)
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
		AddAthleteEquipment(db, athlete.ID, bench.ID)
		compat, err := CheckExerciseCompatibility(db, athlete.ID, benchPress.ID)
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
		pushUps, _ := CreateExercise(db, "Push-ups", "", "", "", 0)
		compat, err := CheckExerciseCompatibility(db, athlete.ID, pushUps.ID)
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

	athlete, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	benchPress, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	pushUps, _ := CreateExercise(db, "Push-ups", "", "", "", 0)

	barbell, _ := CreateEquipment(db, "Barbell", "")
	bench, _ := CreateEquipment(db, "Flat Bench", "")

	// Assign both exercises.
	AssignExercise(db, athlete.ID, benchPress.ID, 0)
	AssignExercise(db, athlete.ID, pushUps.ID, 0)

	// Set up bench press requirements.
	AddExerciseEquipment(db, benchPress.ID, barbell.ID, false)
	AddExerciseEquipment(db, benchPress.ID, bench.ID, false)

	t.Run("no equipment", func(t *testing.T) {
		results, err := CheckAthleteExerciseCompatibility(db, athlete.ID)
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
		AddAthleteEquipment(db, athlete.ID, barbell.ID)
		AddAthleteEquipment(db, athlete.ID, bench.ID)

		results, err := CheckAthleteExerciseCompatibility(db, athlete.ID)
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

	athlete, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	eq, _ := CreateEquipment(db, "Barbell", "")
	AddAthleteEquipment(db, athlete.ID, eq.ID)

	// Delete athlete — cascade should remove athlete_equipment.
	DeleteAthlete(db, athlete.ID)

	items, err := ListAthleteEquipment(db, athlete.ID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("athlete equipment count = %d, want 0 after cascade", len(items))
	}
}

func TestCheckProgramCompatibility(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	barbell, _ := CreateEquipment(db, "Barbell", "")
	rack, _ := CreateEquipment(db, "Squat Rack", "")
	bench, _ := CreateEquipment(db, "Flat Bench", "")

	squat, _ := CreateExercise(db, "Squat", "", "", "", 0)
	AddExerciseEquipment(db, squat.ID, barbell.ID, false)
	AddExerciseEquipment(db, squat.ID, rack.ID, false)

	benchPress, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	AddExerciseEquipment(db, benchPress.ID, barbell.ID, false)
	AddExerciseEquipment(db, benchPress.ID, bench.ID, false)

	pushUps, _ := CreateExercise(db, "Push-ups", "", "", "", 0)

	tmpl, _ := CreateProgramTemplate(db, nil, "Test Program", "", 4, 3, false, "")
	reps := 5
	pct := 80.0
	CreatePrescribedSet(db, tmpl.ID, squat.ID, 1, 1, 1, &reps, &pct, nil, 0, "reps", "")
	CreatePrescribedSet(db, tmpl.ID, benchPress.ID, 1, 2, 1, &reps, &pct, nil, 0, "reps", "")
	CreatePrescribedSet(db, tmpl.ID, pushUps.ID, 1, 3, 1, &reps, nil, nil, 0, "reps", "")

	t.Run("no equipment — partial readiness", func(t *testing.T) {
		result, err := CheckProgramCompatibility(db, athlete.ID, tmpl.ID)
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
		AddAthleteEquipment(db, athlete.ID, barbell.ID)

		result, err := CheckProgramCompatibility(db, athlete.ID, tmpl.ID)
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
		AddAthleteEquipment(db, athlete.ID, rack.ID)
		AddAthleteEquipment(db, athlete.ID, bench.ID)

		result, err := CheckProgramCompatibility(db, athlete.ID, tmpl.ID)
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
		emptyTmpl, _ := CreateProgramTemplate(db, nil, "Empty", "", 1, 1, false, "")
		result, err := CheckProgramCompatibility(db, athlete.ID, emptyTmpl.ID)
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

	exercise, _ := CreateExercise(db, "Test Exercise", "", "", "", 0)
	eq, _ := CreateEquipment(db, "Barbell", "")
	AddExerciseEquipment(db, exercise.ID, eq.ID, false)

	// Delete exercise — cascade should remove exercise_equipment.
	DeleteExercise(db, exercise.ID)

	items, err := ListExerciseEquipment(db, exercise.ID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("exercise equipment count = %d, want 0 after cascade", len(items))
	}
}

func TestEquipmentCascadeOnEquipmentDelete(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	exercise, _ := CreateExercise(db, "Test Exercise", "", "", "", 0)
	eq, _ := CreateEquipment(db, "Barbell", "")

	AddAthleteEquipment(db, athlete.ID, eq.ID)
	AddExerciseEquipment(db, exercise.ID, eq.ID, false)

	// Delete equipment — cascade should remove both join table entries.
	DeleteEquipment(db, eq.ID)

	athleteItems, _ := ListAthleteEquipment(db, athlete.ID)
	if len(athleteItems) != 0 {
		t.Errorf("athlete equipment count = %d, want 0 after cascade", len(athleteItems))
	}

	exerciseItems, _ := ListExerciseEquipment(db, exercise.ID)
	if len(exerciseItems) != 0 {
		t.Errorf("exercise equipment count = %d, want 0 after cascade", len(exerciseItems))
	}
}
