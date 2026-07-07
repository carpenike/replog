package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// TestMultiModalP2_FiveDisciplineSameDay verifies that an athlete can log all
// five disciplines — resistance, throwing, conditioning, skill, recovery — on
// the same date. The widened UNIQUE(athlete_id, date, discipline) must allow
// one session per discipline per day.
func TestMultiModalP2_FiveDisciplineSameDay(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	const date = "2026-05-12"

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts", athlete.ID),
		fmt.Sprintf(`{"date":%q}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/throwing-sessions", athlete.ID),
		fmt.Sprintf(`{"date":%q,"throw_type":"bullpen","throw_count":30}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/conditioning-sessions", athlete.ID),
		fmt.Sprintf(`{"date":%q,"modality":"run","session_type":"steady","duration_seconds":1800}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/skill-sessions", athlete.ID),
		fmt.Sprintf(`{"date":%q,"skill_type":"batting","rep_count":50,"load_kg":2}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)

	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/recovery-checkins", athlete.ID),
		fmt.Sprintf(`{"date":%q,"sleep_hours":8.5,"soreness":3,"energy":7}`, date), cookies)
	requireStatus(t, rr, http.StatusCreated)

	// The journal should render all five as distinct, correctly-typed entries.
	entries, err := models.ListJournalEntries(context.Background(), env.DB, athlete.ID, true, 100)
	if err != nil {
		t.Fatalf("list journal: %v", err)
	}
	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Type]++
	}
	for _, typ := range []string{"workout", "throwing", "conditioning", "skill", "recovery"} {
		if counts[typ] != 1 {
			t.Errorf("journal type %q count=%d, want 1", typ, counts[typ])
		}
	}
}

// TestMultiModalP2_ConditioningIntervalsPersist verifies interval child rows are
// written with their session and read back ordered by interval_number.
func TestMultiModalP2_ConditioningIntervalsPersist(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body := `{
		"date":"2026-05-12",
		"modality":"run",
		"session_type":"interval",
		"intervals":[
			{"interval_number":2,"work_seconds":90,"rest_seconds":60},
			{"interval_number":1,"work_seconds":120,"rest_seconds":90},
			{"interval_number":3,"work_seconds":75,"rest_seconds":45}
		]
	}`
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/conditioning-sessions", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var session ConditioningSession
	decodeJSON(t, rr, &session)
	if len(session.Intervals) != 3 {
		t.Fatalf("got %d intervals, want 3", len(session.Intervals))
	}
	// Must be ordered by interval_number ascending.
	for i, iv := range session.Intervals {
		want := int64(i + 1)
		if iv.IntervalNumber != want {
			t.Errorf("interval[%d].interval_number=%d, want %d", i, iv.IntervalNumber, want)
		}
	}
	if session.Intervals[0].WorkSeconds == nil || *session.Intervals[0].WorkSeconds != 120 {
		t.Errorf("interval 1 work_seconds mismatch: %+v", session.Intervals[0].WorkSeconds)
	}
}

// TestMultiModalP2_SkillRecordsLoadKg verifies a skill session records its
// youth-safety load_kg datum.
func TestMultiModalP2_SkillRecordsLoadKg(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/skill-sessions", athlete.ID),
		`{"date":"2026-05-12","skill_type":"medball","rep_count":20,"load_kg":3.5}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var ss SkillSession
	decodeJSON(t, rr, &ss)
	if ss.LoadKg == nil || *ss.LoadKg != 3.5 {
		t.Errorf("load_kg=%v, want 3.5", ss.LoadKg)
	}
	if ss.RepCount == nil || *ss.RepCount != 20 {
		t.Errorf("rep_count=%v, want 20", ss.RepCount)
	}
}

// TestMultiModalP2_RecoveryRecordsCheckin verifies a recovery check-in records
// sleep/soreness/energy.
func TestMultiModalP2_RecoveryRecordsCheckin(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/recovery-checkins", athlete.ID),
		`{"date":"2026-05-12","sleep_hours":7.5,"soreness":4,"energy":6}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var rc RecoveryCheckin
	decodeJSON(t, rr, &rc)
	if rc.SleepHours == nil || *rc.SleepHours != 7.5 {
		t.Errorf("sleep_hours=%v, want 7.5", rc.SleepHours)
	}
	if rc.Soreness == nil || *rc.Soreness != 4 {
		t.Errorf("soreness=%v, want 4", rc.Soreness)
	}
	if rc.Energy == nil || *rc.Energy != 6 {
		t.Errorf("energy=%v, want 6", rc.Energy)
	}
}

// TestMultiModalP2_LoadColdStart verifies that an athlete with no chronic
// history gets a nil ACWR and an "insufficient_history" marker per discipline,
// not a divide-by-zero or a fabricated ratio.
func TestMultiModalP2_LoadColdStart(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/load", athlete.ID), "", cookies)
	requireStatus(t, rr, http.StatusOK)
	var load LoadSummary
	decodeJSON(t, rr, &load)
	if len(load.Disciplines) == 0 {
		t.Fatal("expected per-discipline rows even with no history")
	}
	for _, d := range load.Disciplines {
		if d.ACWR != nil {
			t.Errorf("discipline %q ACWR=%v, want nil on cold start", d.Discipline, *d.ACWR)
		}
		if d.Marker != "insufficient_history" {
			t.Errorf("discipline %q marker=%q, want insufficient_history", d.Discipline, d.Marker)
		}
	}
}

// TestMultiModalP2_LoadIsReadOnly is the zero-write guarantee: a GET /load call
// must not insert, update, or delete any row. Snapshot every relevant table's
// row count before and after the call; they must match exactly.
func TestMultiModalP2_LoadIsReadOnly(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	// Seed one session of each discipline so the load query has data to read.
	const date = "2026-05-12"
	mustCreate := func(path, body string) {
		t.Helper()
		rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/%s", athlete.ID, path), body, cookies)
		requireStatus(t, rr, http.StatusCreated)
	}
	lift := env.createWorkout(t, athlete.ID, date)
	exercise := env.createExercise(t, "Bench Press")
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", athlete.ID, lift.ID),
		fmt.Sprintf(`{"exercise_id":%d,"reps":5,"weight":135}`, exercise.ID), cookies)
	requireStatus(t, rr, http.StatusCreated)
	mustCreate("throwing-sessions", fmt.Sprintf(`{"date":%q,"throw_type":"bullpen","throw_count":30}`, date))
	mustCreate("conditioning-sessions", fmt.Sprintf(`{"date":%q,"modality":"run","session_type":"interval","duration_seconds":1500,"intervals":[{"interval_number":1,"work_seconds":90}]}`, date))
	mustCreate("skill-sessions", fmt.Sprintf(`{"date":%q,"skill_type":"batting","rep_count":40}`, date))
	mustCreate("recovery-checkins", fmt.Sprintf(`{"date":%q,"sleep_hours":8,"soreness":2,"energy":8}`, date))

	tables := []string{
		"workouts", "workout_sets", "throwing_sessions",
		"conditioning_sessions", "conditioning_intervals",
		"skill_sessions", "recovery_checkins",
	}
	before := snapshotCounts(t, env.DB, tables)

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/load", athlete.ID), "", cookies)
	requireStatus(t, rr, http.StatusOK)

	after := snapshotCounts(t, env.DB, tables)
	for _, tbl := range tables {
		if before[tbl] != after[tbl] {
			t.Errorf("table %q row count changed across GET /load: before=%d after=%d (load must be read-only)",
				tbl, before[tbl], after[tbl])
		}
	}
}

// snapshotCounts returns the row count of each named table.
func snapshotCounts(t *testing.T, db *sql.DB, tables []string) map[string]int {
	t.Helper()
	counts := make(map[string]int, len(tables))
	for _, tbl := range tables {
		var n int
		// Table names are a fixed test-local allowlist, not user input.
		if err := db.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		counts[tbl] = n
	}
	return counts
}
