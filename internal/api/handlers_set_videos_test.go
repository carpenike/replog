package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// fakeMP4 is the start of an ISO base media file (ftyp isom) followed by
// filler, enough for the server's container sniff.
func fakeMP4(size int) []byte {
	head := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 2, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '1'}
	if size < len(head) {
		size = len(head)
	}
	return append(head, bytes.Repeat([]byte{0x42}, size-len(head))...)
}

// videoUpload builds a multipart body with one file part.
func videoUpload(t *testing.T, field string, content []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("note", "ignored")
	fw, err := w.CreateFormFile(field, "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(content)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

// videoFixture is an athlete with a linked user, a workout and one set.
type videoFixture struct {
	coach, kid   *models.User
	athlete      *models.Athlete
	workout      *models.Workout
	set          *models.WorkoutSet
	coachCookies []*http.Cookie
	kidCookies   []*http.Cookie
	setPath      string
	videoPath    string
	workoutPath  string
}

func newVideoFixture(t *testing.T, env *testEnv) *videoFixture {
	t.Helper()
	f := &videoFixture{}
	f.coach = env.createUser(t, "coach", true, false)
	f.athlete = env.createAthlete(t, "Sam", f.coach.ID)
	f.kid = env.createUser(t, "sam", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, f.athlete.ID, f.kid.ID); err != nil {
		t.Fatal(err)
	}
	squat := env.createExercise(t, "Back Squat")
	f.workout = env.createWorkout(t, f.athlete.ID, "2026-11-20")
	s, err := models.AddSet(context.Background(), env.DB, f.workout.ID, squat.ID, 5, 135, 0, "reps", "", "")
	if err != nil {
		t.Fatal(err)
	}
	f.set = s
	f.coachCookies = env.loginAs(t, f.coach)
	f.kidCookies = env.loginAs(t, f.kid)
	f.workoutPath = fmt.Sprintf("/api/athletes/%d/workouts/%d", f.athlete.ID, f.workout.ID)
	f.setPath = fmt.Sprintf("%s/sets/%d", f.workoutPath, s.ID)
	f.videoPath = f.setPath + "/video"
	return f
}

func videoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestSetVideoLifecycle covers gap 12 end to end: an athlete attaches a clip
// to their set, it shows on the workout read and in pending reviews, it is
// streamed with auth (and Range), the coach comments, a re-upload replaces
// the file, and delete removes the row and the file.
func TestSetVideoLifecycle(t *testing.T) {
	env := setupTest(t)
	f := newVideoFixture(t, env)
	clip := fakeMP4(4096)

	body, ctype := videoUpload(t, "video", clip)
	rr := env.doMultipart(t, "POST", f.videoPath, body, ctype, f.kidCookies)
	requireStatus(t, rr, http.StatusCreated)
	var v SetVideo
	decodeJSON(t, rr, &v)
	if v.ContentType != "video/mp4" || v.SizeBytes != int64(len(clip)) || v.URL != f.videoPath || v.SetID != f.set.ID {
		t.Fatalf("uploaded video = %+v", v)
	}
	files := videoFiles(t, env.Handlers.VideoDir)
	if len(files) != 1 {
		t.Fatalf("video dir holds %v, want one file", files)
	}
	firstFile := files[0]

	// The workout read carries the clip on its set.
	rr = env.do(t, "GET", f.workoutPath, nil, f.coachCookies)
	requireStatus(t, rr, http.StatusOK)
	var detail WorkoutDetailResponse
	decodeJSON(t, rr, &detail)
	if got := detail.Groups[0].Sets[0].Video; got == nil || got.URL != f.videoPath {
		t.Fatalf("workout set video = %+v", got)
	}

	// The coach's pending-review list counts it.
	rr = env.do(t, "GET", "/api/reviews/pending", nil, f.coachCookies)
	requireStatus(t, rr, http.StatusOK)
	var pending []UnreviewedWorkout
	decodeJSON(t, rr, &pending)
	if len(pending) != 1 || pending[0].VideoCount != 1 {
		t.Fatalf("pending = %+v", pending)
	}

	// Streaming: full read, then a Range read.
	rr = env.do(t, "GET", f.videoPath, nil, f.coachCookies)
	requireStatus(t, rr, http.StatusOK)
	if rr.Header().Get("Content-Type") != "video/mp4" || !bytes.Equal(rr.Body.Bytes(), clip) {
		t.Fatalf("stream: type=%q len=%d", rr.Header().Get("Content-Type"), rr.Body.Len())
	}
	req := httptest.NewRequest("GET", f.videoPath, nil)
	req.Header.Set("Range", "bytes=0-99")
	for _, c := range f.kidCookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 {
		t.Fatalf("range: status=%d len=%d", rec.Code, rec.Body.Len())
	}

	// Unauthenticated callers get nothing.
	rr = env.do(t, "GET", f.videoPath, nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)

	// Only a coach comments.
	rr = env.do(t, "PUT", f.videoPath+"/comment", `{"comment":"nice depth"}`, f.kidCookies)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.do(t, "PUT", f.videoPath+"/comment", `{"comment":"  Knees out on rep 3.  "}`, f.coachCookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &v)
	if v.CoachComment == nil || *v.CoachComment != "Knees out on rep 3." || v.CommentedBy == nil || v.CommentedAt == nil {
		t.Fatalf("commented video = %+v", v)
	}
	long := bytes.Repeat([]byte("x"), models.MaxVideoCommentLen+1)
	rr = env.do(t, "PUT", f.videoPath+"/comment", map[string]string{"comment": string(long)}, f.coachCookies)
	requireStatus(t, rr, http.StatusBadRequest)

	// Re-upload replaces the file and clears the comment.
	body, ctype = videoUpload(t, "video", fakeMP4(2048))
	rr = env.doMultipart(t, "POST", f.videoPath, body, ctype, f.coachCookies)
	requireStatus(t, rr, http.StatusCreated)
	v = SetVideo{}
	decodeJSON(t, rr, &v)
	if v.CoachComment != nil || v.SizeBytes != 2048 {
		t.Fatalf("replaced video = %+v", v)
	}
	files = videoFiles(t, env.Handlers.VideoDir)
	if len(files) != 1 || files[0] == firstFile {
		t.Fatalf("after replace video dir holds %v (first was %s)", files, firstFile)
	}

	// Delete removes the row and the file.
	rr = env.do(t, "DELETE", f.videoPath, nil, f.kidCookies)
	requireStatus(t, rr, http.StatusOK)
	if files = videoFiles(t, env.Handlers.VideoDir); len(files) != 0 {
		t.Fatalf("after delete video dir holds %v", files)
	}
	rr = env.do(t, "GET", f.videoPath, nil, f.kidCookies)
	requireStatus(t, rr, http.StatusNotFound)
	rr = env.do(t, "DELETE", f.videoPath, nil, f.kidCookies)
	requireStatus(t, rr, http.StatusNotFound)
}

// TestSetVideoUploadChecks covers the type and size checks.
func TestSetVideoUploadChecks(t *testing.T) {
	env := setupTest(t)
	f := newVideoFixture(t, env)
	env.Handlers.VideoMaxBytes = 8 << 10

	cases := []struct {
		name    string
		field   string
		content []byte
		want    int
	}{
		{"html disguised as mp4", "video", []byte("<html><script>alert(1)</script></html>"), http.StatusUnsupportedMediaType},
		{"jpeg", "video", append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{1}, 64)...), http.StatusUnsupportedMediaType},
		{"too large", "video", fakeMP4(64 << 10), http.StatusRequestEntityTooLarge},
		{"wrong field", "file", fakeMP4(1024), http.StatusBadRequest},
		{"empty", "video", nil, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, ctype := videoUpload(t, c.field, c.content)
			rr := env.doMultipart(t, "POST", f.videoPath, body, ctype, f.kidCookies)
			requireStatus(t, rr, c.want)
		})
	}
	rr := env.do(t, "POST", f.videoPath, `{"video":"nope"}`, f.kidCookies)
	requireStatus(t, rr, http.StatusBadRequest)
	if files := videoFiles(t, env.Handlers.VideoDir); len(files) != 0 {
		t.Fatalf("refused uploads left %v on disk", files)
	}

	// No storage configured → 503.
	env.Handlers.VideoDir = ""
	body, ctype := videoUpload(t, "video", fakeMP4(1024))
	rr = env.doMultipart(t, "POST", f.videoPath, body, ctype, f.kidCookies)
	requireStatus(t, rr, http.StatusServiceUnavailable)
}

// TestSetVideoAccess covers object-level scoping (ADR 021), a sibling
// athlete's access, and the service-token refusal on writes.
func TestSetVideoAccess(t *testing.T) {
	env := setupTest(t)
	f := newVideoFixture(t, env)
	body, ctype := videoUpload(t, "video", fakeMP4(1024))
	requireStatus(t, env.doMultipart(t, "POST", f.videoPath, body, ctype, f.kidCookies), http.StatusCreated)

	// A sibling athlete user cannot see or touch it.
	sibling := env.createAthlete(t, "Alex", f.coach.ID)
	sib := env.createUser(t, "alex", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, sibling.ID, sib.ID); err != nil {
		t.Fatal(err)
	}
	sibCookies := env.loginAs(t, sib)
	requireStatus(t, env.do(t, "GET", f.videoPath, nil, sibCookies), http.StatusForbidden)
	requireStatus(t, env.do(t, "DELETE", f.videoPath, nil, sibCookies), http.StatusForbidden)

	// Pairing the sibling's own athlete ID with Sam's workout/set → 404.
	foreign := fmt.Sprintf("/api/athletes/%d/workouts/%d/sets/%d/video", sibling.ID, f.workout.ID, f.set.ID)
	requireStatus(t, env.do(t, "GET", foreign, nil, sibCookies), http.StatusNotFound)
	body, ctype = videoUpload(t, "video", fakeMP4(1024))
	requireStatus(t, env.doMultipart(t, "POST", foreign, body, ctype, sibCookies), http.StatusNotFound)
	requireStatus(t, env.do(t, "DELETE", foreign, nil, sibCookies), http.StatusNotFound)

	// A set from another workout of the same athlete under this workout's path → 404.
	other := env.createWorkout(t, f.athlete.ID, "2026-11-21")
	mismatch := fmt.Sprintf("/api/athletes/%d/workouts/%d/sets/%d/video", f.athlete.ID, other.ID, f.set.ID)
	requireStatus(t, env.do(t, "GET", mismatch, nil, f.kidCookies), http.StatusNotFound)

	// A coach service token can watch but not change clips.
	token := env.coachServiceToken(t, f.coach.ID)
	rr := env.serveBearer(t, "GET", f.videoPath, token, nil, nil)
	requireStatus(t, rr, http.StatusOK)
	requireStatus(t, env.serveBearer(t, "DELETE", f.videoPath, token, nil, nil), http.StatusForbidden)
	requireStatus(t, env.serveBearer(t, "PUT", f.videoPath+"/comment", token, `{"comment":"x"}`, nil), http.StatusForbidden)
}

// TestSetVideoFilesFollowDeletes checks that deleting the set or the
// workout removes the clip file, not just the row.
func TestSetVideoFilesFollowDeletes(t *testing.T) {
	env := setupTest(t)
	f := newVideoFixture(t, env)
	body, ctype := videoUpload(t, "video", fakeMP4(1024))
	requireStatus(t, env.doMultipart(t, "POST", f.videoPath, body, ctype, f.kidCookies), http.StatusCreated)
	requireStatus(t, env.do(t, "DELETE", f.setPath, nil, f.kidCookies), http.StatusOK)
	if files := videoFiles(t, env.Handlers.VideoDir); len(files) != 0 {
		t.Fatalf("set delete left %v", files)
	}

	s, err := models.AddSet(context.Background(), env.DB, f.workout.ID, f.set.ExerciseID, 5, 135, 0, "reps", "", "")
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("%s/sets/%d/video", f.workoutPath, s.ID)
	body, ctype = videoUpload(t, "video", fakeMP4(1024))
	requireStatus(t, env.doMultipart(t, "POST", path, body, ctype, f.kidCookies), http.StatusCreated)
	requireStatus(t, env.do(t, "DELETE", f.workoutPath, nil, f.coachCookies), http.StatusOK)
	if files := videoFiles(t, env.Handlers.VideoDir); len(files) != 0 {
		t.Fatalf("workout delete left %v", files)
	}
	// Nothing was ever written to the public avatar directory.
	if files := videoFiles(t, env.Handlers.AvatarDir); len(files) != 0 {
		t.Fatalf("avatar dir holds %v", files)
	}
}
