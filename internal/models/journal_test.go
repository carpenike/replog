package models

import (
	"database/sql"
	"strings"
	"testing"
)

func TestListJournalEntries(t *testing.T) {
	db := testDB(t)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})
	a, _ := CreateAthlete(db, "Test Athlete", "foundational", "", "Build strength", sql.NullInt64{Int64: coach.ID, Valid: true}, true)

	// Seed some data across multiple tables.
	w, _ := CreateWorkout(db, a.ID, "2026-02-10", "Felt strong today")
	e, _ := CreateExercise(db, "Squat", "", "", "", 0)
	e2, _ := CreateExercise(db, "Bench Press", "", "", "", 0)
	AddSet(db, w.ID, e.ID, 5, 225, 0, "reps", "")
	AddSet(db, w.ID, e.ID, 5, 225, 0, "reps", "")
	AddSet(db, w.ID, e2.ID, 8, 135, 0, "reps", "")
	CreateBodyWeight(db, a.ID, "2026-02-11", 185.0, "morning weight")
	RecordGoalChange(db, a.ID, "Build strength", "", coach.ID, "2026-02-01", "")
	RecordTierChange(db, a.ID, "foundational", "", coach.ID, "2026-01-15", "")
	CreateAthleteNote(db, a.ID, coach.ID, "2026-02-12", "Good progress", false, false)
	CreateAthleteNote(db, a.ID, coach.ID, "2026-02-12", "Internal observation", true, false)

	t.Run("includes all event types", func(t *testing.T) {
		entries, err := ListJournalEntries(db, a.ID, true, 100)
		if err != nil {
			t.Fatalf("list journal entries: %v", err)
		}
		// Should have: 1 workout, 1 body_weight, 1 goal_change, 1 tier_change, 2 notes = 6
		if len(entries) < 5 {
			t.Fatalf("len = %d, want at least 5", len(entries))
		}

		types := make(map[string]int)
		for _, e := range entries {
			types[e.Type]++
		}
		if types["workout"] < 1 {
			t.Error("missing workout entries")
		}
		if types["body_weight"] < 1 {
			t.Error("missing body_weight entries")
		}
		if types["goal_change"] < 1 {
			t.Error("missing goal_change entries")
		}
		if types["tier_change"] < 1 {
			t.Error("missing tier_change entries")
		}
		if types["note"] < 1 {
			t.Error("missing note entries")
		}
	})

	t.Run("excludes private notes for non-coach", func(t *testing.T) {
		entries, err := ListJournalEntries(db, a.ID, false, 100)
		if err != nil {
			t.Fatalf("list journal entries: %v", err)
		}

		for _, e := range entries {
			if e.Type == "note" && e.IsPrivate {
				t.Error("private note should not appear when includePrivate=false")
			}
		}

		// Count notes — should only have the public one.
		noteCount := 0
		for _, e := range entries {
			if e.Type == "note" {
				noteCount++
			}
		}
		if noteCount != 1 {
			t.Errorf("note count = %d, want 1 (private excluded)", noteCount)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		entries, err := ListJournalEntries(db, a.ID, true, 2)
		if err != nil {
			t.Fatalf("list journal entries: %v", err)
		}
		if len(entries) > 2 {
			t.Errorf("len = %d, want <= 2", len(entries))
		}
	})

	t.Run("empty for different athlete", func(t *testing.T) {
		a2, _ := CreateAthlete(db, "Other Athlete", "", "", "", sql.NullInt64{}, true)
		entries, err := ListJournalEntries(db, a2.ID, true, 100)
		if err != nil {
			t.Fatalf("list journal entries: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("len = %d, want 0", len(entries))
		}
	})

	t.Run("workout summary includes exercise names", func(t *testing.T) {
		entries, err := ListJournalEntries(db, a.ID, true, 100)
		if err != nil {
			t.Fatalf("list journal entries: %v", err)
		}
		for _, entry := range entries {
			if entry.Type == "workout" {
				if !strings.Contains(entry.Summary, "Squat") {
					t.Errorf("workout summary should contain 'Squat', got %q", entry.Summary)
				}
				if !strings.Contains(entry.Summary, "Bench Press") {
					t.Errorf("workout summary should contain 'Bench Press', got %q", entry.Summary)
				}
				if !strings.Contains(entry.Summary, "3 sets") {
					t.Errorf("workout summary should contain '3 sets', got %q", entry.Summary)
				}
				break
			}
		}
	})

	t.Run("workout detail contains notes", func(t *testing.T) {
		entries, err := ListJournalEntries(db, a.ID, true, 100)
		if err != nil {
			t.Fatalf("list journal entries: %v", err)
		}
		for _, entry := range entries {
			if entry.Type == "workout" {
				if entry.Detail != "Felt strong today" {
					t.Errorf("workout detail = %q, want %q", entry.Detail, "Felt strong today")
				}
				break
			}
		}
	})

	t.Run("body weight detail contains notes", func(t *testing.T) {
		entries, err := ListJournalEntries(db, a.ID, true, 100)
		if err != nil {
			t.Fatalf("list journal entries: %v", err)
		}
		for _, entry := range entries {
			if entry.Type == "body_weight" {
				if entry.Detail != "morning weight" {
					t.Errorf("body_weight detail = %q, want %q", entry.Detail, "morning weight")
				}
				break
			}
		}
	})
}
