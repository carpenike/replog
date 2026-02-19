package models

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRecordTierChange(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "foundational", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	t.Run("initial tier", func(t *testing.T) {
		th, err := RecordTierChange(db, a.ID, "foundational", "", coach.ID, "", "")
		if err != nil {
			t.Fatalf("record tier change: %v", err)
		}
		if th.Tier != "foundational" {
			t.Errorf("tier = %q, want %q", th.Tier, "foundational")
		}
		if th.PreviousTier.Valid {
			t.Errorf("expected NULL previous_tier, got %q", th.PreviousTier.String)
		}
		if !th.SetBy.Valid || th.SetBy.Int64 != coach.ID {
			t.Errorf("set_by = %v, want %d", th.SetBy, coach.ID)
		}
	})

	t.Run("tier promotion", func(t *testing.T) {
		th, err := RecordTierChange(db, a.ID, "intermediate", "foundational", coach.ID, "2026-03-01", "Promoted")
		if err != nil {
			t.Fatalf("record tier change: %v", err)
		}
		if th.Tier != "intermediate" {
			t.Errorf("tier = %q, want %q", th.Tier, "intermediate")
		}
		if !th.PreviousTier.Valid || th.PreviousTier.String != "foundational" {
			t.Errorf("previous_tier = %v, want %q", th.PreviousTier, "foundational")
		}
		if !strings.HasPrefix(th.EffectiveDate, "2026-03-01") {
			t.Errorf("effective_date = %q, want prefix %q", th.EffectiveDate, "2026-03-01")
		}
		if !th.Notes.Valid || th.Notes.String != "Promoted" {
			t.Errorf("notes = %v, want %q", th.Notes, "Promoted")
		}
	})

	t.Run("default effective date", func(t *testing.T) {
		th, err := RecordTierChange(db, a.ID, "sport_performance", "intermediate", coach.ID, "", "")
		if err != nil {
			t.Fatalf("record tier change: %v", err)
		}
		if th.EffectiveDate == "" {
			t.Error("expected non-empty effective_date")
		}
	})
}

func TestListTierHistory(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "foundational", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	RecordTierChange(db, a.ID, "foundational", "", coach.ID, "2026-01-01", "")
	RecordTierChange(db, a.ID, "intermediate", "foundational", coach.ID, "2026-02-01", "")
	RecordTierChange(db, a.ID, "sport_performance", "intermediate", coach.ID, "2026-03-01", "")

	t.Run("returns newest first", func(t *testing.T) {
		history, err := ListTierHistory(db, a.ID)
		if err != nil {
			t.Fatalf("list tier history: %v", err)
		}
		if len(history) != 3 {
			t.Fatalf("len = %d, want 3", len(history))
		}
		if history[0].Tier != "sport_performance" {
			t.Errorf("first entry tier = %q, want %q", history[0].Tier, "sport_performance")
		}
		if history[2].Tier != "foundational" {
			t.Errorf("last entry tier = %q, want %q", history[2].Tier, "foundational")
		}
	})

	t.Run("cascades on athlete delete", func(t *testing.T) {
		a2, _ := CreateAthlete(db, "Delete Me", "foundational", "", "", sql.NullInt64{}, true)
		RecordTierChange(db, a2.ID, "foundational", "", coach.ID, "", "")
		db.Exec("DELETE FROM athletes WHERE id = ?", a2.ID)

		history, err := ListTierHistory(db, a2.ID)
		if err != nil {
			t.Fatalf("list tier history after delete: %v", err)
		}
		if len(history) != 0 {
			t.Errorf("len = %d, want 0 after cascade delete", len(history))
		}
	})
}
