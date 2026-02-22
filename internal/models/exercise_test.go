package models

import (
	"database/sql"
	"testing"
)

func TestCreateExercise(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		e, err := CreateExercise(db, "Bench Press", "intermediate", "Control the descent", "", 0)
		if err != nil {
			t.Fatalf("create exercise: %v", err)
		}
		if e.Name != "Bench Press" {
			t.Errorf("name = %q, want Bench Press", e.Name)
		}
		if !e.Tier.Valid || e.Tier.String != "intermediate" {
			t.Errorf("tier = %v, want intermediate", e.Tier)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := CreateExercise(db, "Bench Press", "", "", "", 0)
		if err != ErrDuplicateExerciseName {
			t.Errorf("err = %v, want ErrDuplicateExerciseName", err)
		}
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		_, err := CreateExercise(db, "bench press", "", "", "", 0)
		if err != ErrDuplicateExerciseName {
			t.Errorf("err = %v, want ErrDuplicateExerciseName", err)
		}
	})
}

func TestDeleteExercise(t *testing.T) {
	db := testDB(t)

	e, _ := CreateExercise(db, "Squats", "", "", "", 0)

	t.Run("delete unreferenced", func(t *testing.T) {
		if err := DeleteExercise(db, e.ID); err != nil {
			t.Fatalf("delete exercise: %v", err)
		}
	})

	t.Run("delete referenced (RESTRICT)", func(t *testing.T) {
		e2, _ := CreateExercise(db, "Deadlift", "", "", "", 0)
		a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
		w, _ := CreateWorkout(db, a.ID, "2026-01-01", "")
		_, err := AddSet(db, w.ID, e2.ID, 5, 225, 0, "", "", "")
		if err != nil {
			t.Fatalf("add set: %v", err)
		}

		err = DeleteExercise(db, e2.ID)
		if err != ErrExerciseInUse {
			t.Errorf("err = %v, want ErrExerciseInUse", err)
		}
	})
}

func TestListExercises(t *testing.T) {
	db := testDB(t)

	CreateExercise(db, "Push-ups", "foundational", "", "", 0)
	CreateExercise(db, "Back Squat", "", "", "", 0)
	CreateExercise(db, "Cleans", "sport_performance", "", "", 0)

	t.Run("all", func(t *testing.T) {
		exercises, err := ListExercises(db, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(exercises) != 3 {
			t.Errorf("count = %d, want 3", len(exercises))
		}
	})

	t.Run("filter by tier", func(t *testing.T) {
		exercises, err := ListExercises(db, "foundational")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(exercises) != 1 {
			t.Errorf("count = %d, want 1", len(exercises))
		}
	})

	t.Run("filter no tier", func(t *testing.T) {
		exercises, err := ListExercises(db, "none")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(exercises) != 1 {
			t.Errorf("count = %d, want 1", len(exercises))
		}
		if exercises[0].Name != "Back Squat" {
			t.Errorf("name = %q, want Back Squat", exercises[0].Name)
		}
	})
}

func TestEffectiveRestSeconds(t *testing.T) {
	t.Run("custom rest", func(t *testing.T) {
		e := &Exercise{RestSeconds: sql.NullInt64{Int64: 120, Valid: true}}
		if got := e.EffectiveRestSeconds(); got != 120 {
			t.Errorf("got %d, want 120", got)
		}
	})

	t.Run("default rest", func(t *testing.T) {
		e := &Exercise{}
		if got := e.EffectiveRestSeconds(); got != DefaultRestSeconds {
			t.Errorf("got %d, want %d", got, DefaultRestSeconds)
		}
	})
}

func TestUpdateExercise(t *testing.T) {
	db := testDB(t)

	e, _ := CreateExercise(db, "Original Name", "foundational", "old notes", "", 0)

	t.Run("basic update", func(t *testing.T) {
		updated, err := UpdateExercise(db, e.ID, "New Name", "intermediate", "new notes", "https://demo.url", 120)
		if err != nil {
			t.Fatalf("update exercise: %v", err)
		}
		if updated.Name != "New Name" {
			t.Errorf("name = %q, want New Name", updated.Name)
		}
		if !updated.Tier.Valid || updated.Tier.String != "intermediate" {
			t.Errorf("tier = %v, want intermediate", updated.Tier)
		}
		if !updated.RestSeconds.Valid || updated.RestSeconds.Int64 != 120 {
			t.Errorf("rest_seconds = %v, want 120", updated.RestSeconds)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		CreateExercise(db, "Taken Name", "", "", "", 0)
		_, err := UpdateExercise(db, e.ID, "Taken Name", "", "", "", 0)
		if err != ErrDuplicateExerciseName {
			t.Errorf("err = %v, want ErrDuplicateExerciseName", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := UpdateExercise(db, 99999, "Whatever", "", "", "", 0)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestFeaturedExercise(t *testing.T) {
	db := testDB(t)

	t.Run("create with featured flag", func(t *testing.T) {
		e, err := CreateExercise(db, "Featured Squat", "", "", "", 0, true)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if !e.Featured {
			t.Error("expected Featured = true")
		}
	})

	t.Run("create defaults to not featured", func(t *testing.T) {
		e, err := CreateExercise(db, "Ordinary Exercise", "", "", "", 0)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if e.Featured {
			t.Error("expected Featured = false by default")
		}
	})

	t.Run("update featured flag", func(t *testing.T) {
		e, _ := CreateExercise(db, "Toggle Featured", "", "", "", 0)
		updated, err := UpdateExercise(db, e.ID, e.Name, "", "", "", 0, true)
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if !updated.Featured {
			t.Error("expected Featured = true after update")
		}

		unfeatured, err := UpdateExercise(db, e.ID, e.Name, "", "", "", 0, false)
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if unfeatured.Featured {
			t.Error("expected Featured = false after unsetting")
		}
	})
}

func TestListFeaturedLifts(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Feat Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	squat, _ := CreateExercise(db, "F Squat", "", "", "", 0, true)
	bench, _ := CreateExercise(db, "F Bench", "", "", "", 0, true)
	CreateExercise(db, "F Curl", "", "", "", 0) // not featured

	t.Run("no data returns nil", func(t *testing.T) {
		lifts, err := ListFeaturedLifts(db, athlete.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(lifts) != 0 {
			t.Errorf("expected 0 lifts with no data, got %d", len(lifts))
		}
	})

	t.Run("with training max only", func(t *testing.T) {
		SetTrainingMax(db, athlete.ID, squat.ID, 315, "2026-01-01", "")

		lifts, err := ListFeaturedLifts(db, athlete.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(lifts) != 1 {
			t.Fatalf("expected 1 lift, got %d", len(lifts))
		}
		if lifts[0].ExerciseName != "F Squat" {
			t.Errorf("name = %q, want F Squat", lifts[0].ExerciseName)
		}
		if lifts[0].CurrentTM == nil {
			t.Fatal("expected CurrentTM to be set")
		}
		if lifts[0].CurrentTM.Weight != 315 {
			t.Errorf("TM weight = %v, want 315", lifts[0].CurrentTM.Weight)
		}
	})

	t.Run("with logged sets", func(t *testing.T) {
		w, _ := CreateWorkout(db, athlete.ID, "2026-01-15", "")
		AddSet(db, w.ID, bench.ID, 5, 225, 0, "", "", "")
		AddSet(db, w.ID, bench.ID, 3, 245, 0, "", "", "")

		lifts, err := ListFeaturedLifts(db, athlete.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}

		// Should now have 2 lifts: squat (TM only) and bench (sets only).
		if len(lifts) != 2 {
			t.Fatalf("expected 2 lifts, got %d", len(lifts))
		}

		// Find bench in the results.
		var benchLift *FeaturedLift
		for _, l := range lifts {
			if l.ExerciseID == bench.ID {
				benchLift = l
			}
		}
		if benchLift == nil {
			t.Fatal("expected bench lift in results")
		}

		if !benchLift.BestWeight.Valid || benchLift.BestWeight.Float64 != 245 {
			t.Errorf("best weight = %v, want 245", benchLift.BestWeight)
		}
		if benchLift.BestReps != 3 {
			t.Errorf("best reps = %d, want 3", benchLift.BestReps)
		}

		// Epley: 245 × (1 + 3/30) = 245 × 1.1 = 269.5
		expected1RM := 245.0 * (1 + 3.0/30.0)
		if benchLift.Estimated1RM != expected1RM {
			t.Errorf("est 1RM = %v, want %v", benchLift.Estimated1RM, expected1RM)
		}
	})

	t.Run("single rep set equals weight", func(t *testing.T) {
		athlete2, _ := CreateAthlete(db, "Single Rep Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
		w, _ := CreateWorkout(db, athlete2.ID, "2026-02-01", "")
		AddSet(db, w.ID, squat.ID, 1, 405, 0, "", "", "")

		lifts, err := ListFeaturedLifts(db, athlete2.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(lifts) != 1 {
			t.Fatalf("expected 1 lift, got %d", len(lifts))
		}
		if lifts[0].Estimated1RM != 405 {
			t.Errorf("est 1RM for 1 rep = %v, want 405", lifts[0].Estimated1RM)
		}
	})
}
