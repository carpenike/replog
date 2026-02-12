package models

import (
	"strings"
	"testing"
)

func TestWorkoutCRUD(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Workout Athlete", "", "")

	t.Run("create workout", func(t *testing.T) {
		w, err := CreateWorkout(db, a.ID, "2026-02-01", "leg day")
		if err != nil {
			t.Fatalf("create workout: %v", err)
		}
		if !strings.HasPrefix(w.Date, "2026-02-01") {
			t.Errorf("date = %q, want prefix 2026-02-01", w.Date)
		}
		if !w.Notes.Valid || w.Notes.String != "leg day" {
			t.Errorf("notes = %v, want leg day", w.Notes)
		}
	})

	t.Run("one workout per day", func(t *testing.T) {
		_, err := CreateWorkout(db, a.ID, "2026-02-01", "")
		if err != ErrWorkoutExists {
			t.Errorf("err = %v, want ErrWorkoutExists", err)
		}
	})

	t.Run("different dates ok", func(t *testing.T) {
		_, err := CreateWorkout(db, a.ID, "2026-02-02", "")
		if err != nil {
			t.Fatalf("create workout: %v", err)
		}
	})
}

func TestUpdateWorkoutNotes(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Notes Athlete", "", "")
	w, _ := CreateWorkout(db, a.ID, "2026-03-01", "")

	if err := UpdateWorkoutNotes(db, w.ID, "updated notes"); err != nil {
		t.Fatalf("update notes: %v", err)
	}

	updated, _ := GetWorkoutByID(db, w.ID)
	if !updated.Notes.Valid || updated.Notes.String != "updated notes" {
		t.Errorf("notes = %v, want updated notes", updated.Notes)
	}
}

func TestDeleteWorkout(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Del Athlete", "", "")
	w, _ := CreateWorkout(db, a.ID, "2026-04-01", "")

	if err := DeleteWorkout(db, w.ID); err != nil {
		t.Fatalf("delete workout: %v", err)
	}
	_, err := GetWorkoutByID(db, w.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListWorkouts(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "List Athlete", "", "")
	CreateWorkout(db, a.ID, "2026-01-01", "")
	CreateWorkout(db, a.ID, "2026-01-15", "")
	CreateWorkout(db, a.ID, "2026-01-10", "")

	workouts, err := ListWorkouts(db, a.ID)
	if err != nil {
		t.Fatalf("list workouts: %v", err)
	}
	if len(workouts) != 3 {
		t.Fatalf("count = %d, want 3", len(workouts))
	}
	// Should be ordered by date descending.
	if !strings.HasPrefix(workouts[0].Date, "2026-01-15") {
		t.Errorf("first date = %q, want prefix 2026-01-15", workouts[0].Date)
	}
}
