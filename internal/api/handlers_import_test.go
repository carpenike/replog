package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// uploadFile builds a multipart/form-data body with a single "file" part
// (and optional extra form fields) and returns the assembled request body
// plus the matching Content-Type header value.
func uploadFile(t *testing.T, filename, content string, extraFields map[string]string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range extraFields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %q: %v", k, err)
		}
	}
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.WriteString(fw, content); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

// doMultipart issues a multipart upload request through the test router.
// It mirrors testEnv.do but writes the multipart Content-Type header itself.
func (e *testEnv) doMultipart(t *testing.T, method, path string, body io.Reader, contentType string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", contentType)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	e.Router.ServeHTTP(rr, req)
	return rr
}

const strongCSVSample = `Date,Workout Name,Duration,Exercise Name,Set Order,Weight,Reps,Distance,Seconds,Notes,Workout Notes,RPE
2026-04-01 08:00:00,Morning,30m,Bench Press,1,135,5,,,,,7
2026-04-01 08:00:00,Morning,30m,Bench Press,2,155,5,,,,,8
2026-04-01 08:00:00,Morning,30m,Squat,1,225,3,,,,,8
`

const hevyCSVSample = `title,start_time,end_time,description,exercise_title,superset_id,exercise_notes,set_index,set_type,weight_lbs,reps,distance_miles,duration_seconds,rpe
Morning,2026-04-01 08:00:00,2026-04-01 09:00:00,,Bench Press,,,1,normal,135,5,,,7
Morning,2026-04-01 08:00:00,2026-04-01 09:00:00,,Bench Press,,,2,normal,155,5,,,8
`

const replogJSONSample = `{
  "version": "1",
  "weight_unit": "lbs",
  "exercises": [
    {"name": "Imported Bench Press"}
  ],
  "workouts": [
    {
      "date": "2026-04-01",
      "sets": [
        {"exercise": "Imported Bench Press", "reps": 5, "weight": 135, "rpe": 7}
      ]
    }
  ]
}`

const catalogJSONSample = `{
  "version": "1",
  "type": "catalog",
  "exercises": [
    {"name": "Imported Squat"},
    {"name": "Imported Deadlift"}
  ],
  "equipment": [
    {"name": "Imported Squat Rack", "description": "Test rack"}
  ],
  "programs": []
}`

// --- Per-athlete workout import ---

func TestImportUpload_RequiresCoach(t *testing.T) {
	env := setupTest(t)
	athlete := env.createAthlete(t, "Charlie", 0)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	body, ct := uploadFile(t, "x.csv", strongCSVSample, map[string]string{"format": "strong"})
	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athlete.ID),
		body, ct, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestImportUpload_RequiresFile(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	// Multipart body with only the format field, no file.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("format", "strong")
	_ = w.Close()

	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athlete.ID),
		&buf, w.FormDataContentType(), cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestImportUpload_RejectsUnknownFormat(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body, ct := uploadFile(t, "x.csv", strongCSVSample, map[string]string{"format": "garbage"})
	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athlete.ID),
		body, ct, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestImportUpload_StrongCSV_ReturnsMappings(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body, ct := uploadFile(t, "strong.csv", strongCSVSample, map[string]string{"format": "strong"})
	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athlete.ID),
		body, ct, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got ImportUploadResponse
	decodeJSON(t, rr, &got)
	if got.Format != "strong" {
		t.Errorf("got format %q, want strong", got.Format)
	}
	// Sample contains 2 distinct exercises: Bench Press, Squat.
	if len(got.Exercises) != 2 {
		t.Errorf("got %d exercise mappings, want 2", len(got.Exercises))
	}
	names := map[string]bool{}
	for _, ex := range got.Exercises {
		names[ex.Name] = true
	}
	if !names["Bench Press"] || !names["Squat"] {
		t.Errorf("expected Bench Press and Squat in mappings, got %+v", got.Exercises)
	}
}

func TestImportUpload_HevyCSV_ReturnsMappings(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body, ct := uploadFile(t, "hevy.csv", hevyCSVSample, map[string]string{"format": "hevy"})
	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athlete.ID),
		body, ct, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got ImportUploadResponse
	decodeJSON(t, rr, &got)
	if got.Format != "hevy" {
		t.Errorf("got format %q, want hevy", got.Format)
	}
	if len(got.Exercises) == 0 {
		t.Error("expected at least one exercise mapping for Hevy import")
	}
}

func TestImportUpload_RepLogJSON_ReturnsMappings(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body, ct := uploadFile(t, "data.json", replogJSONSample, map[string]string{"format": "replog"})
	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athlete.ID),
		body, ct, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got ImportUploadResponse
	decodeJSON(t, rr, &got)
	if got.Format != "replog" {
		t.Errorf("got format %q, want replog", got.Format)
	}
}

func TestImportUpload_RejectsBadCSV(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body, ct := uploadFile(t, "x.csv", "this is not a csv", map[string]string{"format": "strong"})
	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athlete.ID),
		body, ct, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestImportExecute_RequiresUploadFirst(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/execute", athlete.ID),
		`{"exercises":[]}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
	if !strings.Contains(rr.Body.String(), "no import") {
		t.Errorf("expected 'no import' error, got %q", rr.Body.String())
	}
}

func TestImportExecute_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/athletes/1/import/execute",
		`{"exercises":[]}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- Catalog import (admin-only) ---

func TestCatalogImportUpload_AdminParsesAndCounts(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	body, ct := uploadFile(t, "catalog.json", catalogJSONSample, nil)
	rr := env.doMultipart(t, "POST", "/api/catalog/import/upload", body, ct, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got map[string]any
	decodeJSON(t, rr, &got)
	// Sample has 2 exercises, 1 piece of equipment, 0 programs.
	if got["exercises"].(float64) != 2 {
		t.Errorf("got exercises=%v, want 2", got["exercises"])
	}
	if got["equipment"].(float64) != 1 {
		t.Errorf("got equipment=%v, want 1", got["equipment"])
	}
}

func TestCatalogImportUpload_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	body, ct := uploadFile(t, "catalog.json", catalogJSONSample, nil)
	rr := env.doMultipart(t, "POST", "/api/catalog/import/upload", body, ct, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestCatalogImportUpload_RejectsBadJSON(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	body, ct := uploadFile(t, "catalog.json", "{not valid json", nil)
	rr := env.doMultipart(t, "POST", "/api/catalog/import/upload", body, ct, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestCatalogImportExecute_RequiresUploadFirst(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST", "/api/catalog/import/execute", `{}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestCatalogImportExecute_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", "/api/catalog/import/execute", `{}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- Catalog export ---

func TestCatalogExportJSON_AdminReturnsJSON(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	env.createExercise(t, "Bench Press")
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", "/api/catalog/export", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), "Bench Press") {
		t.Errorf("expected exported catalog to include Bench Press, got %s", rr.Body.String()[:min(200, len(rr.Body.String()))])
	}
}

func TestCatalogExportJSON_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", "/api/catalog/export", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- IDOR coverage (issue #5) ---

func TestImportUpload_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	body, ct := uploadFile(t, "x.csv", strongCSVSample, map[string]string{"format": "strong"})
	rr := env.doMultipart(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/upload", athleteOfA.ID),
		body, ct, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestImportExecute_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/import/execute", athleteOfA.ID),
		`{}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}
