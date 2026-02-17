package models

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestTrainingMaxCRUD(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "TM Athlete", "", "", "", sql.NullInt64{})
	e, _ := CreateExercise(db, "TM Exercise", "", 0, "", "", 0)

	t.Run("set training max", func(t *testing.T) {
		tm, err := SetTrainingMax(db, a.ID, e.ID, 100.0, "2024-01-01", "")
		if err != nil {
			t.Fatalf("set TM: %v", err)
		}
		if tm.Weight != 100.0 {
			t.Errorf("weight = %f, want 100", tm.Weight)
		}
		if !strings.HasPrefix(tm.EffectiveDate, "2024-01-01") {
			t.Errorf("date = %q, want prefix 2024-01-01", tm.EffectiveDate)
		}
	})

	t.Run("current training max", func(t *testing.T) {
		tm, err := CurrentTrainingMax(db, a.ID, e.ID)
		if err != nil {
			t.Fatalf("current TM: %v", err)
		}
		if tm.Weight != 100.0 {
			t.Errorf("weight = %f, want 100", tm.Weight)
		}
	})

	t.Run("update training max uses latest date", func(t *testing.T) {
		_, err := SetTrainingMax(db, a.ID, e.ID, 110.0, "2024-02-01", "")
		if err != nil {
			t.Fatalf("set TM: %v", err)
		}

		tm, _ := CurrentTrainingMax(db, a.ID, e.ID)
		if tm.Weight != 110.0 {
			t.Errorf("weight = %f, want 110 (latest)", tm.Weight)
		}
	})

	t.Run("history returns all entries", func(t *testing.T) {
		history, err := ListTrainingMaxHistory(db, a.ID, e.ID)
		if err != nil {
			t.Fatalf("list history: %v", err)
		}
		if len(history) != 2 {
			t.Errorf("count = %d, want 2", len(history))
		}
		// Most recent first.
		if history[0].Weight != 110.0 {
			t.Errorf("first entry weight = %f, want 110", history[0].Weight)
		}
	})
}

func TestListCurrentTrainingMaxes(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "TM List Athlete", "", "", "", sql.NullInt64{})
	e1, _ := CreateExercise(db, "TM List Ex 1", "", 0, "", "", 0)
	e2, _ := CreateExercise(db, "TM List Ex 2", "", 0, "", "", 0)

	today := time.Now().Format("2006-01-02")
	SetTrainingMax(db, a.ID, e1.ID, 200.0, today, "")
	SetTrainingMax(db, a.ID, e2.ID, 150.0, today, "")

	maxes, err := ListCurrentTrainingMaxes(db, a.ID)
	if err != nil {
		t.Fatalf("list current TMs: %v", err)
	}
	if len(maxes) != 2 {
		t.Fatalf("count = %d, want 2", len(maxes))
	}
}
