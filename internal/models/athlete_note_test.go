package models

import (
	"database/sql"
	"testing"
)

func TestCreateAthleteNote(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	t.Run("basic note", func(t *testing.T) {
		note, err := CreateAthleteNote(db, a.ID, coach.ID, "", "Great session today", false, false)
		if err != nil {
			t.Fatalf("create note: %v", err)
		}
		if note.Content != "Great session today" {
			t.Errorf("content = %q, want %q", note.Content, "Great session today")
		}
		if note.IsPrivate {
			t.Error("expected public note")
		}
		if note.Pinned {
			t.Error("expected unpinned note")
		}
		if note.AuthorName != "coach" {
			t.Errorf("author_name = %q, want %q", note.AuthorName, "coach")
		}
	})

	t.Run("private pinned note", func(t *testing.T) {
		note, err := CreateAthleteNote(db, a.ID, coach.ID, "2026-03-15", "Watch for overtraining", true, true)
		if err != nil {
			t.Fatalf("create note: %v", err)
		}
		if !note.IsPrivate {
			t.Error("expected private note")
		}
		if !note.Pinned {
			t.Error("expected pinned note")
		}
	})

	t.Run("empty content rejected", func(t *testing.T) {
		_, err := CreateAthleteNote(db, a.ID, coach.ID, "", "", false, false)
		if err == nil {
			t.Fatal("expected error for empty content")
		}
	})
}

func TestUpdateAthleteNote(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	note, _ := CreateAthleteNote(db, a.ID, coach.ID, "", "Original note", false, false)

	t.Run("update content and visibility", func(t *testing.T) {
		updated, err := UpdateAthleteNote(db, note.ID, "Updated note", true, true)
		if err != nil {
			t.Fatalf("update note: %v", err)
		}
		if updated.Content != "Updated note" {
			t.Errorf("content = %q, want %q", updated.Content, "Updated note")
		}
		if !updated.IsPrivate {
			t.Error("expected private after update")
		}
		if !updated.Pinned {
			t.Error("expected pinned after update")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := UpdateAthleteNote(db, 99999, "anything", false, false)
		if err == nil {
			t.Fatal("expected error for non-existent note")
		}
	})
}

func TestDeleteAthleteNote(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	note, _ := CreateAthleteNote(db, a.ID, coach.ID, "", "Delete me", false, false)

	if err := DeleteAthleteNote(db, note.ID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	// Should not exist anymore.
	_, err := GetAthleteNoteByID(db, note.ID)
	if err == nil {
		t.Error("expected not found after delete")
	}
}

func TestListAthleteNotes(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach, _ := CreateUser(db, "coach", "", "password123", "", true, false, sql.NullInt64{})

	CreateAthleteNote(db, a.ID, coach.ID, "2026-01-01", "Public note", false, false)
	CreateAthleteNote(db, a.ID, coach.ID, "2026-01-02", "Private note", true, false)
	CreateAthleteNote(db, a.ID, coach.ID, "2026-01-03", "Pinned note", false, true)

	t.Run("include private", func(t *testing.T) {
		notes, err := ListAthleteNotes(db, a.ID, true)
		if err != nil {
			t.Fatalf("list notes: %v", err)
		}
		if len(notes) != 3 {
			t.Fatalf("len = %d, want 3", len(notes))
		}
		// Pinned note should be first.
		if !notes[0].Pinned {
			t.Error("expected pinned note first")
		}
	})

	t.Run("exclude private", func(t *testing.T) {
		notes, err := ListAthleteNotes(db, a.ID, false)
		if err != nil {
			t.Fatalf("list notes: %v", err)
		}
		if len(notes) != 2 {
			t.Fatalf("len = %d, want 2 (private excluded)", len(notes))
		}
		for _, n := range notes {
			if n.IsPrivate {
				t.Error("private note should not appear")
			}
		}
	})

	t.Run("cascades on athlete delete", func(t *testing.T) {
		a2, _ := CreateAthlete(db, "Delete Me", "", "", "", "", "", "", sql.NullInt64{}, true)
		CreateAthleteNote(db, a2.ID, coach.ID, "", "Temp note", false, false)
		db.Exec("DELETE FROM athletes WHERE id = ?", a2.ID)

		notes, err := ListAthleteNotes(db, a2.ID, true)
		if err != nil {
			t.Fatalf("list notes after delete: %v", err)
		}
		if len(notes) != 0 {
			t.Errorf("len = %d, want 0 after cascade delete", len(notes))
		}
	})
}
