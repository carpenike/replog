package models

import (
	"testing"
)

func TestAssignmentLifecycle(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Assign Athlete", "", "")
	e, _ := CreateExercise(db, "Assign Exercise", "foundational", 20, "", "")

	t.Run("assign exercise", func(t *testing.T) {
		ae, err := AssignExercise(db, a.ID, e.ID)
		if err != nil {
			t.Fatalf("assign: %v", err)
		}
		if !ae.Active {
			t.Error("assignment should be active")
		}
		if ae.ExerciseName != "Assign Exercise" {
			t.Errorf("exercise name = %q, want Assign Exercise", ae.ExerciseName)
		}
	})

	t.Run("duplicate assignment", func(t *testing.T) {
		_, err := AssignExercise(db, a.ID, e.ID)
		if err != ErrAlreadyAssigned {
			t.Errorf("err = %v, want ErrAlreadyAssigned", err)
		}
	})

	t.Run("list active", func(t *testing.T) {
		assignments, err := ListActiveAssignments(db, a.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(assignments) != 1 {
			t.Errorf("count = %d, want 1", len(assignments))
		}
	})

	t.Run("deactivate", func(t *testing.T) {
		assignments, _ := ListActiveAssignments(db, a.ID)
		if err := DeactivateAssignment(db, assignments[0].ID); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		active, _ := ListActiveAssignments(db, a.ID)
		if len(active) != 0 {
			t.Errorf("active count = %d, want 0", len(active))
		}
	})

	t.Run("reactivate creates new row", func(t *testing.T) {
		ae, err := ReactivateAssignment(db, a.ID, e.ID)
		if err != nil {
			t.Fatalf("reactivate: %v", err)
		}
		if !ae.Active {
			t.Error("reactivated assignment should be active")
		}

		active, _ := ListActiveAssignments(db, a.ID)
		if len(active) != 1 {
			t.Errorf("active count = %d, want 1", len(active))
		}
	})
}

func TestListDeactivatedAssignments(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Deact Athlete", "", "")
	e1, _ := CreateExercise(db, "Deact Ex 1", "", 0, "", "")
	e2, _ := CreateExercise(db, "Deact Ex 2", "", 0, "", "")

	// Assign both, deactivate e1 only.
	ae1, _ := AssignExercise(db, a.ID, e1.ID)
	AssignExercise(db, a.ID, e2.ID)
	DeactivateAssignment(db, ae1.ID)

	deactivated, err := ListDeactivatedAssignments(db, a.ID)
	if err != nil {
		t.Fatalf("list deactivated: %v", err)
	}
	if len(deactivated) != 1 {
		t.Fatalf("count = %d, want 1", len(deactivated))
	}
	if deactivated[0].ExerciseName != "Deact Ex 1" {
		t.Errorf("name = %q, want Deact Ex 1", deactivated[0].ExerciseName)
	}
}

func TestListUnassignedExercises(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Unassigned Athlete", "", "")
	e1, _ := CreateExercise(db, "Assigned Ex", "", 0, "", "")
	CreateExercise(db, "Free Ex", "", 0, "", "")

	AssignExercise(db, a.ID, e1.ID)

	unassigned, err := ListUnassignedExercises(db, a.ID)
	if err != nil {
		t.Fatalf("list unassigned: %v", err)
	}
	if len(unassigned) != 1 {
		t.Fatalf("count = %d, want 1", len(unassigned))
	}
	if unassigned[0].Name != "Free Ex" {
		t.Errorf("name = %q, want Free Ex", unassigned[0].Name)
	}
}
