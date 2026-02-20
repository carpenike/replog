package models

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRecordGoalChange(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	t.Run("initial goal", func(t *testing.T) {
		gh, err := RecordGoalChange(db, a.ID, "Build overall strength", "", coach.ID, "", "")
		if err != nil {
			t.Fatalf("record goal change: %v", err)
		}
		if gh.Goal != "Build overall strength" {
			t.Errorf("goal = %q, want %q", gh.Goal, "Build overall strength")
		}
		if gh.PreviousGoal.Valid {
			t.Errorf("expected NULL previous_goal, got %q", gh.PreviousGoal.String)
		}
		if !gh.SetBy.Valid || gh.SetBy.Int64 != coach.ID {
			t.Errorf("set_by = %v, want %d", gh.SetBy, coach.ID)
		}
		if gh.SetByName != "coach" {
			t.Errorf("set_by_name = %q, want %q", gh.SetByName, "coach")
		}
	})

	t.Run("goal change with previous", func(t *testing.T) {
		gh, err := RecordGoalChange(db, a.ID, "Prepare for football season", "Build overall strength", coach.ID, "2026-03-01", "Shifting focus")
		if err != nil {
			t.Fatalf("record goal change: %v", err)
		}
		if gh.Goal != "Prepare for football season" {
			t.Errorf("goal = %q, want %q", gh.Goal, "Prepare for football season")
		}
		if !gh.PreviousGoal.Valid || gh.PreviousGoal.String != "Build overall strength" {
			t.Errorf("previous_goal = %v, want %q", gh.PreviousGoal, "Build overall strength")
		}
		if !strings.HasPrefix(gh.EffectiveDate, "2026-03-01") {
			t.Errorf("effective_date = %q, want prefix %q", gh.EffectiveDate, "2026-03-01")
		}
		if !gh.Notes.Valid || gh.Notes.String != "Shifting focus" {
			t.Errorf("notes = %v, want %q", gh.Notes, "Shifting focus")
		}
	})

	t.Run("default effective date", func(t *testing.T) {
		gh, err := RecordGoalChange(db, a.ID, "Recovery phase", "", coach.ID, "", "")
		if err != nil {
			t.Fatalf("record goal change: %v", err)
		}
		if gh.EffectiveDate == "" {
			t.Error("expected non-empty effective_date")
		}
	})
}

func TestListGoalHistory(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	// Record multiple goal changes.
	RecordGoalChange(db, a.ID, "Goal 1", "", coach.ID, "2026-01-01", "")
	RecordGoalChange(db, a.ID, "Goal 2", "Goal 1", coach.ID, "2026-02-01", "")
	RecordGoalChange(db, a.ID, "Goal 3", "Goal 2", coach.ID, "2026-03-01", "")

	t.Run("returns newest first", func(t *testing.T) {
		history, err := ListGoalHistory(db, a.ID)
		if err != nil {
			t.Fatalf("list goal history: %v", err)
		}
		if len(history) != 3 {
			t.Fatalf("len = %d, want 3", len(history))
		}
		if history[0].Goal != "Goal 3" {
			t.Errorf("first entry goal = %q, want %q", history[0].Goal, "Goal 3")
		}
		if history[2].Goal != "Goal 1" {
			t.Errorf("last entry goal = %q, want %q", history[2].Goal, "Goal 1")
		}
	})

	t.Run("empty for different athlete", func(t *testing.T) {
		a2, _ := CreateAthlete(db, "Other Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
		history, err := ListGoalHistory(db, a2.ID)
		if err != nil {
			t.Fatalf("list goal history: %v", err)
		}
		if len(history) != 0 {
			t.Errorf("len = %d, want 0", len(history))
		}
	})

	t.Run("cascades on athlete delete", func(t *testing.T) {
		a3, _ := CreateAthlete(db, "Delete Me", "", "", "", "", "", "", sql.NullInt64{}, true)
		RecordGoalChange(db, a3.ID, "Temp goal", "", coach.ID, "", "")

		// Delete athlete — goal history should cascade.
		db.Exec("DELETE FROM athletes WHERE id = ?", a3.ID)

		history, err := ListGoalHistory(db, a3.ID)
		if err != nil {
			t.Fatalf("list goal history after delete: %v", err)
		}
		if len(history) != 0 {
			t.Errorf("len = %d, want 0 after cascade delete", len(history))
		}
	})
}
