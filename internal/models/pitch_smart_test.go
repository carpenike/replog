package models

import (
	"database/sql"
	"testing"
	"time"
)

// TestComputePitchSmartStatus_RestDaysOwed verifies the read-only advisory math:
// a high-volume session for a 13-year-old (daily max 95, 66+ pitches → 4 rest
// days) leaves rest days owed when checked the next day, and computes the
// recommended next-eligible date. This is guidance only; nothing is written.
func TestComputePitchSmartStatus_RestDaysOwed(t *testing.T) {
	db := testDB(t)
	dob := "2013-01-01"
	a, err := CreateAthlete(db, "Pitcher", "", "", "", dob, "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// 80 pitches on 2026-05-10 → for age 13 (95 max), 66+ band → 4 rest days.
	count := int64(80)
	if _, err := CreateThrowingSession(db, a.ID, ThrowingSessionInput{
		Date: "2026-05-10", ThrowType: "game", ThrowCount: &count,
	}); err != nil {
		t.Fatalf("create throwing session: %v", err)
	}

	// Check one day later: 4 required − 1 elapsed = 3 owed.
	asOf := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	status, err := ComputePitchSmartStatus(db, a.ID, asOf)
	if err != nil {
		t.Fatalf("compute pitch smart: %v", err)
	}
	if status.DailyMax != 95 {
		t.Errorf("daily max=%d, want 95", status.DailyMax)
	}
	if status.RestDaysRequired != 4 {
		t.Errorf("rest days required=%d, want 4", status.RestDaysRequired)
	}
	if status.RestDaysOwed != 3 {
		t.Errorf("rest days owed=%d, want 3", status.RestDaysOwed)
	}
	if status.NextEligibleDate != "2026-05-14" {
		t.Errorf("next eligible=%q, want 2026-05-14", status.NextEligibleDate)
	}
	if status.OverDailyMax {
		t.Error("80 pitches should not exceed the 95 daily max")
	}

	// After the rest window, nothing is owed.
	rested := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	status, err = ComputePitchSmartStatus(db, a.ID, rested)
	if err != nil {
		t.Fatalf("compute pitch smart (rested): %v", err)
	}
	if status.RestDaysOwed != 0 {
		t.Errorf("rest days owed=%d after window, want 0", status.RestDaysOwed)
	}
}

// TestComputePitchSmartStatus_NoDOB returns ErrNoPitchSmartLimit (treated as
// "no advisory") rather than a hard error when age can't be determined.
func TestComputePitchSmartStatus_NoDOB(t *testing.T) {
	db := testDB(t)
	a, err := CreateAthlete(db, "Unknown Age", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	if _, err := ComputePitchSmartStatus(db, a.ID, time.Now()); err != ErrNoPitchSmartLimit {
		t.Errorf("err=%v, want ErrNoPitchSmartLimit", err)
	}
}
