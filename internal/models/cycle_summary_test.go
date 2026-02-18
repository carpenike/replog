package models

import (
	"database/sql"
	"testing"
	"time"
)

func TestGetCycleSummary_NoProgramReturnsNil(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Test", "", "", "", sql.NullInt64{}, true)

	summary, err := GetCycleSummary(db, a.ID, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Error("expected nil summary when no program assigned")
	}
}

func TestGetCycleSummary_NoCycleCompletedReturnsNil(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Test", "", "", "", sql.NullInt64{}, true)
	tmpl, _ := CreateProgramTemplate(db, "531", "", 3, 4)
	AssignProgram(db, a.ID, tmpl.ID, "2026-01-01", "", "")

	// No workouts logged — still in cycle 1.
	summary, err := GetCycleSummary(db, a.ID, mustParseDate("2026-02-01"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Error("expected nil summary when no cycle completed")
	}
}

func TestGetCycleSummary_AfterFirstCycle(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Test", "", "", "", sql.NullInt64{}, true)
	squat, _ := CreateExercise(db, "Squat", "", "", "", 0)
	bench, _ := CreateExercise(db, "Bench Press", "", "", "", 0)

	// Create a 3-week × 2-day program (6 workouts per cycle).
	tmpl, _ := CreateProgramTemplate(db, "531", "", 3, 2)
	AssignProgram(db, a.ID, tmpl.ID, "2026-01-01", "", "")

	// Add AMRAP prescribed sets (reps=NULL) on week 3 day 1.
	CreatePrescribedSet(db, tmpl.ID, squat.ID, 3, 1, 1, nil, ptrFloat(95), "", "")
	CreatePrescribedSet(db, tmpl.ID, bench.ID, 3, 1, 2, nil, ptrFloat(95), "", "")

	// Add some non-AMRAP sets.
	five := 5
	CreatePrescribedSet(db, tmpl.ID, squat.ID, 1, 1, 1, &five, ptrFloat(65), "", "")

	// Add progression rules.
	SetProgressionRule(db, tmpl.ID, squat.ID, 10.0)
	SetProgressionRule(db, tmpl.ID, bench.ID, 5.0)

	// Set training maxes.
	SetTrainingMax(db, a.ID, squat.ID, 300, "2026-01-01", "")
	SetTrainingMax(db, a.ID, bench.ID, 200, "2026-01-01", "")

	// Log 6 workouts to complete cycle 1.
	dates := []string{
		"2026-01-02", "2026-01-03", "2026-01-06", "2026-01-07", "2026-01-09", "2026-01-10",
	}
	for _, d := range dates {
		w, err := CreateWorkout(db, a.ID, d, "")
		if err != nil {
			t.Fatalf("create workout %s: %v", d, err)
		}

		// On the 5th workout (week 3, day 1 in the cycle), log AMRAP results.
		if d == "2026-01-09" {
			AddSet(db, w.ID, squat.ID, 5, 285, 0, "reps", "")  // 5 reps at 95%
			AddSet(db, w.ID, bench.ID, 3, 190, 0, "reps", "") // 3 reps at 95%
		}
	}

	// Get cycle summary — should show the completed first cycle.
	summary, err := GetCycleSummary(db, a.ID, mustParseDate("2026-01-15"))
	if err != nil {
		t.Fatalf("get cycle summary: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary after completing a cycle")
	}

	if summary.CycleNumber != 1 {
		t.Errorf("expected cycle 1, got %d", summary.CycleNumber)
	}

	if len(summary.Suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(summary.Suggestions))
	}

	// Suggestions should be ordered by exercise name (Bench Press, Squat).
	benchSuggestion := summary.Suggestions[0]
	squatSuggestion := summary.Suggestions[1]

	if benchSuggestion.ExerciseName != "Bench Press" {
		t.Errorf("expected first suggestion to be Bench Press, got %q", benchSuggestion.ExerciseName)
	}
	if benchSuggestion.CurrentTM != 200 {
		t.Errorf("expected bench current TM 200, got %f", benchSuggestion.CurrentTM)
	}
	if benchSuggestion.SuggestedTM != 205 {
		t.Errorf("expected bench suggested TM 205, got %f", benchSuggestion.SuggestedTM)
	}
	if benchSuggestion.Increment != 5.0 {
		t.Errorf("expected bench increment 5.0, got %f", benchSuggestion.Increment)
	}

	if squatSuggestion.ExerciseName != "Squat" {
		t.Errorf("expected second suggestion to be Squat, got %q", squatSuggestion.ExerciseName)
	}
	if squatSuggestion.CurrentTM != 300 {
		t.Errorf("expected squat current TM 300, got %f", squatSuggestion.CurrentTM)
	}
	if squatSuggestion.SuggestedTM != 310 {
		t.Errorf("expected squat suggested TM 310, got %f", squatSuggestion.SuggestedTM)
	}

	// Check AMRAP results are populated.
	if len(summary.AllAMRAPs) == 0 {
		t.Error("expected AMRAP results in summary")
	}
}

func TestGetCycleSummary_NoTMSkipsExercise(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Test", "", "", "", sql.NullInt64{}, true)
	squat, _ := CreateExercise(db, "Squat", "", "", "", 0)

	tmpl, _ := CreateProgramTemplate(db, "531", "", 1, 2)
	AssignProgram(db, a.ID, tmpl.ID, "2026-01-01", "", "")

	// Progression rule but no TM set.
	SetProgressionRule(db, tmpl.ID, squat.ID, 10.0)

	// Complete one cycle (2 workouts).
	CreateWorkout(db, a.ID, "2026-01-02", "")
	CreateWorkout(db, a.ID, "2026-01-03", "")

	summary, err := GetCycleSummary(db, a.ID, mustParseDate("2026-01-10"))
	if err != nil {
		t.Fatalf("get cycle summary: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}

	if len(summary.Suggestions) != 0 {
		t.Errorf("expected 0 suggestions (no TM), got %d", len(summary.Suggestions))
	}
}

func TestTMSuggestion_IncrementLabel(t *testing.T) {
	tests := []struct {
		increment float64
		want      string
	}{
		{5.0, "5"},
		{10.0, "10"},
		{2.5, "2.5"},
	}

	for _, tt := range tests {
		s := &TMSuggestion{Increment: tt.increment}
		if got := s.IncrementLabel(); got != tt.want {
			t.Errorf("IncrementLabel(%f) = %q, want %q", tt.increment, got, tt.want)
		}
	}
}


