package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// TestWhiteboard covers gap 15: today's prescribed lines and logged sets for
// several athletes, warm-ups excluded from the counts, an unprescribed
// logged lift appended, the default board, access checks, and that the
// board writes nothing.
func TestWhiteboard(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)
	sam := env.createAthlete(t, "Sam", coach.ID)
	alex := env.createAthlete(t, "Alex", coach.ID)
	stranger := env.createAthlete(t, "Stranger", 0)
	squat := env.createExercise(t, "Back Squat")
	row := env.createExercise(t, "Barbell Row")
	curl := env.createExercise(t, "Curl")

	// Sam: a 1x1 program with a warm-up and two working squat sets, plus a
	// superset row.
	tmpl := env.createProgramTemplate(t, "Board Block", 1, 1)
	setsPath := fmt.Sprintf("/api/programs/%d/sets", tmpl.ID)
	for _, b := range []string{
		fmt.Sprintf(`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":5,"non_working":true}`, squat.ID),
		fmt.Sprintf(`{"exercise_id":%d,"week":1,"day":1,"set_number":2,"reps":5,"absolute_weight":185,"group_key":"A","sort_order":1}`, squat.ID),
		fmt.Sprintf(`{"exercise_id":%d,"week":1,"day":1,"set_number":3,"reps":5,"absolute_weight":185,"group_key":"A","sort_order":1}`, squat.ID),
		fmt.Sprintf(`{"exercise_id":%d,"week":1,"day":1,"set_number":1,"reps":10,"group_key":"A","sort_order":2}`, row.ID),
	} {
		requireStatus(t, env.do(t, "POST", setsPath, b, cookies), http.StatusCreated)
	}
	requireStatus(t, env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", sam.ID),
		fmt.Sprintf(`{"template_id":%d,"start_date":"2026-01-01"}`, tmpl.ID), cookies), http.StatusCreated)

	loc, _ := time.LoadLocation(models.DefaultTimezone)
	today := time.Now().In(loc).Format("2006-01-02")
	env.logTestSet(t, sam.ID, squat.ID, today, 5, 95, models.SetTypeWarmup)
	env.logTestSet(t, sam.ID, squat.ID, today, 5, 185, models.SetTypeWorking)
	env.logTestSet(t, sam.ID, curl.ID, today, 12, 25, models.SetTypeWorking)

	var before int
	_ = env.DB.QueryRow(`SELECT COUNT(*) FROM workouts`).Scan(&before)

	rr := env.do(t, "GET", fmt.Sprintf("/api/whiteboard?athletes=%d,%d", sam.ID, alex.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var board WhiteboardResponse
	decodeJSON(t, rr, &board)
	if board.Date != today || len(board.Athletes) != 2 {
		t.Fatalf("board = %+v", board)
	}
	s := board.Athletes[0]
	if s.Name != "Sam" || s.ProgramName == nil || *s.ProgramName != "Board Block" || s.WorkoutID == nil {
		t.Fatalf("sam column = %+v", s)
	}
	if len(s.Lines) != 3 {
		t.Fatalf("sam lines = %d, want squat, row, curl", len(s.Lines))
	}
	sq := s.Lines[0]
	if sq.ExerciseID != squat.ID || sq.WorkingTarget != 2 || sq.WorkingDone != 1 || len(sq.Logged) != 2 || len(sq.Prescribed) != 3 {
		t.Fatalf("squat line = %+v", sq)
	}
	if sq.GroupKey == nil || *sq.GroupKey != "A" {
		t.Fatalf("squat group = %v", sq.GroupKey)
	}
	if c := s.Lines[2]; c.ExerciseID != curl.ID || len(c.Prescribed) != 0 || c.WorkingDone != 1 {
		t.Fatalf("unprescribed curl line = %+v", c)
	}
	if s.WorkingTarget != 3 || s.WorkingDone != 2 {
		t.Fatalf("sam totals target=%d done=%d", s.WorkingTarget, s.WorkingDone)
	}
	if a := board.Athletes[1]; a.Name != "Alex" || len(a.Lines) != 0 || a.WorkoutID != nil || a.ProgramName != nil {
		t.Fatalf("alex column = %+v", a)
	}

	var after int
	_ = env.DB.QueryRow(`SELECT COUNT(*) FROM workouts`).Scan(&after)
	if after != before {
		t.Fatalf("whiteboard read created workouts (%d → %d)", before, after)
	}

	// Default board: every athlete the coach sees; available lists them.
	rr = env.do(t, "GET", "/api/whiteboard", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	board = WhiteboardResponse{}
	decodeJSON(t, rr, &board)
	if len(board.Athletes) != 2 || len(board.Available) != 2 || board.Truncated {
		t.Fatalf("default board = %+v", board)
	}

	// Access and validation.
	requireStatus(t, env.do(t, "GET", fmt.Sprintf("/api/whiteboard?athletes=%d", stranger.ID), nil, cookies), http.StatusForbidden)
	requireStatus(t, env.do(t, "GET", "/api/whiteboard?athletes=1,x", nil, cookies), http.StatusBadRequest)
	ids := make([]string, 9)
	for i := range ids {
		ids[i] = fmt.Sprint(i + 1)
	}
	requireStatus(t, env.do(t, "GET", "/api/whiteboard?athletes="+strings.Join(ids, ","), nil, cookies), http.StatusBadRequest)
	requireStatus(t, env.do(t, "GET", "/api/whiteboard", nil, nil), http.StatusUnauthorized)

	// An athlete user's board is their own athlete only.
	kid := env.createUser(t, "alexuser", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, alex.ID, kid.ID); err != nil {
		t.Fatal(err)
	}
	kidCookies := env.loginAs(t, kid)
	rr = env.do(t, "GET", "/api/whiteboard", nil, kidCookies)
	requireStatus(t, rr, http.StatusOK)
	board = WhiteboardResponse{}
	decodeJSON(t, rr, &board)
	if len(board.Athletes) != 1 || board.Athletes[0].AthleteID != alex.ID || len(board.Available) != 1 {
		t.Fatalf("kid board = %+v", board)
	}
	requireStatus(t, env.do(t, "GET", fmt.Sprintf("/api/whiteboard?athletes=%d", sam.ID), nil, kidCookies), http.StatusForbidden)

	// A coach service token sees only its coach's athletes.
	token := env.coachServiceToken(t, coach.ID)
	requireStatus(t, env.serveBearer(t, "GET", fmt.Sprintf("/api/whiteboard?athletes=%d", sam.ID), token, nil, nil), http.StatusOK)
	requireStatus(t, env.serveBearer(t, "GET", fmt.Sprintf("/api/whiteboard?athletes=%d", stranger.ID), token, nil, nil), http.StatusForbidden)
}
