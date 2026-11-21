package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
)

func TestCoachReads_HistoryWeekProgressPositionAndNote(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	squat := env.createExercise(t, "Back Squat")
	bench := env.createExercise(t, "Bench Press")
	token := env.coachServiceToken(t, coach.ID)

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("timezone: %v", err)
	}
	today := time.Now().In(loc)
	pastMonday := dateAtMonday(today).AddDate(0, 0, -14)
	mon := pastMonday.Format(dateLayout)
	wed := pastMonday.AddDate(0, 0, 2).Format(dateLayout)
	fri := pastMonday.AddDate(0, 0, 4).Format(dateLayout)

	tpl := env.createProgramTemplate(t, "531", 1, 3)
	reps := 5
	pct := 75.0
	for _, day := range []int{1, 2} {
		if _, err := models.CreatePrescribedSet(context.Background(), env.DB, tpl.ID, squat.ID, 1, day, 1, &reps, &pct, nil, 0, "reps", ""); err != nil {
			t.Fatalf("prescribe squat day %d: %v", day, err)
		}
		if _, err := models.CreatePrescribedSet(context.Background(), env.DB, tpl.ID, bench.ID, 1, day, 1, &reps, &pct, nil, 1, "reps", ""); err != nil {
			t.Fatalf("prescribe bench day %d: %v", day, err)
		}
	}
	if _, err := models.CreatePrescribedSet(context.Background(), env.DB, tpl.ID, squat.ID, 1, 3, 1, &reps, &pct, nil, 0, "reps", ""); err != nil {
		t.Fatalf("prescribe day 3: %v", err)
	}
	if _, err := models.SetTrainingMax(context.Background(), env.DB, athlete.ID, squat.ID, 200, mon, ""); err != nil {
		t.Fatalf("tm: %v", err)
	}
	program, err := models.AssignProgram(context.Background(), env.DB, athlete.ID, tpl.ID, mon, "", "", "primary", "[1,3,5]")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	mondaySession, err := models.CreateWorkout(context.Background(), env.DB, athlete.ID, mon, "solid", program.ID)
	if err != nil {
		t.Fatalf("monday workout: %v", err)
	}
	if _, err := models.AddSet(context.Background(), env.DB, mondaySession.ID, squat.ID, 5, 135, 6, "reps", "main", "warmup-ish"); err != nil {
		t.Fatalf("light set: %v", err)
	}
	if _, err := models.AddSet(context.Background(), env.DB, mondaySession.ID, squat.ID, 3, 245, 8, "reps", "main", ""); err != nil {
		t.Fatalf("top set: %v", err)
	}
	if _, err := models.AddSet(context.Background(), env.DB, mondaySession.ID, bench.ID, 5, 100, 7, "reps", "main", ""); err != nil {
		t.Fatalf("bench set: %v", err)
	}
	wedSession, err := models.CreateWorkout(context.Background(), env.DB, athlete.ID, wed, "", program.ID)
	if err != nil {
		t.Fatalf("wednesday workout: %v", err)
	}
	if _, err := models.AddSet(context.Background(), env.DB, wedSession.ID, squat.ID, 5, 185, 0, "reps", "main", ""); err != nil {
		t.Fatalf("wednesday set: %v", err)
	}

	base := fmt.Sprintf("/api/athletes/%d/coach", athlete.ID)
	rr := env.serveBearer(t, "GET", base+"/sessions?start="+mon+"&end="+fri, token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var history CoachHistoryResponse
	decodeJSON(t, rr, &history)
	if len(history.Sessions) != 2 || history.NextCursor != nil {
		t.Fatalf("history = %+v, want 2 sessions and no cursor", history)
	}
	if history.Sessions[0].SessionID != wedSession.ID || history.Sessions[0].Date != wed {
		t.Fatalf("newest session = %+v, want %d on %s", history.Sessions[0], wedSession.ID, wed)
	}
	older := history.Sessions[1]
	if older.SessionID != mondaySession.ID || older.Date != mon || older.StartedAt != nil || older.Duration != nil {
		t.Fatalf("monday session missing nulls or identity: %+v", older)
	}
	if older.ProgramDayLabel == nil || *older.ProgramDayLabel != "Week 1 Day 1" {
		t.Fatalf("program day label = %v, want Week 1 Day 1", older.ProgramDayLabel)
	}
	if older.Notes == nil || *older.Notes != "solid" {
		t.Fatalf("notes = %v, want solid", older.Notes)
	}
	if len(older.Exercises) != 2 || older.Exercises[0].ExerciseID != squat.ID || len(older.Exercises[0].Sets) != 2 {
		t.Fatalf("exercises = %+v", older.Exercises)
	}
	top := older.Exercises[0].Sets[1]
	if top.Reps != 3 || top.Load.Value == nil || *top.Load.Value != 245 || top.Load.Unit != "lb" || top.RPE == nil || *top.RPE != 8 || top.RIR != nil || top.SetType != nil || top.Category != "main" {
		t.Fatalf("top set = %+v", top)
	}

	rr = env.serveBearer(t, "GET", base+"/week?week_start="+mon, token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var week CoachWeekResponse
	decodeJSON(t, rr, &week)
	if week.Summary != "1 of 3" || week.DoneCount != 1 || week.PartialCount != 1 || week.MissedCount != 1 || week.PlannedCount != 3 {
		t.Fatalf("week counts = %+v", week)
	}
	if len(week.Days) != 3 || week.Days[0].Status != "done" || week.Days[1].Status != "partial" || week.Days[2].Status != "missed" {
		t.Fatalf("week days = %+v", week.Days)
	}
	if week.Days[0].CompletedOn == nil || *week.Days[0].CompletedOn != mon || week.Days[2].CompletedOn != nil {
		t.Fatalf("completion dates = %+v", week.Days)
	}
	if week.Timezone != "America/New_York" {
		t.Fatalf("timezone = %s", week.Timezone)
	}

	rr = env.serveBearer(t, "GET", base+"/progress?exercise="+fmt.Sprint(squat.ID)+"&start="+wed+"&end="+wed, token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var progress CoachProgressResponse
	decodeJSON(t, rr, &progress)
	if progress.E1RMSource != "epley" || len(progress.Exercises) != 1 {
		t.Fatalf("progress = %+v", progress)
	}
	ex := progress.Exercises[0]
	if len(ex.Sessions) != 1 || len(ex.PRs) != 0 {
		t.Fatalf("window should not invent a PR over the heavier Monday set: %+v", ex)
	}
	if ex.LastSessionTopSet == nil || ex.LastSessionTopSet.Load.Value == nil || *ex.LastSessionTopSet.Load.Value != 185 || ex.LastSessionTopSet.Load.Unit != "lb" {
		t.Fatalf("last top set = %+v", ex.LastSessionTopSet)
	}

	rr = env.serveBearer(t, "GET", base+"/progress?exercise="+fmt.Sprint(squat.ID), token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &progress)
	ex = progress.Exercises[0]
	if len(ex.Sessions) != 2 || len(ex.PRs) != 1 || ex.PRs[0].Date != mon {
		t.Fatalf("full progress = %+v", ex)
	}
	wantE := 245.0 * (1 + 3.0/30.0)
	if ex.PRs[0].E1RM != wantE || ex.Sessions[0].E1RM == nil || *ex.Sessions[0].E1RM != wantE {
		t.Fatalf("e1rm = %v / %+v, want %v", ex.PRs[0].E1RM, ex.Sessions[0].E1RM, wantE)
	}
	if ex.Sessions[0].TopSet.Load.Value == nil || *ex.Sessions[0].TopSet.Load.Value != 245 || ex.Sessions[0].TopSet.Reps != 3 {
		t.Fatalf("monday top set = %+v", ex.Sessions[0].TopSet)
	}

	var workoutsBefore int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM workouts`).Scan(&workoutsBefore); err != nil {
		t.Fatalf("count workouts: %v", err)
	}
	rr = env.serveBearer(t, "GET", base+"/position", token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var position CoachPositionResponse
	decodeJSON(t, rr, &position)
	if position.Block != nil || position.ProgramName == nil || *position.ProgramName != "531" || position.Week == nil || *position.Week != 1 || position.Day == nil || *position.Day != 3 {
		t.Fatalf("position = %+v", position)
	}
	if position.NextSession == nil || len(position.NextSession.Exercises) != 1 || position.NextSession.Exercises[0].Name != "Back Squat" {
		t.Fatalf("next session = %+v", position.NextSession)
	}
	target := position.NextSession.Exercises[0].Sets[0].Load
	if target.Value == nil || *target.Value != 150 || target.Unit != "lb" {
		t.Fatalf("target load = %+v, want 150 lb", target)
	}
	var workoutsAfter int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM workouts`).Scan(&workoutsAfter); err != nil {
		t.Fatalf("count workouts: %v", err)
	}
	if workoutsAfter != workoutsBefore {
		t.Fatalf("position created workouts: before %d after %d", workoutsBefore, workoutsAfter)
	}

	rr = env.serveBearer(t, "GET", fmt.Sprintf("%s/exercises/%d/last-session", base, squat.ID), token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var last CoachLastSessionResponse
	decodeJSON(t, rr, &last)
	if last.Session == nil || last.Session.SessionID != wedSession.ID || last.Session.StartedAt != nil || len(last.Session.Sets) != 1 {
		t.Fatalf("last squat session = %+v", last)
	}
	if last.Session.Sets[0].RIR != nil || last.Session.Sets[0].SetType != nil || last.Session.Sets[0].Load.Unit != "lb" {
		t.Fatalf("last set = %+v", last.Session.Sets[0])
	}

	notePath := fmt.Sprintf("%s/sessions/%d/notes", base, mondaySession.ID)
	rr = env.serveBearer(t, "POST", notePath+"/preview", token, `{"notes":"knee was fine"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	var preview CoachNotePreviewResponse
	decodeJSON(t, rr, &preview)
	if preview.ConfirmToken == "" || preview.WouldWrite.Notes != "knee was fine" || preview.WouldWrite.SessionID != mondaySession.ID {
		t.Fatalf("preview = %+v", preview)
	}
	var unchanged string
	if err := env.DB.QueryRow(`SELECT notes FROM workouts WHERE id = ?`, mondaySession.ID).Scan(&unchanged); err != nil {
		t.Fatalf("notes after preview: %v", err)
	}
	if unchanged != "solid" {
		t.Fatalf("preview wrote notes %q", unchanged)
	}
	rr = env.serveBearer(t, "POST", notePath+"/confirm", token, `{"notes":"different","confirm_token":"`+preview.ConfirmToken+`"}`, nil)
	requireStatus(t, rr, http.StatusConflict)
	rr = env.serveBearer(t, "POST", notePath+"/confirm", token, `{"notes":"knee was fine","confirm_token":"`+preview.ConfirmToken+`"}`, nil)
	requireStatus(t, rr, http.StatusOK)
	rr = env.serveBearer(t, "POST", notePath+"/confirm", token, `{"notes":"knee was fine","confirm_token":"`+preview.ConfirmToken+`"}`, nil)
	requireStatus(t, rr, http.StatusBadRequest)
	var stored string
	if err := env.DB.QueryRow(`SELECT notes FROM workouts WHERE id = ?`, mondaySession.ID).Scan(&stored); err != nil {
		t.Fatalf("notes after confirm: %v", err)
	}
	if stored != "knee was fine" {
		t.Fatalf("stored notes = %q", stored)
	}
	var setCount, tmCount int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM workout_sets`).Scan(&setCount); err != nil {
		t.Fatal(err)
	}
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM training_maxes`).Scan(&tmCount); err != nil {
		t.Fatal(err)
	}
	if setCount != 4 || tmCount != 1 || workoutsAfter != 2 {
		t.Fatalf("note confirm changed other rows: sets %d tms %d workouts %d", setCount, tmCount, workoutsAfter)
	}
}

func TestCoachReads_PaginationAndCursor(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	other := env.createAthlete(t, "Other", coach.ID)
	token := env.coachServiceToken(t, coach.ID)
	for i := 0; i < models.CoachSessionPageSize+1; i++ {
		date := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(dateLayout)
		env.createWorkout(t, athlete.ID, date)
	}
	foreign := env.createWorkout(t, other.ID, "2024-06-01")
	path := fmt.Sprintf("/api/athletes/%d/coach/sessions?start=2024-01-01&end=2024-12-31", athlete.ID)
	rr := env.serveBearer(t, "GET", path, token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var page CoachHistoryResponse
	decodeJSON(t, rr, &page)
	if len(page.Sessions) != models.CoachSessionPageSize || page.NextCursor == nil {
		t.Fatalf("page len %d cursor %v", len(page.Sessions), page.NextCursor)
	}
	rr = env.serveBearer(t, "GET", path+"&cursor="+*page.NextCursor, token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var rest CoachHistoryResponse
	decodeJSON(t, rr, &rest)
	if len(rest.Sessions) != 1 || rest.NextCursor != nil {
		t.Fatalf("second page = %+v", rest)
	}
	rr = env.serveBearer(t, "GET", path+"&cursor="+fmt.Sprint(foreign.ID), token, nil, nil)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestCrossAthleteIDOR_CoachSessionNote(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	athleteA := env.createAthlete(t, "Athlete A", admin.ID)
	athleteB := env.createAthlete(t, "Athlete B", admin.ID)
	workoutA := env.createWorkout(t, athleteA.ID, "2026-05-12")
	if err := models.UpdateWorkoutNotes(context.Background(), env.DB, workoutA.ID, athleteA.ID, "kept"); err != nil {
		t.Fatalf("seed notes: %v", err)
	}
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/coach/sessions/%d/notes/preview", athleteB.ID, workoutA.ID),
		`{"notes":"tampered"}`, cookies)
	requireStatus(t, rr, http.StatusNotFound)
	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/coach/sessions/%d/notes/confirm", athleteB.ID, workoutA.ID),
		`{"notes":"tampered","confirm_token":"rlnote_nope"}`, cookies)
	requireStatus(t, rr, http.StatusNotFound)

	var notes string
	if err := env.DB.QueryRow(`SELECT notes FROM workouts WHERE id = ?`, workoutA.ID).Scan(&notes); err != nil {
		t.Fatalf("notes: %v", err)
	}
	if notes != "kept" {
		t.Fatalf("notes = %q, want kept", notes)
	}
	rr = env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/coach/sessions?start=2026-05-01&end=2026-05-31", athleteA.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestServiceToken_CannotReadOrWriteAnotherUser(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, true)
	other := env.createUser(t, "other", true, false)
	mine := env.createAthlete(t, "Mine", coach.ID)
	theirs := env.createAthlete(t, "Theirs", other.ID)
	exercise := env.createExercise(t, "Deadlift")
	myWorkout := env.createWorkout(t, mine.ID, "2026-11-01")
	theirWorkout := env.createWorkout(t, theirs.ID, "2026-11-02")
	if _, err := models.AddSet(context.Background(), env.DB, theirWorkout.ID, exercise.ID, 3, 315, 9, "reps", "main", ""); err != nil {
		t.Fatalf("their set: %v", err)
	}
	if _, err := models.AddSet(context.Background(), env.DB, myWorkout.ID, exercise.ID, 5, 225, 8, "reps", "main", ""); err != nil {
		t.Fatalf("my set: %v", err)
	}
	token := env.coachServiceToken(t, coach.ID)
	kid := env.createUser(t, "kid", false, false)
	if _, err := models.UpdateUser(context.Background(), env.DB, kid.ID, kid.Username, "", kid.Email.String,
		sql.NullInt64{Int64: mine.ID, Valid: true}, false, false); err != nil {
		t.Fatalf("link kid: %v", err)
	}
	kidToken := env.coachServiceToken(t, kid.ID)

	foreign := []string{
		fmt.Sprintf("/api/athletes/%d/coach/sessions?start=2026-11-01&end=2026-11-30", theirs.ID),
		fmt.Sprintf("/api/athletes/%d/coach/week?week_start=2026-11-02", theirs.ID),
		fmt.Sprintf("/api/athletes/%d/coach/progress", theirs.ID),
		fmt.Sprintf("/api/athletes/%d/coach/position", theirs.ID),
		fmt.Sprintf("/api/athletes/%d/coach/exercises/%d/last-session", theirs.ID, exercise.ID),
	}
	for _, path := range foreign {
		rr := env.serveBearer(t, "GET", path, token, nil, nil)
		if rr.Code != http.StatusForbidden {
			t.Errorf("GET %s = %d, want 403\n%s", path, rr.Code, rr.Body.String())
		}
		rr = env.serveBearer(t, "GET", path, kidToken, nil, nil)
		if rr.Code != http.StatusForbidden {
			t.Errorf("kid GET %s = %d, want 403\n%s", path, rr.Code, rr.Body.String())
		}
	}

	note := fmt.Sprintf("/api/athletes/%d/coach/sessions/%d/notes/preview", theirs.ID, theirWorkout.ID)
	rr := env.serveBearer(t, "POST", note, token, `{"notes":"nope"}`, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.serveBearer(t, "POST", note, kidToken, `{"notes":"nope"}`, nil)
	requireStatus(t, rr, http.StatusForbidden)
	ownNote := fmt.Sprintf("/api/athletes/%d/coach/sessions/%d/notes/preview", mine.ID, myWorkout.ID)
	rr = env.serveBearer(t, "POST", ownNote, kidToken, `{"notes":"nope"}`, nil)
	requireStatus(t, rr, http.StatusForbidden)

	rr = env.serveBearer(t, "POST",
		fmt.Sprintf("/api/athletes/%d/workouts/%d/sets", theirs.ID, theirWorkout.ID),
		token, fmt.Sprintf(`{"exercise_id":%d,"reps":1,"weight":45}`, exercise.ID), nil)
	requireStatus(t, rr, http.StatusForbidden)

	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/exercises/%d/last-session", mine.ID, exercise.ID), token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var last CoachLastSessionResponse
	decodeJSON(t, rr, &last)
	if last.Session == nil || last.Session.SessionID != myWorkout.ID {
		t.Fatalf("last session leaked or missed: %+v", last)
	}
	rr = env.serveBearer(t, "GET", fmt.Sprintf("/api/athletes/%d/coach/sessions?start=2026-11-01&end=2026-11-30", mine.ID), kidToken, nil, nil)
	requireStatus(t, rr, http.StatusOK)

	var n int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM workout_sets WHERE workout_id = ?`, theirWorkout.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("foreign set rows = %d, want 1", n)
	}
}

func TestServiceToken_AdminIssuesAndRevokes(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Dana", coach.ID)
	env.createWorkout(t, athlete.ID, "2026-11-01")
	adminCookies := env.loginAs(t, admin)
	coachCookies := env.loginAs(t, coach)
	adminToken := env.coachServiceToken(t, admin.ID)

	rr := env.serveBearer(t, "POST", fmt.Sprintf("/api/users/%d/service-tokens", coach.ID), adminToken, `{"label":"agent"}`, nil)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.do(t, "POST", fmt.Sprintf("/api/users/%d/service-tokens", coach.ID), `{"label":"agent"}`, coachCookies)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.do(t, "POST", fmt.Sprintf("/api/users/%d/service-tokens", coach.ID), `{"label":"coach-service"}`, adminCookies)
	requireStatus(t, rr, http.StatusBadRequest)

	rr = env.do(t, "POST", fmt.Sprintf("/api/users/%d/service-tokens", coach.ID), `{"label":"agent"}`, adminCookies)
	requireStatus(t, rr, http.StatusCreated)
	var issued ServiceTokenIssued
	decodeJSON(t, rr, &issued)
	if !strings.HasPrefix(issued.Token, models.ServiceTokenPrefix) || issued.UserID != coach.ID {
		t.Fatalf("issued = %+v", issued)
	}
	var stored string
	if err := env.DB.QueryRow(`SELECT token_hash FROM service_tokens WHERE id = ?`, issued.ID).Scan(&stored); err != nil {
		t.Fatalf("hash: %v", err)
	}
	if stored == issued.Token || strings.Contains(stored, issued.Token) {
		t.Fatal("service_tokens persisted a usable token")
	}
	rr = env.do(t, "GET", fmt.Sprintf("/api/users/%d/service-tokens", coach.ID), nil, adminCookies)
	requireStatus(t, rr, http.StatusOK)
	if strings.Contains(rr.Body.String(), issued.Token) {
		t.Fatal("list response included the plaintext token")
	}

	path := fmt.Sprintf("/api/athletes/%d/coach/sessions?start=2026-11-01&end=2026-11-30", athlete.ID)
	rr = env.serveBearer(t, "GET", path, issued.Token, nil, nil)
	requireStatus(t, rr, http.StatusOK)

	rr = env.do(t, "DELETE", fmt.Sprintf("/api/users/%d/service-tokens/%d", admin.ID, issued.ID), nil, adminCookies)
	requireStatus(t, rr, http.StatusNotFound)
	rr = env.do(t, "DELETE", fmt.Sprintf("/api/users/%d/service-tokens/%d", coach.ID, issued.ID), nil, adminCookies)
	requireStatus(t, rr, http.StatusOK)
	rr = env.serveBearer(t, "GET", path, issued.Token, nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

func dateAtMonday(now time.Time) time.Time {
	offset := int(now.Weekday() - time.Monday)
	if now.Weekday() == time.Sunday {
		offset = 6
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -offset)
}
