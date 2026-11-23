package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// shareFixture is a videoFixture whose set already has a clip.
type shareFixture struct {
	*videoFixture
	clip       []byte
	sharesPath string
}

func newShareFixture(t *testing.T, env *testEnv) *shareFixture {
	t.Helper()
	f := &shareFixture{videoFixture: newVideoFixture(t, env), clip: fakeMP4(4096)}
	body, ctype := videoUpload(t, "video", f.clip)
	requireStatus(t, env.doMultipart(t, "POST", f.videoPath, body, ctype, f.kidCookies), http.StatusCreated)
	f.sharesPath = f.videoPath + "/shares"
	return f
}

// createShare creates a link and returns the decoded response.
func (f *shareFixture) createShare(t *testing.T, env *testEnv, body any, cookies []*http.Cookie) CreatedVideoShare {
	t.Helper()
	rr := env.do(t, "POST", f.sharesPath, body, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var out CreatedVideoShare
	decodeJSON(t, rr, &out)
	return out
}

// withShareClock pins the share-link clock for one test.
func withShareClock(t *testing.T, now *time.Time) {
	t.Helper()
	prev := shareNow
	shareNow = func() time.Time { return *now }
	t.Cleanup(func() { shareNow = prev })
}

// getRange GETs path with a Range header and no cookies.
func getRange(env *testEnv, path, rng string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Range", rng)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	return rec
}

// requireShareGone checks every public route answers the single 404 for a
// token and returns the API body so callers can compare cases.
func requireShareGone(t *testing.T, env *testEnv, token string) string {
	t.Helper()
	meta := env.do(t, "GET", "/api/share/"+token, nil, nil)
	requireStatus(t, meta, http.StatusNotFound)
	video := env.do(t, "GET", "/api/share/"+token+"/video", nil, nil)
	requireStatus(t, video, http.StatusNotFound)
	if meta.Body.String() != video.Body.String() {
		t.Fatalf("meta and video 404 bodies differ: %q vs %q", meta.Body.String(), video.Body.String())
	}
	page := env.do(t, "GET", "/s/"+token, nil, nil)
	requireStatus(t, page, http.StatusNotFound)
	if page.Body.String() != testShareIndexHTML {
		t.Fatalf("page 404 body = %q, want the SPA shell", page.Body.String())
	}
	for _, rr := range []*httptest.ResponseRecorder{meta, video, page} {
		if got := rr.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Fatalf("404 Cache-Control = %q", got)
		}
	}
	return meta.Body.String()
}

// TestVideoShareLifecycle: create a link, open it with no login (metadata,
// full and Range streams, viewer page), list it, revoke it, and it is gone.
func TestVideoShareLifecycle(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)
	now := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	withShareClock(t, &now)

	created := f.createShare(t, env, map[string]any{"label": "  Grandpa  "}, f.coachCookies)
	if len(created.Token) != 43 || created.URL != "/s/"+created.Token {
		t.Fatalf("created = %+v", created)
	}
	if created.Share.Status != "active" || created.Share.Label == nil || *created.Share.Label != "Grandpa" {
		t.Fatalf("created share = %+v", created.Share)
	}
	if created.Share.ExpiresAt != now.Add(7*24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("default expiry = %s", created.Share.ExpiresAt)
	}
	if created.Share.CreatedBy == nil || *created.Share.CreatedBy != "coach" {
		t.Fatalf("created_by = %v", created.Share.CreatedBy)
	}

	// Only the hash is stored; no column holds the token.
	var storedHash string
	if err := env.DB.QueryRow(`SELECT token_hash FROM set_video_shares WHERE id = ?`, created.Share.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash != models.HashVideoShareToken(created.Token) || storedHash == created.Token {
		t.Fatalf("stored %q, want the SHA-256 of the token", storedHash)
	}
	var leaks int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM set_video_shares WHERE token_hash = ? OR label = ?`,
		created.Token, created.Token).Scan(&leaks); err != nil || leaks != 0 {
		t.Fatalf("token found in the table (%d rows, err %v)", leaks, err)
	}
	var audits int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'video_share_create' AND details NOT LIKE ?`,
		"%"+created.Token+"%").Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("create audit rows = %d (err %v)", audits, err)
	}

	// Metadata, with no login: exercise and date, nothing about the athlete.
	rr := env.do(t, "GET", "/api/share/"+created.Token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	var meta SharedVideo
	decodeJSON(t, rr, &meta)
	if meta.ExerciseName != "Back Squat" || meta.Date != "2026-11-20" || meta.ContentType != "video/mp4" ||
		meta.VideoURL != "/api/share/"+created.Token+"/video" || meta.ExpiresAt != created.Share.ExpiresAt {
		t.Fatalf("shared metadata = %+v", meta)
	}
	if strings.Contains(rr.Body.String(), "Sam") || strings.Contains(rr.Body.String(), "athlete") {
		t.Fatalf("shared metadata leaks the athlete: %s", rr.Body.String())
	}
	for h, want := range map[string]string{
		"Cache-Control":          "private, no-store",
		"Referrer-Policy":        "no-referrer",
		"X-Robots-Tag":           "noindex, nofollow, noarchive",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := rr.Header().Get(h); got != want {
			t.Errorf("metadata %s = %q, want %q", h, got, want)
		}
	}

	// The clip, full and by Range.
	rr = env.do(t, "GET", meta.VideoURL, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	if rr.Header().Get("Content-Type") != "video/mp4" || !bytes.Equal(rr.Body.Bytes(), f.clip) {
		t.Fatalf("shared stream: type=%q len=%d", rr.Header().Get("Content-Type"), rr.Body.Len())
	}
	if rr.Header().Get("Cache-Control") != "private, no-store" || rr.Header().Get("X-Robots-Tag") == "" {
		t.Fatalf("shared stream headers = %v", rr.Header())
	}
	rec := getRange(env, meta.VideoURL, "bytes=0-99")
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 || !bytes.Equal(rec.Body.Bytes(), f.clip[:100]) {
		t.Fatalf("shared range: status=%d len=%d", rec.Code, rec.Body.Len())
	}

	// The viewer page.
	rr = env.do(t, "GET", created.URL, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	if rr.Body.String() != testShareIndexHTML || !strings.Contains(rr.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatalf("viewer page: body=%q csp=%q", rr.Body.String(), rr.Header().Get("Content-Security-Policy"))
	}

	// The owner list: one active link, two views (metadata + plain GET; the
	// Range request and the page do not count).
	now = now.Add(time.Hour)
	rr = env.do(t, "GET", f.sharesPath, nil, f.kidCookies)
	requireStatus(t, rr, http.StatusOK)
	var list VideoShareList
	decodeJSON(t, rr, &list)
	if !list.SharingEnabled || len(list.Shares) != 1 {
		t.Fatalf("list = %+v", list)
	}
	if s := list.Shares[0]; s.AccessCount != 2 || s.LastAccessedAt == nil || s.Status != "active" {
		t.Fatalf("listed share = %+v", s)
	}
	if strings.Contains(rr.Body.String(), created.Token) || strings.Contains(rr.Body.String(), storedHash) {
		t.Fatal("list response carries the token or its hash")
	}

	// A second link, then revoke the first: only it stops working.
	second := f.createShare(t, env, map[string]any{"expires_in_hours": 24}, f.kidCookies)
	rr = env.do(t, "DELETE", fmt.Sprintf("%s/%d", f.sharesPath, created.Share.ID), nil, f.kidCookies)
	requireStatus(t, rr, http.StatusOK)
	var revoked VideoShare
	decodeJSON(t, rr, &revoked)
	if revoked.Status != "revoked" || revoked.RevokedAt == nil {
		t.Fatalf("revoked = %+v", revoked)
	}
	requireShareGone(t, env, created.Token)
	requireStatus(t, env.do(t, "GET", "/api/share/"+second.Token, nil, nil), http.StatusOK)

	// Revoking again is a no-op; an unknown id is a 404.
	requireStatus(t, env.do(t, "DELETE", fmt.Sprintf("%s/%d", f.sharesPath, created.Share.ID), nil, f.kidCookies), http.StatusOK)
	requireStatus(t, env.do(t, "DELETE", f.sharesPath+"/99999", nil, f.kidCookies), http.StatusNotFound)
	requireStatus(t, env.do(t, "DELETE", f.sharesPath+"/abc", nil, f.kidCookies), http.StatusBadRequest)

	// Active-only by default; include_inactive shows the revoked one too.
	rr = env.do(t, "GET", f.sharesPath, nil, f.coachCookies)
	decodeJSON(t, rr, &list)
	if len(list.Shares) != 1 || list.Shares[0].ID != second.Share.ID {
		t.Fatalf("active list = %+v", list.Shares)
	}
	rr = env.do(t, "GET", f.sharesPath+"?include_inactive=true", nil, f.coachCookies)
	decodeJSON(t, rr, &list)
	if len(list.Shares) != 2 || list.Shares[1].Status != "revoked" {
		t.Fatalf("full list = %+v", list.Shares)
	}
}

// TestVideoShareExpiry covers the allowed lifetimes, the 30-day cap, and
// that expired, revoked, unknown and malformed tokens are indistinguishable.
func TestVideoShareExpiry(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)
	now := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	withShareClock(t, &now)

	for _, hours := range []int{24, 168, 720} {
		c := f.createShare(t, env, map[string]any{"expires_in_hours": hours}, f.coachCookies)
		if want := now.Add(time.Duration(hours) * time.Hour).Format(time.RFC3339); c.Share.ExpiresAt != want {
			t.Fatalf("%dh link expires %s, want %s", hours, c.Share.ExpiresAt, want)
		}
	}
	for _, hours := range []int{721, 1000, 24 * 365, -1, 48, 1} {
		rr := env.do(t, "POST", f.sharesPath, map[string]any{"expires_in_hours": hours}, f.coachCookies)
		requireStatus(t, rr, http.StatusBadRequest)
	}
	rr := env.do(t, "POST", f.sharesPath, map[string]any{"label": strings.Repeat("x", 101)}, f.coachCookies)
	requireStatus(t, rr, http.StatusBadRequest)
	requireStatus(t, env.do(t, "POST", f.sharesPath, `{not json`, f.coachCookies), http.StatusBadRequest)

	day := f.createShare(t, env, map[string]any{"expires_in_hours": 24}, f.coachCookies)
	revoked := f.createShare(t, env, map[string]any{}, f.coachCookies)
	requireStatus(t, env.do(t, "DELETE", fmt.Sprintf("%s/%d", f.sharesPath, revoked.Share.ID), nil, f.coachCookies), http.StatusOK)

	// Still good one second before expiry, gone at expiry.
	now = now.Add(24*time.Hour - time.Second)
	requireStatus(t, env.do(t, "GET", "/api/share/"+day.Token, nil, nil), http.StatusOK)
	now = now.Add(time.Second)
	expiredBody := requireShareGone(t, env, day.Token)

	revokedBody := requireShareGone(t, env, revoked.Token)
	unknownBody := requireShareGone(t, env, strings.Repeat("A", 43))
	malformedBody := requireShareGone(t, env, "not-a-token")
	tooLongBody := requireShareGone(t, env, strings.Repeat("A", 500))
	for name, body := range map[string]string{"revoked": revokedBody, "unknown": unknownBody, "malformed": malformedBody, "too long": tooLongBody} {
		if body != expiredBody {
			t.Errorf("%s body %q differs from expired body %q", name, body, expiredBody)
		}
	}

	// Both 24-hour links are now listed as expired (only with include_inactive).
	var list VideoShareList
	decodeJSON(t, env.do(t, "GET", f.sharesPath+"?include_inactive=1", nil, f.coachCookies), &list)
	statuses := map[string]int{}
	for _, s := range list.Shares {
		statuses[s.Status]++
	}
	if statuses["expired"] != 2 || statuses["revoked"] != 1 || statuses["active"] != 2 {
		t.Fatalf("statuses = %v", statuses)
	}
}

// TestVideoShareAccessControl: only the athlete, their coach or an admin may
// manage links; everyone else gets 403, and a foreign set is a 404.
func TestVideoShareAccessControl(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)

	// A non-coach linked to another athlete, and a coach of another athlete.
	otherAthlete := env.createAthlete(t, "Alex", f.coach.ID)
	other := env.createUser(t, "alex", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, otherAthlete.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	otherCookies := env.loginAs(t, other)
	otherCoach := env.createUser(t, "coach2", true, false)
	otherCoachCookies := env.loginAs(t, otherCoach)
	unlinked := env.createUser(t, "visitor", false, false)
	unlinkedCookies := env.loginAs(t, unlinked)

	created := f.createShare(t, env, map[string]any{}, f.kidCookies)
	revokePath := fmt.Sprintf("%s/%d", f.sharesPath, created.Share.ID)
	for name, cookies := range map[string][]*http.Cookie{"other athlete": otherCookies, "other coach": otherCoachCookies, "unlinked": unlinkedCookies} {
		requireStatus(t, env.do(t, "POST", f.sharesPath, map[string]any{}, cookies), http.StatusForbidden)
		requireStatus(t, env.do(t, "GET", f.sharesPath, nil, cookies), http.StatusForbidden)
		if rr := env.do(t, "DELETE", revokePath, nil, cookies); rr.Code != http.StatusForbidden {
			t.Fatalf("%s revoke: %d", name, rr.Code)
		}
	}
	requireStatus(t, env.do(t, "POST", f.sharesPath, map[string]any{}, nil), http.StatusUnauthorized)
	requireStatus(t, env.do(t, "GET", f.sharesPath, nil, nil), http.StatusUnauthorized)

	// An admin can share.
	admin := env.createUser(t, "admin", false, true)
	f.createShare(t, env, map[string]any{}, env.loginAs(t, admin))

	// IDOR: the other athlete pairs their own athlete id with this set.
	other2Workout := env.createWorkout(t, otherAthlete.ID, "2026-11-21")
	foreign := fmt.Sprintf("/api/athletes/%d/workouts/%d/sets/%d/video/shares", otherAthlete.ID, f.workout.ID, f.set.ID)
	requireStatus(t, env.do(t, "POST", foreign, map[string]any{}, otherCookies), http.StatusNotFound)
	requireStatus(t, env.do(t, "GET", foreign, nil, otherCookies), http.StatusNotFound)
	requireStatus(t, env.do(t, "DELETE", fmt.Sprintf("%s/%d", foreign, created.Share.ID), nil, otherCookies), http.StatusNotFound)
	mixed := fmt.Sprintf("/api/athletes/%d/workouts/%d/sets/%d/video/shares", otherAthlete.ID, other2Workout.ID, f.set.ID)
	requireStatus(t, env.do(t, "GET", mixed, nil, otherCookies), http.StatusNotFound)

	// A link id from another clip is not revocable through this clip.
	squat := env.createExercise(t, "Front Squat")
	otherSet, err := models.AddSet(t.Context(), env.DB, other2Workout.ID, squat.ID, 3, 95, 0, "reps", "", "")
	if err != nil {
		t.Fatal(err)
	}
	otherVideo := fmt.Sprintf("/api/athletes/%d/workouts/%d/sets/%d/video", otherAthlete.ID, other2Workout.ID, otherSet.ID)
	body, ctype := videoUpload(t, "video", fakeMP4(1024))
	requireStatus(t, env.doMultipart(t, "POST", otherVideo, body, ctype, otherCookies), http.StatusCreated)
	requireStatus(t, env.do(t, "DELETE", fmt.Sprintf("%s/shares/%d", otherVideo, created.Share.ID), nil, otherCookies), http.StatusNotFound)
	requireStatus(t, env.do(t, "GET", "/api/share/"+created.Token, nil, nil), http.StatusOK)

	// A set without a clip has nothing to share.
	requireStatus(t, env.do(t, "DELETE", otherVideo, nil, otherCookies), http.StatusOK)
	requireStatus(t, env.do(t, "POST", otherVideo+"/shares", map[string]any{}, otherCookies), http.StatusNotFound)
	requireStatus(t, env.do(t, "GET", otherVideo+"/shares", nil, otherCookies), http.StatusNotFound)
}

// TestVideoShareCascade: deleting or replacing the clip kills its links.
func TestVideoShareCascade(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)

	before := f.createShare(t, env, map[string]any{}, f.coachCookies)
	body, ctype := videoUpload(t, "video", fakeMP4(2048))
	requireStatus(t, env.doMultipart(t, "POST", f.videoPath, body, ctype, f.kidCookies), http.StatusCreated)
	requireShareGone(t, env, before.Token)

	after := f.createShare(t, env, map[string]any{}, f.coachCookies)
	requireStatus(t, env.do(t, "GET", "/api/share/"+after.Token, nil, nil), http.StatusOK)
	requireStatus(t, env.do(t, "DELETE", f.videoPath, nil, f.kidCookies), http.StatusOK)
	requireShareGone(t, env, after.Token)

	var n int
	if err := env.DB.QueryRow(`SELECT COUNT(*) FROM set_video_shares`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("share rows after delete = %d (err %v)", n, err)
	}
}

// TestVideoShareKillSwitch: REPLOG_VIDEO_SHARING_DISABLED refuses new links
// and makes every existing one a 404; owners can still list and revoke.
func TestVideoShareKillSwitch(t *testing.T) {
	env := setupTest(t)
	f := newShareFixture(t, env)
	created := f.createShare(t, env, map[string]any{}, f.coachCookies)

	env.Handlers.VideoSharingDisabled = true
	requireStatus(t, env.do(t, "POST", f.sharesPath, map[string]any{}, f.coachCookies), http.StatusForbidden)
	requireShareGone(t, env, created.Token)
	var list VideoShareList
	rr := env.do(t, "GET", f.sharesPath, nil, f.coachCookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &list)
	if list.SharingEnabled || len(list.Shares) != 1 {
		t.Fatalf("disabled list = %+v", list)
	}
	requireStatus(t, env.do(t, "DELETE", fmt.Sprintf("%s/%d", f.sharesPath, created.Share.ID), nil, f.coachCookies), http.StatusOK)

	// Turning sharing back on does not revive a revoked link.
	env.Handlers.VideoSharingDisabled = false
	requireShareGone(t, env, created.Token)
}
