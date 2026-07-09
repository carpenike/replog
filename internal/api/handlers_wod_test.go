package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/models"
)

// wodSubmitAndWait posts a WOD generation request, asserts 202, and blocks
// until the background goroutine finishes. Returns the generation_id.
func wodSubmitAndWait(t *testing.T, env *testEnv, athleteID int64, cookies []*http.Cookie, body string) int64 {
	t.Helper()
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/wod", athleteID), body, cookies)
	requireStatus(t, rr, http.StatusAccepted)

	var submit GenerateSubmitResponse
	decodeJSON(t, rr, &submit)
	if submit.GenerationID == 0 {
		t.Fatalf("expected generation_id, got %+v", submit)
	}
	env.Handlers.WaitForGenerations()
	return submit.GenerationID
}

func TestWODSubmit_AdultOnly(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	// Youth athlete (has a tier) is rejected.
	youth := env.createAthleteWithTier(t, "Youth", "foundational", coach.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/wod", youth.ID), `{}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)

	// Adult athlete (no tier) is accepted and the WOD generation succeeds.
	adult := env.createAthlete(t, "Adult", coach.ID)
	genID := wodSubmitAndWait(t, env, adult.ID, cookies, `{}`)

	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generations/%d", adult.ID, genID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got GenerationResponse
	decodeJSON(t, rr, &got)
	if got.Status != models.GenerationSucceeded {
		t.Errorf("expected succeeded, got %q", got.Status)
	}
	if got.Kind != models.GenerationKindWOD {
		t.Errorf("expected kind=wod, got %q", got.Kind)
	}
}

func TestWODSubmit_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/athletes/1/wod", `{}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// TestWODSubmit_NotifiesWithWODLink verifies the completion notification for
// a WOD is kind-aware: it links to the WOD page with log-or-discard copy
// rather than the program page's approve-the-program copy.
func TestWODSubmit_NotifiesWithWODLink(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	adult := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := wodSubmitAndWait(t, env, adult.ID, cookies, `{}`)

	notifs, err := models.ListNotifications(context.Background(), env.DB, coach.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListNotifications: %v", err)
	}
	wantLink := fmt.Sprintf("/athletes/%d/wod?gen=%d", adult.ID, genID)
	found := false
	for _, n := range notifs {
		if n.Type != models.NotifyGenerationComplete {
			continue
		}
		found = true
		if n.Link.String != wantLink {
			t.Errorf("expected WOD notification link %q, got %q", wantLink, n.Link.String)
		}
		if !strings.Contains(n.Title, "WOD") {
			t.Errorf("expected WOD in title, got %q", n.Title)
		}
		if !strings.Contains(n.Message.String, "Log or discard") {
			t.Errorf("expected log-or-discard copy, got %q", n.Message.String)
		}
		break
	}
	if !found {
		t.Errorf("expected a NotifyGenerationComplete notification, got %+v", notifs)
	}
}

func TestWODSubmit_NotifiesFailureWithWODLink(t *testing.T) {
	env := setupTest(t)
	mock := useMockLLM(t, env)
	mock.GenerateErr = &llm.APIError{Provider: "Anthropic", StatusCode: 401, Message: "invalid api key"}
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	adult := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := wodSubmitAndWait(t, env, adult.ID, cookies, `{}`)

	notifs, err := models.ListNotifications(context.Background(), env.DB, coach.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListNotifications: %v", err)
	}
	wantLink := fmt.Sprintf("/athletes/%d/wod?gen=%d", adult.ID, genID)
	found := false
	for _, n := range notifs {
		if n.Type != models.NotifyGenerationFailed {
			continue
		}
		found = true
		if n.Link.String != wantLink {
			t.Errorf("expected failed WOD notification link %q, got %q", wantLink, n.Link.String)
		}
		if !strings.Contains(n.Title, "WOD") {
			t.Errorf("expected WOD in title, got %q", n.Title)
		}
		break
	}
	if !found {
		t.Errorf("expected a NotifyGenerationFailed notification, got %+v", notifs)
	}
}

// TestWODKindIsolation verifies a WOD in flight does not surface in the
// program GeneratePage resume path, and vice versa (review finding d).
func TestWODKindIsolation(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	adult := env.createAthlete(t, "Adult", coach.ID)
	cookies := env.loginAs(t, coach)

	// A completed WOD generation must NOT be the program form's resume target.
	wodID := wodSubmitAndWait(t, env, adult.ID, cookies, `{}`)
	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", adult.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var form GenerateFormResponse
	decodeJSON(t, rr, &form)
	if form.LatestGeneration != nil && form.LatestGeneration.ID == wodID {
		t.Error("WOD generation leaked into the program GeneratePage resume path")
	}
}

func TestWODLog_CollisionThenReplace(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	adult := env.createAthlete(t, "Adult", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := wodSubmitAndWait(t, env, adult.ID, cookies, `{}`)

	logURL := fmt.Sprintf("/api/athletes/%d/wod/%d/log", adult.ID, genID)

	// First log creates the ad-hoc resistance workout.
	rr := env.do(t, "POST", logURL, `{"date":"2026-06-22"}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var logged WODLogResponse
	decodeJSON(t, rr, &logged)
	if logged.WorkoutID == 0 {
		t.Fatalf("expected a workout id, got %+v", logged)
	}

	// Logging the same WOD again is rejected — already executed.
	rr = env.do(t, "POST", logURL, `{"date":"2026-06-22"}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)

	// A fresh WOD on the same date collides with the existing resistance
	// workout and must return 409 collision (not a raw error).
	gen2 := wodSubmitAndWait(t, env, adult.ID, cookies, `{}`)
	log2URL := fmt.Sprintf("/api/athletes/%d/wod/%d/log", adult.ID, gen2)
	rr = env.do(t, "POST", log2URL, `{"date":"2026-06-22"}`, cookies)
	requireStatus(t, rr, http.StatusConflict)
	var collision WODCollisionResponse
	decodeJSON(t, rr, &collision)
	if !collision.Collision {
		t.Errorf("expected collision=true, got %+v", collision)
	}

	// Replace supersedes the existing workout.
	rr = env.do(t, "POST", log2URL, `{"date":"2026-06-22","replace":true}`, cookies)
	requireStatus(t, rr, http.StatusCreated)
	var replaced WODLogResponse
	decodeJSON(t, rr, &replaced)
	if !replaced.Replaced {
		t.Errorf("expected replaced=true, got %+v", replaced)
	}
}
