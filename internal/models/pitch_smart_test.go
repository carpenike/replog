package models

import (
	"context"
	"database/sql"
	"strings"
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
	a, err := CreateAthlete(context.Background(), db, "Pitcher", "", "", "", dob, "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// 80 pitches on 2026-05-10 → for age 13 (95 max), 66+ band → 4 rest days.
	count := int64(80)
	if _, err := CreateThrowingSession(context.Background(), db, a.ID, ThrowingSessionInput{
		Date: "2026-05-10", ThrowType: "game", ThrowCount: &count,
	}); err != nil {
		t.Fatalf("create throwing session: %v", err)
	}

	// Check one day later: 4 required − 1 elapsed = 3 owed.
	asOf := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	status, err := ComputePitchSmartStatus(context.Background(), db, a.ID, asOf)
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
	status, err = ComputePitchSmartStatus(context.Background(), db, a.ID, rested)
	if err != nil {
		t.Fatalf("compute pitch smart (rested): %v", err)
	}
	if status.RestDaysOwed != 0 {
		t.Errorf("rest days owed=%d after window, want 0", status.RestDaysOwed)
	}
}

// TestComputePitchSmartStatus_NonPitchingThrowDoesNotResetAdvisory is the
// HOF-010 scoping regression: the pitch-count advisory counts mound pitching
// only ('game'/'bullpen'). A later-dated non-pitching throw (here 'position',
// the new infield type) must NOT become the "last counted session" and must
// NOT reset rest-days-owed — even though it still counts toward the broader
// load view (GetLoadSummary, asserted in load_test). The advisory keeps
// pointing at the earlier bullpen session.
func TestComputePitchSmartStatus_NonPitchingThrowDoesNotResetAdvisory(t *testing.T) {
	db := testDB(t)
	dob := "2013-01-01" // age 13 → daily max 95, 66+ band → 4 rest days.
	a, err := CreateAthlete(context.Background(), db, "Two-Way", "", "", "", dob, "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// A bullpen of 80 pitches on 2026-05-10 → 4 rest days required.
	pitches := int64(80)
	if _, err := CreateThrowingSession(context.Background(), db, a.ID, ThrowingSessionInput{
		Date: "2026-05-10", ThrowType: "bullpen", ThrowCount: &pitches,
	}); err != nil {
		t.Fatalf("create bullpen session: %v", err)
	}

	// A LATER-dated position (infield) session of 60 throws on 2026-05-11.
	// This is real arm load, but it is NOT pitching — it must not touch the
	// pitch-count advisory.
	infield := int64(60)
	if _, err := CreateThrowingSession(context.Background(), db, a.ID, ThrowingSessionInput{
		Date: "2026-05-11", ThrowType: "position", ThrowCount: &infield,
	}); err != nil {
		t.Fatalf("create position session: %v", err)
	}

	// As of 2026-05-12: the advisory must reflect the 2026-05-10 BULLPEN, not
	// the later position throw. 4 required − 2 elapsed = 2 owed.
	asOf := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	status, err := ComputePitchSmartStatus(context.Background(), db, a.ID, asOf)
	if err != nil {
		t.Fatalf("compute pitch smart: %v", err)
	}
	if status.LastSessionDate != "2026-05-10" {
		t.Errorf("last session=%q, want 2026-05-10 (the bullpen, not the later position throw)", status.LastSessionDate)
	}
	if status.LastThrowCount != 80 {
		t.Errorf("last throw count=%d, want 80 (bullpen pitches, not the 60 position throws)", status.LastThrowCount)
	}
	if status.RestDaysOwed != 2 {
		t.Errorf("rest days owed=%d, want 2 (the position throw must not reset the rest window)", status.RestDaysOwed)
	}
}

// TestComputePitchSmartStatus_OnlyNonPitchingThrows reports "no counted
// pitching sessions" when an athlete has thrown but never off a mound — the
// advisory is pitch-count-scoped, so catch/position/long_toss don't register.
func TestComputePitchSmartStatus_OnlyNonPitchingThrows(t *testing.T) {
	db := testDB(t)
	a, err := CreateAthlete(context.Background(), db, "Position Only", "", "", "", "2013-01-01", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	count := int64(50)
	if _, err := CreateThrowingSession(context.Background(), db, a.ID, ThrowingSessionInput{
		Date: "2026-05-10", ThrowType: "position", ThrowCount: &count,
	}); err != nil {
		t.Fatalf("create position session: %v", err)
	}

	status, err := ComputePitchSmartStatus(context.Background(), db, a.ID, time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("compute pitch smart: %v", err)
	}
	if status.LastThrowCount != 0 || status.RestDaysOwed != 0 {
		t.Errorf("expected no pitching session to register; got last=%d owed=%d", status.LastThrowCount, status.RestDaysOwed)
	}
	if !strings.Contains(status.Advisory, "No counted pitching sessions") {
		t.Errorf("advisory=%q, want the no-pitching-sessions message", status.Advisory)
	}
}

// TestComputePitchSmartStatus_NoDOB returns ErrNoPitchSmartLimit (treated as
// "no advisory") rather than a hard error when age can't be determined.
func TestComputePitchSmartStatus_NoDOB(t *testing.T) {
	db := testDB(t)
	a, err := CreateAthlete(context.Background(), db, "Unknown Age", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	if _, err := ComputePitchSmartStatus(context.Background(), db, a.ID, time.Now()); err != ErrNoPitchSmartLimit {
		t.Errorf("err=%v, want ErrNoPitchSmartLimit", err)
	}
}
