package models

import (
	"testing"
)

func TestSetCRUD(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Set Athlete", "", "")
	e, _ := CreateExercise(db, "Test Lift", "", 0, "", "")
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
	e, _ := CreateExercise(db, "Update Lift", "", 0, "", "")
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
	e, _ := CreateExercise(db, "Del Lift", "", 0, "", "")
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
	e1, _ := CreateExercise(db, "Lift A", "", 0, "", "")
	e2, _ := CreateExercise(db, "Lift B", "", 0, "", "")
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
