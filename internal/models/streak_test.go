package models

import (
	"database/sql"
	"testing"
	"time"
)

func TestWeeklyStreak_Status(t *testing.T) {
	tests := []struct {
		name     string
		streak   WeeklyStreak
		wantStat string
	}{
		{"no assignments", WeeklyStreak{AssignedCount: 0, CompletedCount: 0}, "none"},
		{"all complete", WeeklyStreak{AssignedCount: 3, CompletedCount: 3}, "complete"},
		{"over complete", WeeklyStreak{AssignedCount: 3, CompletedCount: 5}, "complete"},
		{"partial", WeeklyStreak{AssignedCount: 5, CompletedCount: 2}, "partial"},
		{"missed", WeeklyStreak{AssignedCount: 3, CompletedCount: 0}, "missed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.streak.Status(); got != tt.wantStat {
				t.Errorf("Status() = %q, want %q", got, tt.wantStat)
			}
		})
	}
}

func TestWeeklyStreak_Label(t *testing.T) {
	tests := []struct {
		name   string
		streak WeeklyStreak
		want   string
	}{
		{"no assignments", WeeklyStreak{AssignedCount: 0}, "—"},
		{"partial", WeeklyStreak{AssignedCount: 5, CompletedCount: 3}, "3/5"},
		{"complete", WeeklyStreak{AssignedCount: 3, CompletedCount: 3}, "3/3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.streak.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWeeklyStreaks_NoAssignments(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)

	streaks, err := WeeklyStreaks(db, a.ID, 4)
	if err != nil {
		t.Fatalf("weekly streaks: %v", err)
	}
	if len(streaks) != 4 {
		t.Fatalf("streaks = %d, want 4", len(streaks))
	}
	for i, s := range streaks {
		if s.AssignedCount != 0 {
			t.Errorf("week %d: assigned = %d, want 0", i, s.AssignedCount)
		}
		if s.CompletedCount != 0 {
			t.Errorf("week %d: completed = %d, want 0", i, s.CompletedCount)
		}
		if s.Status() != "none" {
			t.Errorf("week %d: status = %q, want 'none'", i, s.Status())
		}
	}
}

func TestWeeklyStreaks_DefaultWeeks(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)

	// Pass 0 weeks — should default to 8.
	streaks, err := WeeklyStreaks(db, a.ID, 0)
	if err != nil {
		t.Fatalf("weekly streaks: %v", err)
	}
	if len(streaks) != 8 {
		t.Errorf("streaks = %d, want 8 (default)", len(streaks))
	}
}

func TestWeeklyStreaks_WithData(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Streak Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	bench, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	squat, _ := CreateExercise(db, "Back Squat", "", "", "", 0)

	// Assign both exercises.
	AssignExercise(db, a.ID, bench.ID, 0)
	AssignExercise(db, a.ID, squat.ID, 0)

	// Create a workout and log sets for both exercises this week.
	// Use today's date so the workout falls in the current week.
	today := time.Now().Format("2006-01-02")
	w, err := CreateWorkout(db, a.ID, today, "")
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}
	AddSet(db, w.ID, bench.ID, 5, 135.0, 0, "", "", "")
	AddSet(db, w.ID, squat.ID, 5, 225.0, 0, "", "", "")

	streaks, err := WeeklyStreaks(db, a.ID, 4)
	if err != nil {
		t.Fatalf("weekly streaks: %v", err)
	}
	if len(streaks) != 4 {
		t.Fatalf("streaks = %d, want 4", len(streaks))
	}

	// The most recent week (last entry) should have both exercises completed.
	lastWeek := streaks[len(streaks)-1]
	if lastWeek.AssignedCount != 2 {
		t.Errorf("last week assigned = %d, want 2", lastWeek.AssignedCount)
	}
	if lastWeek.CompletedCount != 2 {
		t.Errorf("last week completed = %d, want 2", lastWeek.CompletedCount)
	}
	if lastWeek.Status() != "complete" {
		t.Errorf("last week status = %q, want 'complete'", lastWeek.Status())
	}
}

func TestWeeklyStreaks_PartialCompletion(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Partial Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	bench, _ := CreateExercise(db, "Bench", "", "", "", 0)
	squat, _ := CreateExercise(db, "Squat", "", "", "", 0)
	deadlift, _ := CreateExercise(db, "Deadlift", "", "", "", 0)

	AssignExercise(db, a.ID, bench.ID, 0)
	AssignExercise(db, a.ID, squat.ID, 0)
	AssignExercise(db, a.ID, deadlift.ID, 0)

	// Log only bench this week using today's date.
	today := time.Now().Format("2006-01-02")
	w, _ := CreateWorkout(db, a.ID, today, "")
	AddSet(db, w.ID, bench.ID, 5, 135.0, 0, "", "", "")

	streaks, err := WeeklyStreaks(db, a.ID, 1)
	if err != nil {
		t.Fatalf("weekly streaks: %v", err)
	}

	if streaks[0].AssignedCount != 3 {
		t.Errorf("assigned = %d, want 3", streaks[0].AssignedCount)
	}
	if streaks[0].CompletedCount != 1 {
		t.Errorf("completed = %d, want 1", streaks[0].CompletedCount)
	}
	if streaks[0].Status() != "partial" {
		t.Errorf("status = %q, want 'partial'", streaks[0].Status())
	}
}

func TestWeeklyStreaks_UnassignedExercisesNotCounted(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Unassigned Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	bench, _ := CreateExercise(db, "Press", "", "", "", 0)
	extra, _ := CreateExercise(db, "Extra Move", "", "", "", 0)

	// Only assign bench.
	AssignExercise(db, a.ID, bench.ID, 0)

	// Log both exercises.
	today := time.Now().Format("2006-01-02")
	w, _ := CreateWorkout(db, a.ID, today, "")
	AddSet(db, w.ID, bench.ID, 5, 135.0, 0, "", "", "")
	AddSet(db, w.ID, extra.ID, 10, 0, 0, "", "", "")

	streaks, err := WeeklyStreaks(db, a.ID, 1)
	if err != nil {
		t.Fatalf("weekly streaks: %v", err)
	}

	// Only 1 assigned, only 1 counted as completed (extra is not assigned).
	if streaks[0].AssignedCount != 1 {
		t.Errorf("assigned = %d, want 1", streaks[0].AssignedCount)
	}
	if streaks[0].CompletedCount != 1 {
		t.Errorf("completed = %d, want 1", streaks[0].CompletedCount)
	}
}
