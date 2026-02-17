package models

import (
	"database/sql"
	"testing"
)

func TestCreateAthlete(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		a, err := CreateAthlete(db, "Alice", "foundational", "test notes", sql.NullInt64{})
		if err != nil {
			t.Fatalf("create athlete: %v", err)
		}
		if a.Name != "Alice" {
			t.Errorf("name = %q, want Alice", a.Name)
		}
		if !a.Tier.Valid || a.Tier.String != "foundational" {
			t.Errorf("tier = %v, want foundational", a.Tier)
		}
		if !a.Notes.Valid || a.Notes.String != "test notes" {
			t.Errorf("notes = %v, want test notes", a.Notes)
		}
		if !a.TrackBodyWeight {
			t.Error("TrackBodyWeight should default to true")
		}
	})

	t.Run("nullable tier and notes", func(t *testing.T) {
		a, err := CreateAthlete(db, "Bob", "", "", sql.NullInt64{})
		if err != nil {
			t.Fatalf("create athlete: %v", err)
		}
		if a.Tier.Valid {
			t.Errorf("tier should be null, got %q", a.Tier.String)
		}
		if a.Notes.Valid {
			t.Errorf("notes should be null, got %q", a.Notes.String)
		}
	})
}

func TestGetAthleteByID(t *testing.T) {
	db := testDB(t)

	created, err := CreateAthlete(db, "Charlie", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	t.Run("found", func(t *testing.T) {
		a, err := GetAthleteByID(db, created.ID)
		if err != nil {
			t.Fatalf("get athlete: %v", err)
		}
		if a.Name != "Charlie" {
			t.Errorf("name = %q, want Charlie", a.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetAthleteByID(db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestUpdateAthlete(t *testing.T) {
	db := testDB(t)

	a, err := CreateAthlete(db, "Dave", "foundational", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	t.Run("update name and tier", func(t *testing.T) {
		updated, err := UpdateAthlete(db, a.ID, "David", "intermediate", "promoted", sql.NullInt64{}, true)
		if err != nil {
			t.Fatalf("update athlete: %v", err)
		}
		if updated.Name != "David" {
			t.Errorf("name = %q, want David", updated.Name)
		}
		if !updated.Tier.Valid || updated.Tier.String != "intermediate" {
			t.Errorf("tier = %v, want intermediate", updated.Tier)
		}
	})

	t.Run("disable body weight tracking", func(t *testing.T) {
		updated, err := UpdateAthlete(db, a.ID, "David", "intermediate", "", sql.NullInt64{}, false)
		if err != nil {
			t.Fatalf("update athlete: %v", err)
		}
		if updated.TrackBodyWeight {
			t.Error("TrackBodyWeight should be false after update")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := UpdateAthlete(db, 99999, "Nobody", "", "", sql.NullInt64{}, true)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteAthlete(t *testing.T) {
	db := testDB(t)

	a, err := CreateAthlete(db, "Eve", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	t.Run("delete existing", func(t *testing.T) {
		if err := DeleteAthlete(db, a.ID); err != nil {
			t.Fatalf("delete athlete: %v", err)
		}
		_, err := GetAthleteByID(db, a.ID)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("delete non-existent", func(t *testing.T) {
		if err := DeleteAthlete(db, 99999); err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestListAthletes(t *testing.T) {
	db := testDB(t)

	// Start with empty list.
	athletes, err := ListAthletes(db, sql.NullInt64{})
	if err != nil {
		t.Fatalf("list athletes: %v", err)
	}
	if len(athletes) != 0 {
		t.Fatalf("expected 0 athletes, got %d", len(athletes))
	}

	CreateAthlete(db, "Zara", "", "", sql.NullInt64{})
	CreateAthlete(db, "Aaron", "", "", sql.NullInt64{})

	athletes, err = ListAthletes(db, sql.NullInt64{})
	if err != nil {
		t.Fatalf("list athletes: %v", err)
	}
	if len(athletes) != 2 {
		t.Fatalf("expected 2 athletes, got %d", len(athletes))
	}
	// Should be sorted by name (case-insensitive).
	if athletes[0].Name != "Aaron" {
		t.Errorf("first athlete = %q, want Aaron", athletes[0].Name)
	}
}

func TestNextTier(t *testing.T) {
	tests := []struct {
		current  string
		wantNext string
		wantOK   bool
	}{
		{"foundational", "intermediate", true},
		{"intermediate", "sport_performance", true},
		{"sport_performance", "", false},
		{"", "", false},
		{"unknown", "", false},
	}
	for _, tt := range tests {
		next, ok := NextTier(tt.current)
		if next != tt.wantNext || ok != tt.wantOK {
			t.Errorf("NextTier(%q) = (%q, %v), want (%q, %v)", tt.current, next, ok, tt.wantNext, tt.wantOK)
		}
	}
}

func TestPromoteAthlete(t *testing.T) {
	db := testDB(t)

	t.Run("promote foundational to intermediate", func(t *testing.T) {
		a, err := CreateAthlete(db, "PromoKid", "foundational", "", sql.NullInt64{})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		promoted, err := PromoteAthlete(db, a.ID)
		if err != nil {
			t.Fatalf("promote: %v", err)
		}
		if !promoted.Tier.Valid || promoted.Tier.String != "intermediate" {
			t.Errorf("tier = %v, want intermediate", promoted.Tier)
		}
	})

	t.Run("promote intermediate to sport_performance", func(t *testing.T) {
		a, err := CreateAthlete(db, "PromoKid2", "intermediate", "", sql.NullInt64{})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		promoted, err := PromoteAthlete(db, a.ID)
		if err != nil {
			t.Fatalf("promote: %v", err)
		}
		if !promoted.Tier.Valid || promoted.Tier.String != "sport_performance" {
			t.Errorf("tier = %v, want sport_performance", promoted.Tier)
		}
	})

	t.Run("already at highest tier", func(t *testing.T) {
		a, err := CreateAthlete(db, "MaxTier", "sport_performance", "", sql.NullInt64{})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		_, err = PromoteAthlete(db, a.ID)
		if err == nil {
			t.Fatal("expected error for highest tier, got nil")
		}
	})

	t.Run("no tier set", func(t *testing.T) {
		a, err := CreateAthlete(db, "NoTier", "", "", sql.NullInt64{})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		_, err = PromoteAthlete(db, a.ID)
		if err == nil {
			t.Fatal("expected error for no tier, got nil")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := PromoteAthlete(db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestListAvailableAthletes(t *testing.T) {
	db := testDB(t)

	// Create 3 athletes.
	a1, _ := CreateAthlete(db, "Alice", "", "", sql.NullInt64{})
	a2, _ := CreateAthlete(db, "Bob", "", "", sql.NullInt64{})
	a3, _ := CreateAthlete(db, "Charlie", "", "", sql.NullInt64{})

	// Link Alice to a user.
	CreateUser(db, "alice_user", "", "password123", "", false, false, sql.NullInt64{Int64: a1.ID, Valid: true})

	t.Run("excludes linked athletes", func(t *testing.T) {
		athletes, err := ListAvailableAthletes(db, 0)
		if err != nil {
			t.Fatalf("list available: %v", err)
		}
		for _, a := range athletes {
			if a.ID == a1.ID {
				t.Errorf("linked athlete %d (Alice) should not appear", a1.ID)
			}
		}
		if len(athletes) != 2 {
			t.Errorf("expected 2 available athletes, got %d", len(athletes))
		}
	})

	t.Run("except preserves current link", func(t *testing.T) {
		athletes, err := ListAvailableAthletes(db, a1.ID)
		if err != nil {
			t.Fatalf("list available: %v", err)
		}
		if len(athletes) != 3 {
			t.Errorf("expected 3 athletes (including excepted), got %d", len(athletes))
		}
		found := false
		for _, a := range athletes {
			if a.ID == a1.ID {
				found = true
			}
		}
		if !found {
			t.Error("excepted athlete Alice should appear")
		}
	})

	_ = a2
	_ = a3
}
