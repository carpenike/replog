package models

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/importers"
)

func setDetailFixture(t *testing.T, db *sql.DB) (*Athlete, *Exercise) {
	t.Helper()
	ctx := context.Background()
	a, err := CreateAthlete(ctx, db, "Dana", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	ex, err := CreateExercise(ctx, db, "Back Squat", "", "", "", 0, true)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	return a, ex
}

func TestGroupKeyForIndex(t *testing.T) {
	cases := map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA", -1: ""}
	for i, want := range cases {
		if got := GroupKeyForIndex(i); got != want {
			t.Errorf("GroupKeyForIndex(%d) = %q, want %q", i, got, want)
		}
	}
	for _, ok := range []string{"", "A", "B2", "AA"} {
		if !ValidGroupKey(ok) {
			t.Errorf("ValidGroupKey(%q) = false", ok)
		}
	}
	for _, bad := range []string{"1", "a", "A-1", "ABCDE", "A B"} {
		if ValidGroupKey(bad) {
			t.Errorf("ValidGroupKey(%q) = true", bad)
		}
	}
	if NormalizeGroupKey(" b ") != "B" {
		t.Error("NormalizeGroupKey did not trim and upper-case")
	}
}

func TestPrescribedSetDetail_CreateUpdateCopy(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	_, ex := setDetailFixture(t, db)
	tmpl, err := CreateProgramTemplate(ctx, db, nil, "Hypertrophy", "", 2, 1, false, "")
	if err != nil {
		t.Fatal(err)
	}

	reps := 8
	if field, _ := ValidatePrescribedSetDetail(&reps, PrescribedSetDetail{RepsMax: ptr(6)}); field != "reps_max" {
		t.Fatalf("reps_max below reps validated as %q", field)
	}
	if field, _ := ValidatePrescribedSetDetail(&reps, PrescribedSetDetail{RIRMin: ptr(3), RIRMax: ptr(2)}); field != "rir_max" {
		t.Fatalf("inverted RIR validated as %q", field)
	}
	if field, _ := ValidatePrescribedSetDetail(&reps, PrescribedSetDetail{GroupKey: "9"}); field != "group_key" {
		t.Fatalf("bad group key validated as %q", field)
	}

	set, err := CreatePrescribedSetDetailed(ctx, db, tmpl.ID, ex.ID, 1, 1, 1, &reps, nil, nil, nil, 0, "reps", "", nil, "",
		PrescribedSetDetail{RepsMax: ptr(12), RIRMin: ptr(2), RIRMax: ptr(3), NonWorking: true, GroupKey: "a"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !set.RepsMax.Valid || set.RepsMax.Int64 != 12 || set.RIRMin.Int64 != 2 || set.RIRMax.Int64 != 3 || !set.NonWorking || set.GroupKey.String != "A" {
		t.Fatalf("created set = %+v", set)
	}
	if got := set.RepsLabel(); got != "8-12" {
		t.Fatalf("RepsLabel = %q, want 8-12", got)
	}

	// An update with an empty patch keeps the detail.
	kept, err := UpdatePrescribedSet(ctx, db, set.ID, ex.ID, 1, &reps, nil, nil, nil, 0, "reps", "", nil, "", PrescribedSetDetailPatch{})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !kept.RepsMax.Valid || !kept.RIRMin.Valid || !kept.NonWorking || !kept.GroupKey.Valid {
		t.Fatalf("empty patch changed detail: %+v", kept)
	}

	// Copy week carries the detail.
	if n, err := CopyWeek(ctx, db, tmpl.ID, 1, 2); err != nil || n != 1 {
		t.Fatalf("copy week = %d, %v", n, err)
	}
	week2, err := ListPrescribedSetsForDay(ctx, db, tmpl.ID, 2, 1)
	if err != nil || len(week2) != 1 {
		t.Fatalf("week 2 = %v, %v", week2, err)
	}
	if week2[0].RepsMax.Int64 != 12 || !week2[0].NonWorking || week2[0].GroupKey.String != "A" {
		t.Fatalf("copied set = %+v", week2[0])
	}

	// Clearing patch.
	off, empty := false, ""
	cleared, err := UpdatePrescribedSet(ctx, db, set.ID, ex.ID, 1, &reps, nil, nil, nil, 0, "reps", "", nil, "",
		PrescribedSetDetailPatch{ClearRepsMax: true, ClearRIR: true, NonWorking: &off, GroupKey: &empty})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared.RepsMax.Valid || cleared.RIRMin.Valid || cleared.RIRMax.Valid || cleared.NonWorking || cleared.GroupKey.Valid {
		t.Fatalf("cleared set = %+v", cleared)
	}

	if _, err := UpdatePrescribedSet(ctx, db, 99999, ex.ID, 1, &reps, nil, nil, nil, 0, "reps", "", nil, "", PrescribedSetDetailPatch{}); err != ErrNotFound {
		t.Fatalf("update missing = %v, want ErrNotFound", err)
	}
}

func TestSeedSetsFromPrescription_WarmupSetType(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, ex := setDetailFixture(t, db)
	tmpl, err := CreateProgramTemplate(ctx, db, nil, "Ramp", "", 1, 1, true, "")
	if err != nil {
		t.Fatal(err)
	}
	five := 5
	if _, err := CreatePrescribedSetDetailed(ctx, db, tmpl.ID, ex.ID, 1, 1, 1, &five, nil, nil, nil, 0, "reps", "", nil, "", PrescribedSetDetail{NonWorking: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreatePrescribedSetDetailed(ctx, db, tmpl.ID, ex.ID, 1, 1, 2, &five, nil, nil, nil, 0, "reps", "", nil, "", PrescribedSetDetail{}); err != nil {
		t.Fatal(err)
	}
	ap, err := AssignProgram(ctx, db, a.ID, tmpl.ID, "2026-01-01", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	rx, err := GetPrescription(ctx, db, ap, mustParseDate("2026-01-05"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := CreateWorkout(ctx, db, a.ID, "2026-01-05", "", ap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := SeedSetsFromPrescription(ctx, db, w.ID, rx); err != nil || n != 2 {
		t.Fatalf("seed = %d, %v", n, err)
	}
	groups, err := ListSetsByWorkout(ctx, db, w.ID)
	if err != nil || len(groups) != 1 || len(groups[0].Sets) != 2 {
		t.Fatalf("groups = %v, %v", groups, err)
	}
	if groups[0].Sets[0].SetType != SetTypeWarmup || groups[0].Sets[1].SetType != SetTypeWorking {
		t.Fatalf("seeded set types = %q, %q", groups[0].Sets[0].SetType, groups[0].Sets[1].SetType)
	}
	got, err := GetWorkoutByID(ctx, db, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SetCount != 1 {
		t.Fatalf("set count = %d, want 1 (warm-up excluded)", got.SetCount)
	}
}

// TestWarmupsExcludedFromReads logs one heavy warm-up and one lighter
// working set, then checks every server-side read that counts work
// ignores the warm-up.
func TestWarmupsExcludedFromReads(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, ex := setDetailFixture(t, db)
	if _, err := AssignExercise(ctx, db, a.ID, ex.ID, 0); err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")

	// A warm-up-only day must not count toward a streak.
	warmOnly, err := CreateWorkout(ctx, db, a.ID, time.Now().AddDate(0, 0, -14).Format("2006-01-02"), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddSetWithType(ctx, db, warmOnly.ID, ex.ID, 5, 95, 0, "reps", "main", "", SetTypeWarmup); err != nil {
		t.Fatal(err)
	}

	w, err := CreateWorkout(ctx, db, a.ID, today, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	// The warm-up is (artificially) heavier so a leak would show up as the PR.
	if _, err := AddSetWithType(ctx, db, w.ID, ex.ID, 5, 400, 0, "reps", "main", "", SetTypeWarmup); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSetWithType(ctx, db, w.ID, ex.ID, 5, 200, 0, "reps", "main", "", SetTypeWorking); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSetWithType(ctx, db, w.ID, ex.ID, 8, 150, 0, "reps", "main", "", SetTypeDrop); err != nil {
		t.Fatal(err)
	}
	wantVolume := 5*200.0 + 8*150.0

	vol, err := ExerciseVolumeChart(ctx, db, a.ID, ex.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(vol.Bars) != 1 || vol.MaxVol != wantVolume {
		t.Fatalf("volume chart = %+v, want one bar of %v", vol, wantVolume)
	}

	load, err := GetLoadSummary(ctx, db, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range load.Disciplines {
		if d.Discipline == "resistance" && d.Acute7 != wantVolume {
			t.Fatalf("resistance acute load = %v, want %v", d.Acute7, wantVolume)
		}
	}

	lifts, err := ListFeaturedLifts(ctx, db, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(lifts) != 1 || !lifts[0].BestWeight.Valid || lifts[0].BestWeight.Float64 != 200 {
		t.Fatalf("featured best = %+v", lifts)
	}

	progress, err := ListCoachProgress(ctx, db, a.ID, ex.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) != 1 || len(progress[0].Sessions) != 1 {
		t.Fatalf("progress = %+v (the warm-up-only session must not appear)", progress)
	}
	top := progress[0].Sessions[0]
	if !top.Weight.Valid || top.Weight.Float64 != 200 {
		t.Fatalf("top set = %+v, want 200", top)
	}
	if len(progress[0].PRs) != 1 || progress[0].PRs[0].Weight != 200 {
		t.Fatalf("PRs = %+v", progress[0].PRs)
	}

	got, err := GetWorkoutByID(ctx, db, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SetCount != 2 {
		t.Fatalf("set count = %d, want 2", got.SetCount)
	}

	streaks, err := WeeklyStreaks(ctx, db, a.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	for _, s := range streaks {
		completed += s.CompletedCount
	}
	if completed != 1 {
		t.Fatalf("streak completions = %d, want 1 (warm-up-only week excluded)", completed)
	}

	// Coach session reads still show the warm-up, with its type.
	sessions, _, err := ListCoachSessions(ctx, db, a.ID, "2000-01-01", "2100-01-01", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) == 0 || len(sessions[0].Exercises) != 1 || len(sessions[0].Exercises[0].Sets) != 3 || sessions[0].Exercises[0].Sets[0].SetType != SetTypeWarmup {
		t.Fatalf("coach sessions = %+v", sessions)
	}

	// Update changes the set type and rejects a bad one.
	sets := sessions[0].Exercises[0].Sets
	working := SetTypeWorking
	updated, err := UpdateSetWithType(ctx, db, sets[0].ID, a.ID, nil, nil, nil, nil, &working)
	if err != nil || updated.SetType != SetTypeWorking {
		t.Fatalf("update set type = %+v, %v", updated, err)
	}
	bad := "cooldown"
	if _, err := UpdateSetWithType(ctx, db, sets[0].ID, a.ID, nil, nil, nil, nil, &bad); err == nil {
		t.Fatal("bad set type accepted")
	}
}

// TestSetTypeExportImportRoundTrip exports an athlete with a warm-up set
// and a detailed program, re-imports the JSON for another athlete, and
// checks set types and prescription detail survive.
func TestSetTypeExportImportRoundTrip(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, ex := setDetailFixture(t, db)
	w, err := CreateWorkout(ctx, db, a.ID, "2026-03-02", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddSetWithType(ctx, db, w.ID, ex.ID, 5, 95, 0, "reps", "main", "", SetTypeWarmup); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSetWithType(ctx, db, w.ID, ex.ID, 5, 185, 0, "reps", "main", "", SetTypeFailure); err != nil {
		t.Fatal(err)
	}
	tmpl, err := CreateProgramTemplate(ctx, db, nil, "Detail Block", "", 1, 1, true, "")
	if err != nil {
		t.Fatal(err)
	}
	eight := 8
	if _, err := CreatePrescribedSetDetailed(ctx, db, tmpl.ID, ex.ID, 1, 1, 1, &eight, nil, nil, nil, 0, "reps", "", nil, "",
		PrescribedSetDetail{RepsMax: ptr(12), RIRMin: ptr(1), RIRMax: ptr(2), GroupKey: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, err := AssignProgram(ctx, db, a.ID, tmpl.ID, "2026-03-01", "", "", "", ""); err != nil {
		t.Fatal(err)
	}

	export, err := BuildAthleteExportJSON(ctx, db, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(export.Workouts) != 1 || export.Workouts[0].Sets[0].SetType != SetTypeWarmup {
		t.Fatalf("export sets = %+v", export.Workouts)
	}
	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"set_type":"warmup"`) || !strings.Contains(string(raw), `"reps_max":12`) {
		t.Fatalf("export JSON lacks set_type or reps_max: %s", raw)
	}

	parsed, err := importers.ParseRepLogJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	b, err := CreateAthlete(ctx, db, "Bo", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatal(err)
	}
	ms := &importers.MappingState{
		Format:    importers.FormatRepLogJSON,
		Exercises: []importers.EntityMapping{{ImportName: "Back Squat", MappedID: ex.ID}},
		Programs:  []importers.EntityMapping{{ImportName: "Detail Block", Create: true}},
		Parsed:    parsed,
	}
	// The import creates a template with the same name, so rename the source.
	if _, err := db.Exec(`UPDATE program_templates SET name = 'Detail Block (source)' WHERE id = ?`, tmpl.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteImport(ctx, db, b.ID, 0, ms); err != nil {
		t.Fatalf("import: %v", err)
	}

	var warm, failure int
	if err := db.QueryRow(`SELECT
	        SUM(ws.set_type = 'warmup'), SUM(ws.set_type = 'failure')
	      FROM workout_sets ws JOIN workouts w ON w.id = ws.workout_id WHERE w.athlete_id = ?`, b.ID).Scan(&warm, &failure); err != nil {
		t.Fatal(err)
	}
	if warm != 1 || failure != 1 {
		t.Fatalf("imported set types: warmup %d failure %d", warm, failure)
	}
	var repsMax, rirMin, rirMax sql.NullInt64
	var groupKey sql.NullString
	if err := db.QueryRow(`SELECT ps.reps_max, ps.rir_min, ps.rir_max, ps.group_key
	      FROM prescribed_sets ps JOIN program_templates pt ON pt.id = ps.template_id
	     WHERE pt.name = 'Detail Block'`).Scan(&repsMax, &rirMin, &rirMax, &groupKey); err != nil {
		t.Fatal(err)
	}
	if repsMax.Int64 != 12 || rirMin.Int64 != 1 || rirMax.Int64 != 2 || groupKey.String != "B" {
		t.Fatalf("imported detail = %v %v %v %v", repsMax, rirMin, rirMax, groupKey)
	}
}

func TestAccessoryPlanGroupKey(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, ex := setDetailFixture(t, db)
	plan, err := CreateAccessoryPlanGrouped(ctx, db, a.ID, 1, ex.ID, 3, 10, 12, 0, "", 0, "a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.GroupKey.String != "A" {
		t.Fatalf("group key = %+v", plan.GroupKey)
	}
	if _, err := CreateAccessoryPlanGrouped(ctx, db, a.ID, 2, ex.ID, 3, 10, 12, 0, "", 0, "1"); err == nil {
		t.Fatal("bad group key accepted")
	}
	// The ungrouped update keeps the key; an empty key ungroups.
	if err := UpdateAccessoryPlan(ctx, db, plan.ID, a.ID, 4, 8, 10, 0, "", 0); err != nil {
		t.Fatal(err)
	}
	got, _ := GetAccessoryPlanByID(ctx, db, plan.ID)
	if got.GroupKey.String != "A" || got.TargetSets.Int64 != 4 {
		t.Fatalf("after update = %+v", got)
	}
	empty := ""
	if err := UpdateAccessoryPlanGrouped(ctx, db, plan.ID, a.ID, 4, 8, 10, 0, "", 0, &empty); err != nil {
		t.Fatal(err)
	}
	got, _ = GetAccessoryPlanByID(ctx, db, plan.ID)
	if got.GroupKey.Valid {
		t.Fatalf("group key not cleared: %+v", got.GroupKey)
	}
}

func TestLineHasWorkingSet(t *testing.T) {
	warm := &PrescriptionLine{Sets: []*PrescribedSet{{NonWorking: true}, {NonWorking: true}}}
	mixed := &PrescriptionLine{Sets: []*PrescribedSet{{NonWorking: true}, {}}}
	if lineHasWorkingSet(warm) {
		t.Error("warm-up-only line counted as planned work")
	}
	if !lineHasWorkingSet(mixed) {
		t.Error("line with a working set not counted as planned work")
	}
}
