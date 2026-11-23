package api

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// TestVideoShareCascadeFromParents: deleting the workout or the athlete
// that owns a clip kills its links too (the FK chain reaches the shares).
func TestVideoShareCascadeFromParents(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)

	byWorkout := f.createShare(t, env, map[string]any{}, f.coachCookies)
	if rr := env.do(t, "DELETE", f.workoutPath, nil, f.coachCookies); rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Fatalf("delete workout: %d %s", rr.Code, rr.Body.String())
	}
	requireShareGone(t, env, byWorkout.Token)

	// A fresh workout, set and clip for the athlete, then delete the athlete.
	workout := env.createWorkout(t, f.athlete.ID, "2026-11-21")
	set, err := models.AddSet(t.Context(), env.DB, workout.ID, f.set.ExerciseID, 5, 135, 0, "reps", "", "")
	if err != nil {
		t.Fatal(err)
	}
	videoPath := fmt.Sprintf("/api/athletes/%d/workouts/%d/sets/%d/video", f.athlete.ID, workout.ID, set.ID)
	body, ctype := videoUpload(t, "video", fakeMP4(1024))
	requireStatus(t, env.doMultipart(t, "POST", videoPath, body, ctype, f.kidCookies), http.StatusCreated)
	rr := env.do(t, "POST", videoPath+"/shares", map[string]any{}, f.coachCookies)
	requireStatus(t, rr, http.StatusCreated)
	var byAthlete CreatedVideoShare
	decodeJSON(t, rr, &byAthlete)
	requireStatus(t, env.do(t, "GET", "/api/share/"+byAthlete.Token, nil, nil), http.StatusOK)

	admin := env.createUser(t, "admin", false, true)
	rr = env.do(t, "DELETE", fmt.Sprintf("/api/athletes/%d", f.athlete.ID), nil, env.loginAs(t, admin))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Fatalf("delete athlete: %d %s", rr.Code, rr.Body.String())
	}
	requireShareGone(t, env, byAthlete.Token)

	var n int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM set_video_shares`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("share rows after parent deletes = %d (err %v)", n, err)
	}
}

// TestVideoShareServiceTokenRefused: a coach's service token cannot list,
// create or revoke links, even on a clip the coach owns.
func TestVideoShareServiceTokenRefused(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)
	created := f.createShare(t, env, map[string]any{}, f.coachCookies)
	token := env.coachServiceToken(t, f.coach.ID)

	requireStatus(t, env.serveBearer(t, "POST", f.sharesPath, token, `{}`, nil), http.StatusForbidden)
	requireStatus(t, env.serveBearer(t, "GET", f.sharesPath, token, nil, nil), http.StatusForbidden)
	requireStatus(t, env.serveBearer(t, "DELETE", fmt.Sprintf("%s/%d", f.sharesPath, created.Share.ID), token, nil, nil), http.StatusForbidden)
	requireStatus(t, env.do(t, "GET", "/api/share/"+created.Token, nil, nil), http.StatusOK)
}

// TestVideoShareHostileTokens: tokens shaped like paths, other links' ids or
// encodings never open anything and get the single 404.
func TestVideoShareHostileTokens(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)
	created := f.createShare(t, env, map[string]any{}, f.coachCookies)
	good := env.do(t, "GET", "/api/share/missing", nil, nil).Body.String()

	for _, tok := range []string{
		"..%2F..%2Fetc%2Fpasswd",
		"%2e%2e",
		fmt.Sprint(created.Share.ID),
		created.Token[:42],
		created.Token + "A",
		strings.ToUpper(created.Token),
		created.Token + "%00",
		models.HashVideoShareToken(created.Token),
	} {
		rr := env.do(t, "GET", "/api/share/"+tok, nil, nil)
		if rr.Code != http.StatusNotFound || rr.Body.String() != good {
			t.Errorf("token %q: %d %q", tok, rr.Code, rr.Body.String())
		}
		rr = env.do(t, "GET", "/api/share/"+tok+"/video", nil, nil)
		if rr.Code != http.StatusNotFound || rr.Body.String() != good {
			t.Errorf("token %q video: %d %q", tok, rr.Code, rr.Body.String())
		}
	}
}

// TestVideoShareAccessLogHasNoToken: the structured access log names the
// link by id and never carries the token.
func TestVideoShareAccessLogHasNoToken(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)
	created := f.createShare(t, env, map[string]any{}, f.coachCookies)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	requireStatus(t, env.do(t, "GET", "/api/share/"+created.Token, nil, nil), http.StatusOK)
	requireStatus(t, env.do(t, "GET", "/api/share/"+created.Token+"/video", nil, nil), http.StatusOK)
	requireStatus(t, env.do(t, "GET", "/s/"+created.Token, nil, nil), http.StatusOK)
	requireStatus(t, env.do(t, "GET", "/api/share/"+strings.Repeat("B", 43), nil, nil), http.StatusNotFound)

	out := buf.String()
	if strings.Contains(out, created.Token) {
		t.Fatalf("access log carries the token:\n%s", out)
	}
	if got := strings.Count(out, fmt.Sprintf("share_id=%d", created.Share.ID)); got != 3 {
		t.Fatalf("share_id logged %d times, want 3:\n%s", got, out)
	}
	if !strings.Contains(out, "outcome=not_found") || !strings.Contains(out, "ip=") {
		t.Fatalf("access log lacks outcome or ip:\n%s", out)
	}
}
