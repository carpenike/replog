package models

import (
	"database/sql"
	"strings"
	"testing"
)

func TestCreateBodyWeight(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", sql.NullInt64{})

	t.Run("basic create", func(t *testing.T) {
		bw, err := CreateBodyWeight(db, a.ID, "2026-02-01", 185.5, "morning weigh-in")
		if err != nil {
			t.Fatalf("create body weight: %v", err)
		}
		if bw.AthleteID != a.ID {
			t.Errorf("athlete_id = %d, want %d", bw.AthleteID, a.ID)
		}
		if bw.Weight != 185.5 {
			t.Errorf("weight = %.1f, want 185.5", bw.Weight)
		}
		if !bw.Notes.Valid || bw.Notes.String != "morning weigh-in" {
			t.Errorf("notes = %v, want 'morning weigh-in'", bw.Notes)
		}
	})

	t.Run("duplicate date", func(t *testing.T) {
		_, err := CreateBodyWeight(db, a.ID, "2026-02-01", 186.0, "")
		if err != ErrDuplicateBodyWeight {
			t.Errorf("err = %v, want ErrDuplicateBodyWeight", err)
		}
	})

	t.Run("different athlete same date", func(t *testing.T) {
		a2, _ := CreateAthlete(db, "Another Athlete", "", "", sql.NullInt64{})
		bw, err := CreateBodyWeight(db, a2.ID, "2026-02-01", 150.0, "")
		if err != nil {
			t.Fatalf("create body weight for different athlete: %v", err)
		}
		if bw.Weight != 150.0 {
			t.Errorf("weight = %.1f, want 150.0", bw.Weight)
		}
	})

	t.Run("empty notes", func(t *testing.T) {
		bw, err := CreateBodyWeight(db, a.ID, "2026-02-02", 184.0, "")
		if err != nil {
			t.Fatalf("create body weight: %v", err)
		}
		if bw.Notes.Valid {
			t.Errorf("expected NULL notes, got %v", bw.Notes)
		}
	})
}

func TestGetBodyWeightByID(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", sql.NullInt64{})

	t.Run("found", func(t *testing.T) {
		created, _ := CreateBodyWeight(db, a.ID, "2026-02-01", 185.0, "")
		bw, err := GetBodyWeightByID(db, created.ID)
		if err != nil {
			t.Fatalf("get body weight: %v", err)
		}
		if bw.Weight != 185.0 {
			t.Errorf("weight = %.1f, want 185.0", bw.Weight)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetBodyWeightByID(db, 9999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteBodyWeight(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", sql.NullInt64{})

	t.Run("delete existing", func(t *testing.T) {
		bw, _ := CreateBodyWeight(db, a.ID, "2026-02-01", 185.0, "")
		if err := DeleteBodyWeight(db, bw.ID); err != nil {
			t.Fatalf("delete body weight: %v", err)
		}
		_, err := GetBodyWeightByID(db, bw.ID)
		if err != ErrNotFound {
			t.Errorf("after delete, err = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete nonexistent", func(t *testing.T) {
		err := DeleteBodyWeight(db, 9999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestListBodyWeights(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", sql.NullInt64{})

	t.Run("empty list", func(t *testing.T) {
		page, err := ListBodyWeights(db, a.ID, 0)
		if err != nil {
			t.Fatalf("list body weights: %v", err)
		}
		if len(page.Entries) != 0 {
			t.Errorf("entries = %d, want 0", len(page.Entries))
		}
		if page.HasMore {
			t.Error("expected HasMore = false")
		}
	})

	t.Run("ordered by date descending", func(t *testing.T) {
		CreateBodyWeight(db, a.ID, "2026-02-01", 185.0, "")
		CreateBodyWeight(db, a.ID, "2026-02-03", 184.0, "")
		CreateBodyWeight(db, a.ID, "2026-02-02", 184.5, "")

		page, err := ListBodyWeights(db, a.ID, 0)
		if err != nil {
			t.Fatalf("list body weights: %v", err)
		}
		if len(page.Entries) != 3 {
			t.Fatalf("entries = %d, want 3", len(page.Entries))
		}
		// Most recent first.
		if !strings.HasPrefix(page.Entries[0].Date, "2026-02-03") {
			t.Errorf("first entry date = %s, want prefix 2026-02-03", page.Entries[0].Date)
		}
		if !strings.HasPrefix(page.Entries[2].Date, "2026-02-01") {
			t.Errorf("last entry date = %s, want prefix 2026-02-01", page.Entries[2].Date)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		a2, _ := CreateAthlete(db, "Paginator", "", "", sql.NullInt64{})
		// Insert more than one page of entries.
		for i := 1; i <= BodyWeightPageSize+5; i++ {
			date := mustParseDate("2026-01-01").AddDate(0, 0, i).Format("2006-01-02")
			CreateBodyWeight(db, a2.ID, date, float64(180+i%5), "")
		}

		page, err := ListBodyWeights(db, a2.ID, 0)
		if err != nil {
			t.Fatalf("list page 1: %v", err)
		}
		if len(page.Entries) != BodyWeightPageSize {
			t.Errorf("page 1 entries = %d, want %d", len(page.Entries), BodyWeightPageSize)
		}
		if !page.HasMore {
			t.Error("expected HasMore = true on first page")
		}

		page2, err := ListBodyWeights(db, a2.ID, BodyWeightPageSize)
		if err != nil {
			t.Fatalf("list page 2: %v", err)
		}
		if len(page2.Entries) != 5 {
			t.Errorf("page 2 entries = %d, want 5", len(page2.Entries))
		}
		if page2.HasMore {
			t.Error("expected HasMore = false on last page")
		}
	})
}

func TestLatestBodyWeight(t *testing.T) {
	db := testDB(t)
	a, _ := CreateAthlete(db, "Test Athlete", "", "", sql.NullInt64{})

	t.Run("no entries", func(t *testing.T) {
		bw, err := LatestBodyWeight(db, a.ID)
		if err != nil {
			t.Fatalf("latest body weight: %v", err)
		}
		if bw != nil {
			t.Error("expected nil for athlete with no entries")
		}
	})

	t.Run("returns most recent", func(t *testing.T) {
		CreateBodyWeight(db, a.ID, "2026-02-01", 185.0, "")
		CreateBodyWeight(db, a.ID, "2026-02-03", 183.0, "latest")
		CreateBodyWeight(db, a.ID, "2026-02-02", 184.0, "")

		bw, err := LatestBodyWeight(db, a.ID)
		if err != nil {
			t.Fatalf("latest body weight: %v", err)
		}
		if bw == nil {
			t.Fatal("expected non-nil body weight")
		}
		if !strings.HasPrefix(bw.Date, "2026-02-03") {
			t.Errorf("date = %s, want prefix 2026-02-03", bw.Date)
		}
		if bw.Weight != 183.0 {
			t.Errorf("weight = %.1f, want 183.0", bw.Weight)
		}
	})
}
