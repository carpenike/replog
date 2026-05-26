package api

import (
	"fmt"
	"net/http"
	"testing"
)

// TestGetCycleReview_NoCompletedCycle is a regression test for the panic
// observed when GetCycleReview is called on an athlete with an active
// program but no completed cycle yet (e.g. just-assigned program or
// mid-cycle 1). models.GetCycleSummary legitimately returns (nil, nil)
// in that case; the handler must not deref nil.
func TestGetCycleReview_NoCompletedCycle(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	tpl := env.createProgramTemplate(t, "1×20", 1, 2)
	cookies := env.loginAs(t, coach)

	// Assign program — start of cycle 1, no workouts logged.
	body := fmt.Sprintf(`{"template_id":%d,"start_date":"2026-05-26"}`, tpl.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/programs", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusCreated)

	// GET cycle-review must respond 200 with an empty Suggestions slice,
	// not 500 / panic.
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/cycle-review", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got CycleSummaryResponse
	decodeJSON(t, rr, &got)
	if got.Suggestions == nil {
		t.Errorf("expected non-nil (empty) Suggestions slice, got nil — JSON would be null instead of [] and break the SPA")
	}
	if len(got.Suggestions) != 0 {
		t.Errorf("expected 0 suggestions, got %d", len(got.Suggestions))
	}
}

// TestGetCycleReview_NoProgram covers the no-program path. models.GetActiveProgram
// returns (nil, nil) (not ErrNotFound) when an athlete has no active program,
// and the handler treats both "no program" and "no completed cycle" the same way:
// 200 with an empty Suggestions slice so the UI can render an empty state.
func TestGetCycleReview_NoProgram(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/cycle-review", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got CycleSummaryResponse
	decodeJSON(t, rr, &got)
	if got.Suggestions == nil {
		t.Errorf("expected non-nil (empty) Suggestions slice, got nil")
	}
	if len(got.Suggestions) != 0 {
		t.Errorf("expected 0 suggestions, got %d", len(got.Suggestions))
	}
}
