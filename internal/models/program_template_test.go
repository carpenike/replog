package models

import (
	"database/sql"
	"testing"
)

func TestCreateProgramTemplate(t *testing.T) {
	db := testDB(t)

	t.Run("basic create global", func(t *testing.T) {
		tmpl, err := CreateProgramTemplate(db, nil, "5/3/1 BBB", "Boring But Big", 4, 4, false, "")
		if err != nil {
			t.Fatalf("create program template: %v", err)
		}
		if tmpl.Name != "5/3/1 BBB" {
			t.Errorf("name = %q, want 5/3/1 BBB", tmpl.Name)
		}
		if tmpl.AthleteID != nil {
			t.Errorf("athlete_id = %v, want nil (global)", tmpl.AthleteID)
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

	t.Run("athlete-scoped create", func(t *testing.T) {
		a, _ := CreateAthlete(db, "Scoped Test", "", "", "", "", "", "", sql.NullInt64{}, true)
		tmpl, err := CreateProgramTemplate(db, &a.ID, "Athlete Program", "", 3, 3, false, "")
		if err != nil {
			t.Fatalf("create athlete-scoped template: %v", err)
		}
		if tmpl.AthleteID == nil || *tmpl.AthleteID != a.ID {
			t.Errorf("athlete_id = %v, want %d", tmpl.AthleteID, a.ID)
		}
	})

	t.Run("duplicate name global", func(t *testing.T) {
		_, err := CreateProgramTemplate(db, nil, "5/3/1 BBB", "", 1, 1, false, "")
		if err == nil {
			t.Error("expected error for duplicate name")
		}
	})
}

func TestListProgramTemplates(t *testing.T) {
	db := testDB(t)

	CreateProgramTemplate(db, nil, "Program A", "", 4, 4, false, "")
	CreateProgramTemplate(db, nil, "Program B", "", 3, 3, false, "")

	templates, err := ListProgramTemplates(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("len = %d, want 2", len(templates))
	}
}

func TestListProgramTemplatesForAthlete(t *testing.T) {
	db := testDB(t)

	a1, _ := CreateAthlete(db, "Athlete One", "", "", "", "", "", "", sql.NullInt64{}, true)
	a2, _ := CreateAthlete(db, "Athlete Two", "", "", "", "", "", "", sql.NullInt64{}, true)

	// Global template.
	CreateProgramTemplate(db, nil, "Global Program", "", 4, 4, false, "")
	// Athlete-1-scoped template.
	CreateProgramTemplate(db, &a1.ID, "A1 Program", "", 3, 3, false, "")
	// Athlete-2-scoped template.
	CreateProgramTemplate(db, &a2.ID, "A2 Program", "", 2, 2, false, "")

	t.Run("athlete 1 sees global + own", func(t *testing.T) {
		templates, err := ListProgramTemplatesForAthlete(db, a1.ID)
		if err != nil {
			t.Fatalf("list for athlete 1: %v", err)
		}
		if len(templates) != 2 {
			t.Errorf("len = %d, want 2 (global + a1)", len(templates))
		}
		names := make(map[string]bool)
		for _, tmpl := range templates {
			names[tmpl.Name] = true
		}
		if !names["Global Program"] || !names["A1 Program"] {
			t.Errorf("expected Global Program and A1 Program, got %v", names)
		}
	})

	t.Run("athlete 2 sees global + own", func(t *testing.T) {
		templates, err := ListProgramTemplatesForAthlete(db, a2.ID)
		if err != nil {
			t.Fatalf("list for athlete 2: %v", err)
		}
		if len(templates) != 2 {
			t.Errorf("len = %d, want 2 (global + a2)", len(templates))
		}
	})

	t.Run("all templates listed globally", func(t *testing.T) {
		templates, err := ListProgramTemplates(db)
		if err != nil {
			t.Fatalf("list all: %v", err)
		}
		if len(templates) != 3 {
			t.Errorf("len = %d, want 3", len(templates))
		}
	})

	t.Run("same name allowed for different athletes", func(t *testing.T) {
		_, err := CreateProgramTemplate(db, &a2.ID, "A1 Program", "", 1, 1, false, "")
		if err != nil {
			t.Errorf("should allow same name for different athlete, got: %v", err)
		}
	})

	t.Run("duplicate name within same athlete rejected", func(t *testing.T) {
		_, err := CreateProgramTemplate(db, &a1.ID, "A1 Program", "", 1, 1, false, "")
		if err == nil {
			t.Error("expected unique violation for duplicate name within same athlete")
		}
	})
}

func TestListReferenceTemplatesByAudience(t *testing.T) {
	db := testDB(t)

	// Create templates with different audiences.
	CreateProgramTemplate(db, nil, "Youth Foundations", "For kids", 1, 2, true, "youth")
	CreateProgramTemplate(db, nil, "5/3/1 Program", "For adults", 4, 4, true, "adult")
	CreateProgramTemplate(db, nil, "No Audience", "Unclassified", 1, 1, false, "")

	// Athlete-scoped template should NOT appear.
	a, _ := CreateAthlete(db, "Aud Test", "", "", "", "", "", "", sql.NullInt64{}, true)
	CreateProgramTemplate(db, &a.ID, "Athlete Program", "", 3, 3, false, "youth")

	t.Run("youth audience", func(t *testing.T) {
		templates, err := ListReferenceTemplatesByAudience(db, "youth")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(templates) != 1 {
			t.Fatalf("len = %d, want 1", len(templates))
		}
		if templates[0].Name != "Youth Foundations" {
			t.Errorf("name = %q, want Youth Foundations", templates[0].Name)
		}
		if !templates[0].Audience.Valid || templates[0].Audience.String != "youth" {
			t.Errorf("audience = %v, want youth", templates[0].Audience)
		}
	})

	t.Run("adult audience", func(t *testing.T) {
		templates, err := ListReferenceTemplatesByAudience(db, "adult")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(templates) != 1 {
			t.Fatalf("len = %d, want 1", len(templates))
		}
		if templates[0].Name != "5/3/1 Program" {
			t.Errorf("name = %q, want 5/3/1 Program", templates[0].Name)
		}
	})
}

func TestCreateProgramTemplateAudience(t *testing.T) {
	db := testDB(t)

	t.Run("youth audience", func(t *testing.T) {
		tmpl, err := CreateProgramTemplate(db, nil, "Youth Prog", "", 1, 2, true, "youth")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if !tmpl.Audience.Valid || tmpl.Audience.String != "youth" {
			t.Errorf("audience = %v, want youth", tmpl.Audience)
		}
	})

	t.Run("adult audience", func(t *testing.T) {
		tmpl, err := CreateProgramTemplate(db, nil, "Adult Prog", "", 4, 4, false, "adult")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if !tmpl.Audience.Valid || tmpl.Audience.String != "adult" {
			t.Errorf("audience = %v, want adult", tmpl.Audience)
		}
	})

	t.Run("no audience", func(t *testing.T) {
		tmpl, err := CreateProgramTemplate(db, nil, "Generic Prog", "", 1, 1, false, "")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if tmpl.Audience.Valid {
			t.Errorf("audience should be NULL, got %v", tmpl.Audience)
		}
	})
}

func TestUpdateProgramTemplate(t *testing.T) {
	db := testDB(t)

	tmpl, _ := CreateProgramTemplate(db, nil, "Old Name", "", 4, 4, false, "")

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
		tmpl, _ := CreateProgramTemplate(db, nil, "To Delete", "", 1, 1, false, "")
		if err := DeleteProgramTemplate(db, tmpl.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
	})

	t.Run("delete in use", func(t *testing.T) {
		tmpl, _ := CreateProgramTemplate(db, nil, "In Use", "", 1, 1, false, "")
		a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
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

	tmpl, _ := CreateProgramTemplate(db, nil, "Test Program", "", 4, 4, false, "")
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

	tmpl, _ := CreateProgramTemplate(db, nil, "5/3/1", "", 4, 4, false, "")
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)

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
	tmpl, _ := CreateProgramTemplate(db, nil, "Test 531", "", 4, 4, false, "")
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

	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)

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
		a2, _ := CreateAthlete(db, "No Program", "", "", "", "", "", "", sql.NullInt64{}, true)
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

	tmpl, _ := CreateProgramTemplate(db, nil, "Copy Test", "", 3, 3, false, "")
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

	t.Run("copy replaces existing sets in target week", func(t *testing.T) {
		// Week 2 already has 3 sets from the previous subtest.
		// Add an extra set to week 2 that doesn't exist in week 1.
		e3, _ := CreateExercise(db, "Deadlift", "", "", "", 0)
		r8 := 8
		CreatePrescribedSet(db, tmpl.ID, e3.ID, 2, 3, 1, &r8, nil, nil, 0, "", "")

		// Copy week 1 → week 2 again; should replace all 4 sets with 3.
		inserted, err := CopyWeek(db, tmpl.ID, 1, 2)
		if err != nil {
			t.Fatalf("copy week: %v", err)
		}
		if inserted != 3 {
			t.Errorf("inserted = %d, want 3", inserted)
		}

		// Verify the extra set is gone.
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
			t.Errorf("week 2 sets = %d, want 3 (extra set should be deleted)", week2Sets)
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
