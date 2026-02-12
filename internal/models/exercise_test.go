package models

import (
	"testing"
)

func TestCreateExercise(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		e, err := CreateExercise(db, "Bench Press", "intermediate", 15, "Control the descent")
		if err != nil {
			t.Fatalf("create exercise: %v", err)
		}
		if e.Name != "Bench Press" {
			t.Errorf("name = %q, want Bench Press", e.Name)
		}
		if !e.Tier.Valid || e.Tier.String != "intermediate" {
			t.Errorf("tier = %v, want intermediate", e.Tier)
		}
		if !e.TargetReps.Valid || e.TargetReps.Int64 != 15 {
			t.Errorf("target_reps = %v, want 15", e.TargetReps)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := CreateExercise(db, "Bench Press", "", 0, "")
		if err != ErrDuplicateExerciseName {
			t.Errorf("err = %v, want ErrDuplicateExerciseName", err)
		}
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		_, err := CreateExercise(db, "bench press", "", 0, "")
		if err != ErrDuplicateExerciseName {
			t.Errorf("err = %v, want ErrDuplicateExerciseName", err)
		}
	})
}

func TestDeleteExercise(t *testing.T) {
	db := testDB(t)

	e, _ := CreateExercise(db, "Squats", "", 0, "")

	t.Run("delete unreferenced", func(t *testing.T) {
		if err := DeleteExercise(db, e.ID); err != nil {
			t.Fatalf("delete exercise: %v", err)
		}
	})

	t.Run("delete referenced (RESTRICT)", func(t *testing.T) {
		e2, _ := CreateExercise(db, "Deadlift", "", 0, "")
		a, _ := CreateAthlete(db, "Test Athlete", "", "")
		w, _ := CreateWorkout(db, a.ID, "2026-01-01", "")
		_, err := AddSet(db, w.ID, e2.ID, 5, 225, "")
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

	CreateExercise(db, "Push-ups", "foundational", 20, "")
	CreateExercise(db, "Back Squat", "", 0, "")
	CreateExercise(db, "Cleans", "sport_performance", 0, "")

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
