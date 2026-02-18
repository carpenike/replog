package models

import (
	"database/sql"
	"testing"
)

func TestAssignmentLifecycle(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Assign Athlete", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(db, "Assign Exercise", "foundational", "", "", 0)

	t.Run("assign exercise", func(t *testing.T) {
		ae, err := AssignExercise(db, a.ID, e.ID, 0)
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
		_, err := AssignExercise(db, a.ID, e.ID, 0)
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
		ae, err := ReactivateAssignment(db, a.ID, e.ID, 0)
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

	a, _ := CreateAthlete(db, "Deact Athlete", "", "", "", sql.NullInt64{}, true)
	e1, _ := CreateExercise(db, "Deact Ex 1", "", "", "", 0)
	e2, _ := CreateExercise(db, "Deact Ex 2", "", "", "", 0)

	// Assign both, deactivate e1 only.
	ae1, _ := AssignExercise(db, a.ID, e1.ID, 0)
	AssignExercise(db, a.ID, e2.ID, 0)
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

	a, _ := CreateAthlete(db, "Unassigned Athlete", "", "", "", sql.NullInt64{}, true)
	e1, _ := CreateExercise(db, "Assigned Ex", "", "", "", 0)
	CreateExercise(db, "Free Ex", "", "", "", 0)

	AssignExercise(db, a.ID, e1.ID, 0)

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

func TestListAssignedAthletes(t *testing.T) {
	db := testDB(t)

	a1, _ := CreateAthlete(db, "Alice", "", "", "", sql.NullInt64{}, true)
	a2, _ := CreateAthlete(db, "Bob", "", "", "", sql.NullInt64{}, true)
	e, _ := CreateExercise(db, "Shared Exercise", "", "", "", 0)

	AssignExercise(db, a1.ID, e.ID, 0)
	AssignExercise(db, a2.ID, e.ID, 0)

	t.Run("both assigned", func(t *testing.T) {
		athletes, err := ListAssignedAthletes(db, e.ID)
		if err != nil {
			t.Fatalf("list assigned: %v", err)
		}
		if len(athletes) != 2 {
			t.Errorf("count = %d, want 2", len(athletes))
		}
	})

	t.Run("deactivated not included", func(t *testing.T) {
		active, _ := ListActiveAssignments(db, a1.ID)
		DeactivateAssignment(db, active[0].ID)

		athletes, err := ListAssignedAthletes(db, e.ID)
		if err != nil {
			t.Fatalf("list assigned: %v", err)
		}
		if len(athletes) != 1 {
			t.Errorf("count = %d, want 1", len(athletes))
		}
		if athletes[0].AthleteName != "Bob" {
			t.Errorf("name = %q, want Bob", athletes[0].AthleteName)
		}
	})

	t.Run("empty for unassigned exercise", func(t *testing.T) {
		e2, _ := CreateExercise(db, "Lonely Exercise", "", "", "", 0)
		athletes, err := ListAssignedAthletes(db, e2.ID)
		if err != nil {
			t.Fatalf("list assigned: %v", err)
		}
		if len(athletes) != 0 {
			t.Errorf("count = %d, want 0", len(athletes))
		}
	})
}

func TestAssignProgramExercises(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Program Athlete", "", "", "", sql.NullInt64{}, true)
	ex1, _ := CreateExercise(db, "Squat", "", "", "", 0)
	ex2, _ := CreateExercise(db, "Bench", "", "", "", 0)
	ex3, _ := CreateExercise(db, "Deadlift", "", "", "", 0)

	tmpl, _ := CreateProgramTemplate(db, "Test Program", "", 4, 3)
	reps5 := 5
	pct75 := 75.0
	CreatePrescribedSet(db, tmpl.ID, ex1.ID, 1, 1, 1, &reps5, &pct75, "reps", "")
	CreatePrescribedSet(db, tmpl.ID, ex1.ID, 1, 1, 2, &reps5, &pct75, "reps", "") // duplicate exercise
	CreatePrescribedSet(db, tmpl.ID, ex2.ID, 1, 2, 1, &reps5, &pct75, "reps", "")
	CreatePrescribedSet(db, tmpl.ID, ex3.ID, 1, 3, 1, &reps5, &pct75, "reps", "")

	t.Run("assigns all program exercises", func(t *testing.T) {
		n, err := AssignProgramExercises(db, athlete.ID, tmpl.ID)
		if err != nil {
			t.Fatalf("auto-assign: %v", err)
		}
		if n != 3 {
			t.Errorf("assigned count = %d, want 3", n)
		}

		active, _ := ListActiveAssignments(db, athlete.ID)
		if len(active) != 3 {
			t.Errorf("active count = %d, want 3", len(active))
		}
	})

	t.Run("skips already assigned exercises", func(t *testing.T) {
		n, err := AssignProgramExercises(db, athlete.ID, tmpl.ID)
		if err != nil {
			t.Fatalf("auto-assign: %v", err)
		}
		if n != 0 {
			t.Errorf("assigned count = %d, want 0 (all already assigned)", n)
		}
	})

	t.Run("partial overlap", func(t *testing.T) {
		athlete2, _ := CreateAthlete(db, "Partial Athlete", "", "", "", sql.NullInt64{}, true)
		AssignExercise(db, athlete2.ID, ex1.ID, 0) // pre-assign one exercise

		n, err := AssignProgramExercises(db, athlete2.ID, tmpl.ID)
		if err != nil {
			t.Fatalf("auto-assign: %v", err)
		}
		if n != 2 {
			t.Errorf("assigned count = %d, want 2", n)
		}

		active, _ := ListActiveAssignments(db, athlete2.ID)
		if len(active) != 3 {
			t.Errorf("active count = %d, want 3", len(active))
		}
	})

	t.Run("empty program template", func(t *testing.T) {
		emptyTmpl, _ := CreateProgramTemplate(db, "Empty Program", "", 1, 1)
		n, err := AssignProgramExercises(db, athlete.ID, emptyTmpl.ID)
		if err != nil {
			t.Fatalf("auto-assign empty: %v", err)
		}
		if n != 0 {
			t.Errorf("assigned count = %d, want 0", n)
		}
	})
}
