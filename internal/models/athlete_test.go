package models

import (
	"testing"
)

func TestCreateAthlete(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		a, err := CreateAthlete(db, "Alice", "foundational", "test notes")
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
	})

	t.Run("nullable tier and notes", func(t *testing.T) {
		a, err := CreateAthlete(db, "Bob", "", "")
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

	created, err := CreateAthlete(db, "Charlie", "", "")
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

	a, err := CreateAthlete(db, "Dave", "foundational", "")
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	t.Run("update name and tier", func(t *testing.T) {
		updated, err := UpdateAthlete(db, a.ID, "David", "intermediate", "promoted")
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

	t.Run("not found", func(t *testing.T) {
		_, err := UpdateAthlete(db, 99999, "Nobody", "", "")
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteAthlete(t *testing.T) {
	db := testDB(t)

	a, err := CreateAthlete(db, "Eve", "", "")
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
	athletes, err := ListAthletes(db)
	if err != nil {
		t.Fatalf("list athletes: %v", err)
	}
	if len(athletes) != 0 {
		t.Fatalf("expected 0 athletes, got %d", len(athletes))
	}

	CreateAthlete(db, "Zara", "", "")
	CreateAthlete(db, "Aaron", "", "")

	athletes, err = ListAthletes(db)
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
