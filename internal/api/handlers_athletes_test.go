package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// --- ListAthletes ---

func TestListAthletes_AdminSeesAll(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	env.createAthlete(t, "Alice", coachA.ID)
	env.createAthlete(t, "Bob", coachB.ID)
	env.createAthlete(t, "Charlie", admin.ID)

	cookies := env.loginAs(t, admin)
	rr := env.do(t, "GET", "/api/athletes", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []*AthleteCard
	decodeJSON(t, rr, &got)
	if len(got) != 3 {
		t.Errorf("admin should see all 3 athletes, got %d", len(got))
	}
}

func TestListAthletes_CoachOnlySeesOwn(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	env.createAthlete(t, "Alice", coachA.ID)
	env.createAthlete(t, "Bob", coachB.ID)

	cookies := env.loginAs(t, coachA)
	rr := env.do(t, "GET", "/api/athletes", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []*AthleteCard
	decodeJSON(t, rr, &got)
	if len(got) != 1 {
		t.Fatalf("coachA should see 1 athlete, got %d (%+v)", len(got), got)
	}
	if got[0].Name != "Alice" {
		t.Errorf("expected Alice, got %q", got[0].Name)
	}
}

func TestListAthletes_Unauthenticated(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "GET", "/api/athletes", nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

// --- GetAthlete ---

func TestGetAthlete_OwnCoach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got Athlete
	decodeJSON(t, rr, &got)
	if got.ID != athlete.ID || got.Name != "Charlie" {
		t.Errorf("got %+v, want id=%d name=Charlie", got, athlete.ID)
	}
}

func TestGetAthlete_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d", athleteOfA.ID), nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestGetAthlete_AdminCanAccessAny(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Alice", coach.ID)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestGetAthlete_NotFound(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", "/api/athletes/9999", nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

func TestGetAthlete_BadID(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", "/api/athletes/notanumber", nil, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

// --- CreateAthlete ---

func TestCreateAthlete_Coach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", "/api/athletes",
		`{"name":"Charlie","tier":"foundational","track_body_weight":true}`, cookies)
	requireStatus(t, rr, http.StatusCreated)

	var got Athlete
	decodeJSON(t, rr, &got)
	if got.Name != "Charlie" {
		t.Errorf("got name %q, want Charlie", got.Name)
	}
	if got.Tier == nil || *got.Tier != "foundational" {
		t.Errorf("got tier %v, want foundational", got.Tier)
	}
	if !got.TrackBodyWeight {
		t.Error("expected track_body_weight=true")
	}

	// New athlete should be owned by the creating coach (visible to them, not other coaches).
	otherCoach := env.createUser(t, "other", true, false)
	otherCookies := env.loginAs(t, otherCoach)
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d", got.ID), nil, otherCookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestCreateAthlete_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athleteuser", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/athletes", `{"name":"Charlie"}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestCreateAthlete_RequiresName(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", "/api/athletes", `{"name":""}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

// --- UpdateAthlete ---

func TestUpdateAthlete_OwnCoach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/athletes/%d", athlete.ID),
		`{"name":"Charles","tier":"intermediate"}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got Athlete
	decodeJSON(t, rr, &got)
	if got.Name != "Charles" {
		t.Errorf("got name %q, want Charles", got.Name)
	}
	if got.Tier == nil || *got.Tier != "intermediate" {
		t.Errorf("got tier %v, want intermediate", got.Tier)
	}
}

func TestUpdateAthlete_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/athletes/%d", athleteOfA.ID),
		`{"name":"Hijacked"}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestUpdateAthlete_AdminCanUpdateAny(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Alice", coach.ID)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/athletes/%d", athlete.ID),
		`{"name":"Renamed","tier":"sport_performance"}`, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestUpdateAthlete_NotFound(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "PUT", "/api/athletes/9999", `{"name":"Ghost"}`, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

// --- DeleteAthlete ---

func TestDeleteAthlete_OwnCoach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "DELETE", fmt.Sprintf("/api/athletes/%d", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var status StatusResponse
	decodeJSON(t, rr, &status)
	if status.Status != "ok" {
		t.Errorf("got status %q, want ok", status.Status)
	}

	// The athlete is gone. An admin sees 404; the now-orphaned coach gets 403
	// because CanAccessAthlete can't find the ownership record either way.
	admin := env.createUser(t, "admin", true, true)
	adminCookies := env.loginAs(t, admin)
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d", athlete.ID), nil, adminCookies)
	requireStatus(t, rr, http.StatusNotFound)
}

func TestDeleteAthlete_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "DELETE", fmt.Sprintf("/api/athletes/%d", athleteOfA.ID), nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- UpdateAthleteGoal ---

func TestUpdateAthleteGoal_LinkedAthleteUser(t *testing.T) {
	// An athlete user (non-coach) can update their own linked athlete's goal.
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)

	// Create a non-coach user linked to this athlete via athlete_id.
	user, err := models.CreateUser(env.DB, "charlieuser", "Charlie", "password123",
		"charlie@example.com", false, false,
		sql.NullInt64{Int64: athlete.ID, Valid: true})
	if err != nil {
		t.Fatalf("create linked user: %v", err)
	}
	if err := models.EnsureUserPreferences(env.DB, user.ID); err != nil {
		t.Fatalf("ensure prefs: %v", err)
	}
	cookies := env.loginAs(t, user)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/athletes/%d/goal", athlete.ID),
		`{"goal":"Squat 200 by summer"}`, cookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestUpdateAthleteGoal_OtherUserForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "PUT", fmt.Sprintf("/api/athletes/%d/goal", athleteOfA.ID),
		`{"goal":"Hijack"}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- PromoteAthlete ---

func TestPromoteAthlete_OwnCoach(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete, err := models.CreateAthlete(env.DB, "Charlie", "foundational",
		"", "", "", "", "",
		sql.NullInt64{Int64: coach.ID, Valid: true}, false)
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/promote", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got Athlete
	decodeJSON(t, rr, &got)
	if got.Tier == nil || *got.Tier == "foundational" {
		t.Errorf("expected tier to advance from foundational, got %v", got.Tier)
	}
}

func TestPromoteAthlete_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/promote", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestPromoteAthlete_OtherCoachForbidden(t *testing.T) {
	env := setupTest(t)
	coachA := env.createUser(t, "coachA", true, false)
	coachB := env.createUser(t, "coachB", true, false)
	athleteOfA := env.createAthlete(t, "Alice", coachA.ID)
	cookies := env.loginAs(t, coachB)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/promote", athleteOfA.ID), nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}
