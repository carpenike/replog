package models

import (
	"database/sql"
	"testing"
	"time"
)

// daysAgo returns a YYYY-MM-DD date n days before today, matching the date
// format the load windows compare against.
func daysAgo(n int) string {
	return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
}

func conditioningDiscipline(t *testing.T, s *LoadSummary) *DisciplineLoad {
	t.Helper()
	for _, d := range s.Disciplines {
		if d.Discipline == "conditioning" {
			return d
		}
	}
	t.Fatal("conditioning discipline missing from load summary")
	return nil
}

// TestLoadSummary_WeekOfDataSuppressesACWR is the core guard: with only a week
// of logged history, the chronic window has not yet filled, so the coupled
// ACWR would pin to a false ~4.0 (chronic28 ≈ acute7). The model must instead
// suppress the ratio (nil) and flag "insufficient_history".
func TestLoadSummary_WeekOfDataSuppressesACWR(t *testing.T) {
	db := testDB(t)
	a, err := CreateAthlete(db, "Week Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// Log conditioning on three days, all inside the last 7 days.
	for _, n := range []int{0, 2, 5} {
		dur := int64(1800)
		_, err := CreateConditioningSession(db, a.ID, ConditioningSessionInput{
			Date:            daysAgo(n),
			Modality:        "run",
			SessionType:     "steady",
			DurationSeconds: &dur,
		})
		if err != nil {
			t.Fatalf("create conditioning (%d days ago): %v", n, err)
		}
	}

	summary, err := GetLoadSummary(db, a.ID)
	if err != nil {
		t.Fatalf("get load summary: %v", err)
	}
	cond := conditioningDiscipline(t, summary)

	if cond.ACWR != nil {
		t.Errorf("ACWR = %v with only a week of data; want nil (false ~4.0 must be suppressed)", *cond.ACWR)
	}
	if cond.Marker != "insufficient_history" {
		t.Errorf("Marker = %q; want insufficient_history", cond.Marker)
	}
	// Sanity: the acute total is still reported even while the ratio is suppressed.
	if cond.Acute7 == 0 {
		t.Error("Acute7 = 0; expected the week's logged load to be summed")
	}
}

// TestLoadSummary_FullChronicWindowComputesACWR confirms that once the
// discipline's history spans the full ~28-day chronic window, the coupled ACWR
// is computed instead of suppressed.
func TestLoadSummary_FullChronicWindowComputesACWR(t *testing.T) {
	db := testDB(t)
	a, err := CreateAthlete(db, "Month Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// Log conditioning across the full chronic window: an old anchor at the
	// far edge plus recent sessions inside the acute window.
	for _, n := range []int{27, 20, 12, 4, 1} {
		dur := int64(1800)
		_, err := CreateConditioningSession(db, a.ID, ConditioningSessionInput{
			Date:            daysAgo(n),
			Modality:        "run",
			SessionType:     "steady",
			DurationSeconds: &dur,
		})
		if err != nil {
			t.Fatalf("create conditioning (%d days ago): %v", n, err)
		}
	}

	summary, err := GetLoadSummary(db, a.ID)
	if err != nil {
		t.Fatalf("get load summary: %v", err)
	}
	cond := conditioningDiscipline(t, summary)

	if cond.ACWR == nil {
		t.Fatal("ACWR = nil with a full 28-day window; want a computed ratio")
	}
	if cond.Marker != "" {
		t.Errorf("Marker = %q; want empty once history spans the chronic window", cond.Marker)
	}
}
