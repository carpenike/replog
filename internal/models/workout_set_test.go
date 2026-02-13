package models

import (
	"testing"
)

func TestSetCRUD(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Set Athlete", "", "")
	e, _ := CreateExercise(db, "Test Lift", "", 0, "", "", 0)
	w, _ := CreateWorkout(db, a.ID, "2026-05-01", "")

	t.Run("add sets with auto set_number", func(t *testing.T) {
		s1, err := AddSet(db, w.ID, e.ID, 5, 135, 0, "easy")
		if err != nil {
			t.Fatalf("add set 1: %v", err)
		}
		if s1.SetNumber != 1 {
			t.Errorf("set_number = %d, want 1", s1.SetNumber)
		}
		if s1.Reps != 5 {
			t.Errorf("reps = %d, want 5", s1.Reps)
		}

		s2, err := AddSet(db, w.ID, e.ID, 5, 155, 0, "")
		if err != nil {
			t.Fatalf("add set 2: %v", err)
		}
		if s2.SetNumber != 2 {
			t.Errorf("set_number = %d, want 2", s2.SetNumber)
		}
	})

	t.Run("bodyweight set (null weight)", func(t *testing.T) {
		s, err := AddSet(db, w.ID, e.ID, 20, 0, 0, "")
		if err != nil {
			t.Fatalf("add bodyweight set: %v", err)
		}
		if s.Weight.Valid {
			t.Errorf("weight should be null for bodyweight, got %f", s.Weight.Float64)
		}
	})
}

func TestUpdateSet(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Update Set Athlete", "", "")
	e, _ := CreateExercise(db, "Update Lift", "", 0, "", "", 0)
	w, _ := CreateWorkout(db, a.ID, "2026-06-01", "")
	s, _ := AddSet(db, w.ID, e.ID, 5, 100, 0, "")

	updated, err := UpdateSet(db, s.ID, 8, 110, 0, "form felt better")
	if err != nil {
		t.Fatalf("update set: %v", err)
	}
	if updated.Reps != 8 {
		t.Errorf("reps = %d, want 8", updated.Reps)
	}
	if !updated.Weight.Valid || updated.Weight.Float64 != 110 {
		t.Errorf("weight = %v, want 110", updated.Weight)
	}
	if !updated.Notes.Valid || updated.Notes.String != "form felt better" {
		t.Errorf("notes = %v, want form felt better", updated.Notes)
	}
}

func TestDeleteSet(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Del Set Athlete", "", "")
	e, _ := CreateExercise(db, "Del Lift", "", 0, "", "", 0)
	w, _ := CreateWorkout(db, a.ID, "2026-07-01", "")
	s, _ := AddSet(db, w.ID, e.ID, 5, 100, 0, "")

	if err := DeleteSet(db, s.ID); err != nil {
		t.Fatalf("delete set: %v", err)
	}
	_, err := GetSetByID(db, s.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListSetsByWorkout(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Group Athlete", "", "")
	e1, _ := CreateExercise(db, "Lift A", "", 0, "", "", 0)
	e2, _ := CreateExercise(db, "Lift B", "", 0, "", "", 0)
	w, _ := CreateWorkout(db, a.ID, "2026-08-01", "")

	AddSet(db, w.ID, e1.ID, 5, 100, 0, "")
	AddSet(db, w.ID, e1.ID, 5, 110, 0, "")
	AddSet(db, w.ID, e2.ID, 10, 50, 0, "")

	groups, err := ListSetsByWorkout(db, w.ID)
	if err != nil {
		t.Fatalf("list sets: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
}

func TestDeleteSet_Renumbers(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Renum Athlete", "", "")
	e, _ := CreateExercise(db, "Renum Lift", "", 0, "", "", 0)
	w, _ := CreateWorkout(db, a.ID, "2026-09-01", "")

	s1, _ := AddSet(db, w.ID, e.ID, 5, 100, 0, "")
	s2, _ := AddSet(db, w.ID, e.ID, 5, 110, 0, "")
	s3, _ := AddSet(db, w.ID, e.ID, 5, 120, 0, "")

	// Delete the middle set.
	if err := DeleteSet(db, s2.ID); err != nil {
		t.Fatalf("delete middle set: %v", err)
	}

	// Remaining sets should be renumbered 1, 2.
	got1, _ := GetSetByID(db, s1.ID)
	got3, _ := GetSetByID(db, s3.ID)

	if got1.SetNumber != 1 {
		t.Errorf("s1 set_number = %d, want 1", got1.SetNumber)
	}
	if got3.SetNumber != 2 {
		t.Errorf("s3 set_number = %d, want 2", got3.SetNumber)
	}
}

func TestDeleteSet_NotFound(t *testing.T) {
	db := testDB(t)

	if err := DeleteSet(db, 99999); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListExerciseHistory(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "History Athlete", "", "")
	e, _ := CreateExercise(db, "History Lift", "", 0, "", "", 0)

	t.Run("empty history", func(t *testing.T) {
		page, err := ListExerciseHistory(db, a.ID, e.ID, 0)
		if err != nil {
			t.Fatalf("list exercise history: %v", err)
		}
		if len(page.Days) != 0 {
			t.Errorf("days = %d, want 0", len(page.Days))
		}
		if page.HasMore {
			t.Error("hasMore should be false for empty")
		}
	})

	// Create some workouts with sets.
	w1, _ := CreateWorkout(db, a.ID, "2026-01-01", "")
	AddSet(db, w1.ID, e.ID, 5, 100, 0, "")
	AddSet(db, w1.ID, e.ID, 5, 110, 0, "")

	w2, _ := CreateWorkout(db, a.ID, "2026-01-02", "")
	AddSet(db, w2.ID, e.ID, 3, 130, 0, "")

	t.Run("with data", func(t *testing.T) {
		page, err := ListExerciseHistory(db, a.ID, e.ID, 0)
		if err != nil {
			t.Fatalf("list exercise history: %v", err)
		}
		if len(page.Days) != 2 {
			t.Fatalf("days = %d, want 2", len(page.Days))
		}
		// Most recent first.
		if page.Days[0].WorkoutID != w2.ID {
			t.Errorf("first day workout = %d, want %d", page.Days[0].WorkoutID, w2.ID)
		}
		if len(page.Days[0].Sets) != 1 {
			t.Errorf("first day sets = %d, want 1", len(page.Days[0].Sets))
		}
		if len(page.Days[1].Sets) != 2 {
			t.Errorf("second day sets = %d, want 2", len(page.Days[1].Sets))
		}
	})

	t.Run("different exercise not included", func(t *testing.T) {
		e2, _ := CreateExercise(db, "Other Lift", "", 0, "", "", 0)
		page, err := ListExerciseHistory(db, a.ID, e2.ID, 0)
		if err != nil {
			t.Fatalf("list exercise history: %v", err)
		}
		if len(page.Days) != 0 {
			t.Errorf("days = %d, want 0", len(page.Days))
		}
	})
}

func TestListRecentSetsForExercise(t *testing.T) {
	db := testDB(t)

	a1, _ := CreateAthlete(db, "Athlete A", "", "")
	a2, _ := CreateAthlete(db, "Athlete B", "", "")
	e, _ := CreateExercise(db, "Shared Lift", "", 0, "", "", 0)

	w1, _ := CreateWorkout(db, a1.ID, "2026-01-01", "")
	AddSet(db, w1.ID, e.ID, 5, 135, 0, "")

	w2, _ := CreateWorkout(db, a2.ID, "2026-01-02", "")
	AddSet(db, w2.ID, e.ID, 8, 95, 0, "")

	sets, err := ListRecentSetsForExercise(db, e.ID)
	if err != nil {
		t.Fatalf("list recent sets: %v", err)
	}
	if len(sets) != 2 {
		t.Fatalf("count = %d, want 2", len(sets))
	}
	// Most recent date first.
	if sets[0].AthleteName != "Athlete B" {
		t.Errorf("first set athlete = %q, want Athlete B", sets[0].AthleteName)
	}

	t.Run("empty for unused exercise", func(t *testing.T) {
		e2, _ := CreateExercise(db, "Unused Lift", "", 0, "", "", 0)
		sets, err := ListRecentSetsForExercise(db, e2.ID)
		if err != nil {
			t.Fatalf("list recent sets: %v", err)
		}
		if len(sets) != 0 {
			t.Errorf("count = %d, want 0", len(sets))
		}
	})
}
