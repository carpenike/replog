package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestHealthImport covers gap 14: the generic samples shape and the Health
// Auto Export shape land in bio_samples as watch_import rows, re-sending is
// idempotent, bad readings are skipped and listed, unsupported Health Auto
// Export series are named once, and access matches the other athlete routes
// (session or a service token that can see the athlete).
func TestHealthImport(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	other := env.createAthlete(t, "Unowned", 0)
	path := fmt.Sprintf("/api/athletes/%d/bio-samples/import", athlete.ID)
	token := env.coachServiceToken(t, coach.ID)

	body := `{
	  "samples": [
	    {"metric":"resting_hr","value":52,"unit":"bpm","recorded_at":"2026-11-21T06:30:00-05:00"},
	    {"metric":"sleep","value":450,"unit":"min","recorded_at":"2026-11-21"},
	    {"metric":"vo2max","value":44,"recorded_at":"2026-11-21"},
	    {"metric":"hrv","value":9000,"recorded_at":"2026-11-21"}
	  ],
	  "data": {"metrics": [
	    {"name":"heart_rate_variability","units":"ms","data":[
	      {"date":"2026-11-20 00:00:00 -0500","qty":58.4},
	      {"date":"2026-11-21 00:00:00 -0500","qty":61}
	    ]},
	    {"name":"step_count","units":"count","data":[{"date":"2026-11-20 00:00:00 -0500","qty":9123}]},
	    {"name":"sleep_analysis","units":"hr","data":[{"date":"2026-11-20 00:00:00 -0500","asleep":6.9,"inBed":7.5}]},
	    {"name":"active_energy","units":"kcal","data":[{"date":"2026-11-20 00:00:00 -0500","qty":500}]}
	  ]}
	}`

	rr := env.serveBearer(t, "POST", path, token, body, nil)
	requireStatus(t, rr, http.StatusOK)
	var got BioSampleImportResponse
	decodeJSON(t, rr, &got)
	if got.Received != 8 || got.Inserted != 6 || got.Duplicates != 0 || got.Skipped != 2 {
		t.Fatalf("first import = %+v", got)
	}
	if len(got.Errors) != 2 || got.Errors[0].Index != 2 || !strings.Contains(got.Errors[0].Reason, "unsupported") ||
		got.Errors[1].Index != 3 || !strings.Contains(got.Errors[1].Reason, "outside") {
		t.Fatalf("errors = %+v", got.Errors)
	}
	if len(got.IgnoredMetrics) != 1 || got.IgnoredMetrics[0] != "active_energy" {
		t.Fatalf("ignored = %v", got.IgnoredMetrics)
	}

	// Re-sending the same window inserts nothing.
	rr = env.serveBearer(t, "POST", path, token, body, nil)
	requireStatus(t, rr, http.StatusOK)
	got = BioSampleImportResponse{}
	decodeJSON(t, rr, &got)
	if got.Inserted != 0 || got.Duplicates != 6 {
		t.Fatalf("re-import = %+v", got)
	}

	// The rows are watch_import, canonical metrics and units.
	cookies := env.loginAs(t, coach)
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/bio-samples", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var samples []BioSample
	decodeJSON(t, rr, &samples)
	if len(samples) != 6 {
		t.Fatalf("stored %d samples, want 6", len(samples))
	}
	metrics := map[string]int{}
	for _, s := range samples {
		if s.Source != "watch_import" {
			t.Fatalf("source = %q", s.Source)
		}
		metrics[s.Metric]++
		if s.Metric == "sleep_hours" && s.Value != 7.5 && s.Value != 6.9 {
			t.Fatalf("sleep value = %v", s.Value)
		}
	}
	if metrics["hrv"] != 2 || metrics["resting_hr"] != 1 || metrics["steps"] != 1 || metrics["sleep_hours"] != 2 {
		t.Fatalf("metrics = %v", metrics)
	}

	// A browser session works too, and overlaps are duplicates.
	rr = env.do(t, "POST", path, `{"samples":[{"metric":"resting_heart_rate","value":52,"recorded_at":"2026-11-21 11:30:00 +0000"}]}`, cookies)
	requireStatus(t, rr, http.StatusOK)
	got = BioSampleImportResponse{}
	decodeJSON(t, rr, &got)
	if got.Duplicates != 1 || got.Inserted != 0 {
		t.Fatalf("session overlap = %+v", got)
	}

	// Scoping: a service token cannot import for an athlete it cannot see.
	rr = env.serveBearer(t, "POST", fmt.Sprintf("/api/athletes/%d/bio-samples/import", other.ID), token,
		`{"samples":[{"metric":"hrv","value":50,"recorded_at":"2026-11-21"}]}`, nil)
	requireStatus(t, rr, http.StatusForbidden)

	// An athlete user imports only for their own athlete.
	kid := env.createUser(t, "kid", false, false)
	if _, err := env.DB.Exec(`UPDATE users SET athlete_id = ? WHERE id = ?`, other.ID, kid.ID); err != nil {
		t.Fatal(err)
	}
	kidCookies := env.loginAs(t, kid)
	rr = env.do(t, "POST", path, `{"samples":[{"metric":"hrv","value":50,"recorded_at":"2026-11-21"}]}`, kidCookies)
	requireStatus(t, rr, http.StatusForbidden)
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/bio-samples/import", other.ID),
		`{"samples":[{"metric":"steps","value":4000,"recorded_at":"2026-11-21"}]}`, kidCookies)
	requireStatus(t, rr, http.StatusOK)

	// Unauthenticated → 401; empty or malformed → 400.
	requireStatus(t, env.do(t, "POST", path, `{"samples":[]}`, nil), http.StatusUnauthorized)
	requireStatus(t, env.do(t, "POST", path, `{"samples":[]}`, cookies), http.StatusBadRequest)
	requireStatus(t, env.do(t, "POST", path, `not json`, cookies), http.StatusBadRequest)
}

// TestHealthImportLimits covers the per-request sample cap.
func TestHealthImportLimits(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Ryan", coach.ID)
	cookies := env.loginAs(t, coach)
	path := fmt.Sprintf("/api/athletes/%d/bio-samples/import", athlete.ID)

	var b strings.Builder
	b.WriteString(`{"samples":[`)
	for i := 0; i < 5001; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"metric":"steps","value":%d,"recorded_at":"2026-01-01T00:00:%02dZ"}`, i, i%60)
	}
	b.WriteString(`]}`)
	requireStatus(t, env.do(t, "POST", path, b.String(), cookies), http.StatusRequestEntityTooLarge)
}
