package models

import (
	"database/sql"
	"testing"
)

func TestCreateProgramTemplate(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		tmpl, err := CreateProgramTemplate(db, "5/3/1 BBB", "Boring But Big", 4, 4, false)
		if err != nil {
			t.Fatalf("create program template: %v", err)
		}
		if tmpl.Name != "5/3/1 BBB" {
			t.Errorf("name = %q, want 5/3/1 BBB", tmpl.Name)
		}
		if tmpl.NumWeeks != 4 {
			t.Errorf("num_weeks = %d, want 4", tmpl.NumWeeks)
		}
		if tmpl.NumDays != 4 {
			t.Errorf("num_days = %d, want 4", tmpl.NumDays)
		}
		if !tmpl.Description.Valid || tmpl.Description.String != "Boring But Big" {
			t.Errorf("description = %v, want Boring But Big", tmpl.Description)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := CreateProgramTemplate(db, "5/3/1 BBB", "", 1, 1, false)
		if err == nil {
			t.Error("expected error for duplicate name")
		}
	})
}

func TestListProgramTemplates(t *testing.T) {
	db := testDB(t)

	CreateProgramTemplate(db, "Program A", "", 4, 4, false)
	CreateProgramTemplate(db, "Program B", "", 3, 3, false)

	templates, err := ListProgramTemplates(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("len = %d, want 2", len(templates))
	}
}

func TestUpdateProgramTemplate(t *testing.T) {
	db := testDB(t)

	tmpl, _ := CreateProgramTemplate(db, "Old Name", "", 4, 4, false)

	updated, err := UpdateProgramTemplate(db, tmpl.ID, "New Name", "Updated description", 3, 3, false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("name = %q, want New Name", updated.Name)
	}
	if updated.NumWeeks != 3 {
		t.Errorf("num_weeks = %d, want 3", updated.NumWeeks)
	}
}

func TestDeleteProgramTemplate(t *testing.T) {
	db := testDB(t)

	t.Run("delete unused", func(t *testing.T) {
		tmpl, _ := CreateProgramTemplate(db, "To Delete", "", 1, 1, false)
		if err := DeleteProgramTemplate(db, tmpl.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
	})

	t.Run("delete in use", func(t *testing.T) {
		tmpl, _ := CreateProgramTemplate(db, "In Use", "", 1, 1, false)
		a, _ := CreateAthlete(db, "Test Athlete", "", "", "", sql.NullInt64{}, true)
		_, err := AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "", "")
		if err != nil {
			t.Fatalf("assign program: %v", err)
		}

		err = DeleteProgramTemplate(db, tmpl.ID)
		if err != ErrTemplateInUse {
			t.Errorf("err = %v, want ErrTemplateInUse", err)
		}
	})
}

func TestPrescribedSets(t *testing.T) {
	db := testDB(t)

	tmpl, _ := CreateProgramTemplate(db, "Test Program", "", 4, 4, false)
	e, _ := CreateExercise(db, "Bench Press", "", "", "", 0)

	t.Run("create prescribed set", func(t *testing.T) {
		reps := 5
		pct := 75.0
		ps, err := CreatePrescribedSet(db, tmpl.ID, e.ID, 1, 1, 1, &reps, &pct, nil, 0, "", "heavy")
		if err != nil {
			t.Fatalf("create prescribed set: %v", err)
		}
		if ps.Week != 1 || ps.Day != 1 || ps.SetNumber != 1 {
			t.Errorf("position = w%d d%d s%d, want w1 d1 s1", ps.Week, ps.Day, ps.SetNumber)
		}
		if !ps.Reps.Valid || ps.Reps.Int64 != 5 {
			t.Errorf("reps = %v, want 5", ps.Reps)
		}
		if !ps.Percentage.Valid || ps.Percentage.Float64 != 75.0 {
			t.Errorf("percentage = %v, want 75.0", ps.Percentage)
		}
	})

	t.Run("create AMRAP set (nil reps)", func(t *testing.T) {
		pct := 85.0
		ps, err := CreatePrescribedSet(db, tmpl.ID, e.ID, 1, 1, 2, nil, &pct, nil, 0, "", "")
		if err != nil {
			t.Fatalf("create AMRAP set: %v", err)
		}
		if ps.Reps.Valid {
			t.Errorf("reps should be NULL for AMRAP, got %v", ps.Reps)
		}
		if ps.RepsLabel() != "AMRAP" {
			t.Errorf("RepsLabel = %q, want AMRAP", ps.RepsLabel())
		}
	})

	t.Run("list for day", func(t *testing.T) {
		sets, err := ListPrescribedSetsForDay(db, tmpl.ID, 1, 1)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(sets) != 2 {
			t.Errorf("len = %d, want 2", len(sets))
		}
	})

	t.Run("list all", func(t *testing.T) {
		sets, err := ListPrescribedSets(db, tmpl.ID)
		if err != nil {
			t.Fatalf("list all: %v", err)
		}
		if len(sets) != 2 {
			t.Errorf("len = %d, want 2", len(sets))
		}
	})

	t.Run("delete", func(t *testing.T) {
		reps := 10
		ps, _ := CreatePrescribedSet(db, tmpl.ID, e.ID, 2, 1, 1, &reps, nil, nil, 0, "", "")
		if err := DeletePrescribedSet(db, ps.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
	})
}

func TestAthleteProgram(t *testing.T) {
	db := testDB(t)

	tmpl, _ := CreateProgramTemplate(db, "5/3/1", "", 4, 4, false)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", sql.NullInt64{}, true)

	t.Run("assign program", func(t *testing.T) {
		ap, err := AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "Starting cycle", "")
		if err != nil {
			t.Fatalf("assign: %v", err)
		}
		if !ap.Active {
			t.Error("expected active = true")
		}
		if ap.TemplateName != "5/3/1" {
			t.Errorf("template name = %q, want 5/3/1", ap.TemplateName)
		}
	})

	t.Run("duplicate active", func(t *testing.T) {
		_, err := AssignProgram(db, a.ID, tmpl.ID, "2026-02-15", "", "")
		if err != ErrProgramAlreadyActive {
			t.Errorf("err = %v, want ErrProgramAlreadyActive", err)
		}
	})

	t.Run("get active", func(t *testing.T) {
		ap, err := GetActiveProgram(db, a.ID)
		if err != nil {
			t.Fatalf("get active: %v", err)
		}
		if ap == nil {
			t.Fatal("expected non-nil active program")
		}
	})

	t.Run("deactivate and reassign", func(t *testing.T) {
		ap, _ := GetActiveProgram(db, a.ID)
		if err := DeactivateProgram(db, ap.ID); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		ap2, err := GetActiveProgram(db, a.ID)
		if err != nil {
			t.Fatalf("get active after deactivate: %v", err)
		}
		if ap2 != nil {
			t.Error("expected nil active program after deactivate")
		}

		// Should be able to assign again.
		_, err = AssignProgram(db, a.ID, tmpl.ID, "2026-03-01", "", "")
		if err != nil {
			t.Fatalf("reassign: %v", err)
		}
	})
}

func TestGetPrescription(t *testing.T) {
	db := testDB(t)

	// Set up template: 4 weeks × 4 days, with exercises on W1D1.
	tmpl, _ := CreateProgramTemplate(db, "Test 531", "", 4, 4, false)
	bench, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	squat, _ := CreateExercise(db, "Back Squat", "", "", "", 0)

	// W1D1: Bench 3×5 @ 65%, Squat 3×5 @ 65%
	for i := 1; i <= 3; i++ {
		reps := 5
		pct := 65.0
		CreatePrescribedSet(db, tmpl.ID, bench.ID, 1, 1, i, &reps, &pct, nil, 0, "", "")
		CreatePrescribedSet(db, tmpl.ID, squat.ID, 1, 1, i, &reps, &pct, nil, 0, "", "")
	}

	// W1D2: Bench 3×3 @ 75%
	for i := 1; i <= 3; i++ {
		reps := 3
		pct := 75.0
		CreatePrescribedSet(db, tmpl.ID, bench.ID, 1, 2, i, &reps, &pct, nil, 0, "", "")
	}

	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", sql.NullInt64{}, true)

	// Set training maxes.
	SetTrainingMax(db, a.ID, bench.ID, 200, "2026-01-01", "")
	SetTrainingMax(db, a.ID, squat.ID, 300, "2026-01-01", "")

	// Assign program starting Feb 1.
	AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "", "")

	t.Run("first workout (W1D1)", func(t *testing.T) {
		// Parse a fixed date for repeatable tests.
		today := mustParseDate("2026-02-01")
		rx, err := GetPrescription(db, a.ID, today)
		if err != nil {
			t.Fatalf("get prescription: %v", err)
		}
		if rx == nil {
			t.Fatal("expected non-nil prescription")
		}
		if rx.CurrentWeek != 1 || rx.CurrentDay != 1 {
			t.Errorf("position = W%dD%d, want W1D1", rx.CurrentWeek, rx.CurrentDay)
		}
		if len(rx.Lines) != 2 {
			t.Fatalf("lines = %d, want 2", len(rx.Lines))
		}

		// Lines ordered alphabetically: Back Squat, Bench Press.
		// Find bench line by name.
		var benchLine *PrescriptionLine
		for _, l := range rx.Lines {
			if l.ExerciseName == "Bench Press" {
				benchLine = l
			}
		}
		if benchLine == nil {
			t.Fatal("expected Bench Press line")
		}
		if benchLine.SetsSummary() != "3×5" {
			t.Errorf("sets = %q, want 3×5", benchLine.SetsSummary())
		}
		if benchLine.TargetWeight == nil {
			t.Fatal("expected non-nil target weight")
		}
		// 65% of 200 = 130, rounded to nearest 2.5
		if *benchLine.TargetWeight != 130 {
			t.Errorf("target weight = %.1f, want 130", *benchLine.TargetWeight)
		}
	})

	t.Run("after one workout (W1D2)", func(t *testing.T) {
		// Log one workout to advance to next day.
		CreateWorkout(db, a.ID, "2026-02-01", "")

		today := mustParseDate("2026-02-02")
		rx, err := GetPrescription(db, a.ID, today)
		if err != nil {
			t.Fatalf("get prescription: %v", err)
		}
		if rx.CurrentWeek != 1 || rx.CurrentDay != 2 {
			t.Errorf("position = W%dD%d, want W1D2", rx.CurrentWeek, rx.CurrentDay)
		}
		if len(rx.Lines) != 1 {
			t.Fatalf("lines = %d, want 1 (bench only)", len(rx.Lines))
		}
	})

	t.Run("no active program", func(t *testing.T) {
		a2, _ := CreateAthlete(db, "No Program", "", "", "", sql.NullInt64{}, true)
		rx, err := GetPrescription(db, a2.ID, mustParseDate("2026-02-01"))
		if err != nil {
			t.Fatalf("get prescription: %v", err)
		}
		if rx != nil {
			t.Error("expected nil prescription for athlete without program")
		}
	})
}

func TestCopyWeek(t *testing.T) {
	db := testDB(t)

	tmpl, _ := CreateProgramTemplate(db, "Copy Test", "", 3, 3, false)
	e1, _ := CreateExercise(db, "Squat", "", "", "", 0)
	e2, _ := CreateExercise(db, "Bench", "", "", "", 0)

	// Populate week 1 with sets across two days.
	r5 := 5
	r10 := 10
	pct := 75.0
	CreatePrescribedSet(db, tmpl.ID, e1.ID, 1, 1, 1, &r5, &pct, nil, 1, "", "")
	CreatePrescribedSet(db, tmpl.ID, e1.ID, 1, 1, 2, &r5, &pct, nil, 1, "", "")
	CreatePrescribedSet(db, tmpl.ID, e2.ID, 1, 2, 1, &r10, nil, nil, 2, "", "notes here")

	t.Run("copy to empty week", func(t *testing.T) {
		inserted, err := CopyWeek(db, tmpl.ID, 1, 2)
		if err != nil {
			t.Fatalf("copy week: %v", err)
		}
		if inserted != 3 {
			t.Errorf("inserted = %d, want 3", inserted)
		}

		// Verify target week has the sets.
		sets, err := ListPrescribedSets(db, tmpl.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		week2Sets := 0
		for _, s := range sets {
			if s.Week == 2 {
				week2Sets++
			}
		}
		if week2Sets != 3 {
			t.Errorf("week 2 sets = %d, want 3", week2Sets)
		}
	})

	t.Run("copy to week with existing sets skips duplicates", func(t *testing.T) {
		// Week 2 already has 3 sets from the previous subtest.
		inserted, err := CopyWeek(db, tmpl.ID, 1, 2)
		if err != nil {
			t.Fatalf("copy week: %v", err)
		}
		if inserted != 0 {
			t.Errorf("inserted = %d, want 0 (all duplicates)", inserted)
		}
	})

	t.Run("copy from empty week inserts nothing", func(t *testing.T) {
		inserted, err := CopyWeek(db, tmpl.ID, 3, 2)
		if err != nil {
			t.Fatalf("copy week: %v", err)
		}
		if inserted != 0 {
			t.Errorf("inserted = %d, want 0", inserted)
		}
	})
}
