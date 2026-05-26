package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/models"
)

// mockGeneratedCatalog is a minimal valid CatalogJSON the LLM mock returns.
// The handler parses this via importers.ParseCatalogJSON, so it must satisfy
// the wire format ("version" + "type": "catalog" required).
const mockGeneratedCatalog = `{
  "version": "1",
  "type": "catalog",
  "exercises": [
    {"name": "Generated Bench Press"}
  ],
  "equipment": [],
  "programs": [
    {
      "name": "Generated Program",
      "num_weeks": 3,
      "num_days": 4,
      "is_loop": false
    }
  ]
}`

// mockLLMResponse wraps a CatalogJSON in the reasoning + JSON envelope the
// LLM is prompted to emit and that extractResponse() expects.
const mockLLMResponse = `<reasoning>
This is a mock program. Bench focus.
</reasoning>

` + mockGeneratedCatalog

// mockLLMFactory returns a Handlers.LLMProviderFactory that always yields
// the given mock provider. Lets each test wire its own canned response or
// error path.
func mockLLMFactory(p llm.Provider) func(*sql.DB) (llm.Provider, error) {
	return func(_ *sql.DB) (llm.Provider, error) {
		return p, nil
	}
}

// useMockLLM seeds the llm.provider/llm.model settings so the handler's
// "configured" check passes, and replaces the LLM factory with a stub.
// Returns the mock so the test can mutate FixedContent / GenerateErr.
func useMockLLM(t *testing.T, env *testEnv) *llm.MockProvider {
	t.Helper()
	if err := models.SetSetting(env.DB, "llm.provider", "anthropic"); err != nil {
		t.Fatalf("set llm.provider: %v", err)
	}
	if err := models.SetSetting(env.DB, "llm.model", "claude-sonnet-mock"); err != nil {
		t.Fatalf("set llm.model: %v", err)
	}
	mock := &llm.MockProvider{FixedContent: mockLLMResponse}
	env.Handlers.LLMProviderFactory = mockLLMFactory(mock)
	return mock
}

// submitAndWait posts a generation request, asserts a 202 response, then
// blocks until the background goroutine completes via WaitForGenerations.
// Returns the generation_id from the 202 body.
func submitAndWait(t *testing.T, env *testEnv, athleteID int64, cookies []*http.Cookie, body string) int64 {
	t.Helper()
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athleteID), body, cookies)
	requireStatus(t, rr, http.StatusAccepted)

	var submit GenerateSubmitResponse
	decodeJSON(t, rr, &submit)
	if submit.GenerationID == 0 {
		t.Fatalf("expected generation_id, got %+v", submit)
	}
	if submit.Status != models.GenerationPending {
		t.Fatalf("expected status=pending, got %q", submit.Status)
	}

	env.Handlers.WaitForGenerations()
	return submit.GenerationID
}

// --- GenerateFormData ---

func TestGenerateFormData_ReturnsConfiguredFalseWhenNoProvider(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateFormResponse
	decodeJSON(t, rr, &got)
	if got.Configured {
		t.Error("expected configured=false with no llm.provider set")
	}
}

func TestGenerateFormData_ReturnsContextWhenConfigured(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateFormResponse
	decodeJSON(t, rr, &got)
	if !got.Configured {
		t.Error("expected configured=true with llm.provider set")
	}
	if got.DefaultDays < 1 || got.DefaultWeeks < 1 {
		t.Errorf("expected sensible defaults, got days=%d weeks=%d", got.DefaultDays, got.DefaultWeeks)
	}
}

func TestGenerateFormData_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "GET", "/api/athletes/1/generate", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestGenerateFormData_ExposesLatestGenerationForResume(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"X","num_days":4,"num_weeks":3}`)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateFormResponse
	decodeJSON(t, rr, &got)
	if got.LatestGeneration == nil {
		t.Fatal("expected latest_generation to be populated for resume")
	}
	if got.LatestGeneration.ID != genID {
		t.Errorf("expected latest_generation.id=%d, got %d", genID, got.LatestGeneration.ID)
	}
	if got.LatestGeneration.Status != models.GenerationSucceeded {
		t.Errorf("expected succeeded, got %q", got.LatestGeneration.Status)
	}
}

// --- GenerateSubmit (async) ---

func TestGenerateSubmit_AsyncEnqueuesAndReturns202(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := submitAndWait(t, env, athlete.ID, cookies, `{
		"program_name": "Mock Program",
		"num_days": 4,
		"num_weeks": 3,
		"is_loop": false,
		"focus_areas": ["strength"]
	}`)

	// Status polling reflects the completed state.
	rr := env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/generations/%d", athlete.ID, genID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerationResponse
	decodeJSON(t, rr, &got)
	if got.Status != models.GenerationSucceeded {
		t.Errorf("expected status=succeeded, got %q", got.Status)
	}
	if got.Programs != 1 {
		t.Errorf("expected 1 program from mock catalog, got %d", got.Programs)
	}
	if got.Exercises != 1 {
		t.Errorf("expected 1 exercise from mock catalog, got %d", got.Exercises)
	}
	if !strings.Contains(got.Reasoning, "mock program") {
		t.Errorf("expected reasoning from <reasoning> tags, got %q", got.Reasoning)
	}
	if got.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestGenerateSubmit_RequiresFields(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	cases := []struct {
		name string
		body string
	}{
		{"missing program_name", `{"num_days":4,"num_weeks":3}`},
		{"zero days", `{"program_name":"x","num_days":0,"num_weeks":3}`},
		{"zero weeks", `{"program_name":"x","num_days":4,"num_weeks":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), tc.body, cookies)
			requireStatus(t, rr, http.StatusBadRequest)
		})
	}
}

func TestGenerateSubmit_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/athletes/1/generate",
		`{"program_name":"x","num_days":4,"num_weeks":3}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestGenerateSubmit_PersistsFailureToGenerationRow(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	env.Handlers.LLMProviderFactory = mockLLMFactory(&llm.MockProvider{
		GenerateErr: &llm.APIError{
			Provider:   "Anthropic",
			StatusCode: 401,
			Message:    "invalid api key",
		},
	})
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	// Async submit still returns 202 — failures are recorded on the row,
	// not surfaced in the enqueue response.
	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"x","num_days":4,"num_weeks":3}`)

	rr := env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/generations/%d", athlete.ID, genID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerationResponse
	decodeJSON(t, rr, &got)
	if got.Status != models.GenerationFailed {
		t.Errorf("expected status=failed, got %q", got.Status)
	}
	if !strings.Contains(got.Error, "Invalid API key") {
		t.Errorf("expected user-friendly 401 message, got %q", got.Error)
	}
	if strings.Contains(got.Error, "HTTP 401") {
		t.Errorf("response should not leak raw HTTP status: %q", got.Error)
	}
}

func TestGenerateSubmit_ProviderNotConfigured(t *testing.T) {
	env := setupTest(t)
	// No useMockLLM — factory falls back to NewProviderFromSettings, which
	// returns ErrNotConfigured because no llm.provider setting is set.
	// Provider misconfig is a synchronous failure (we never even create
	// the row), so the response is 500.
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID),
		`{"program_name":"x","num_days":4,"num_weeks":3}`, cookies)
	requireStatus(t, rr, http.StatusInternalServerError)
	if !strings.Contains(rr.Body.String(), "not configured") {
		t.Errorf("expected 'not configured' error, got %q", rr.Body.String())
	}
}

// --- GenerationStatus ---

func TestGenerationStatus_404ForMissingRow(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/generations/999999", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

func TestGenerationStatus_404ForCrossAthlete(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	a1 := env.createAthlete(t, "Alice", coach.ID)
	a2 := env.createAthlete(t, "Bob", coach.ID)
	cookies := env.loginAs(t, coach)

	// Generation belongs to a1.
	genID := submitAndWait(t, env, a1.ID, cookies,
		`{"program_name":"X","num_days":4,"num_weeks":3}`)

	// Asking under a2's URL must not leak it.
	rr := env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/generations/%d", a2.ID, genID), nil, cookies)
	requireStatus(t, rr, http.StatusNotFound)
}

// --- GenerationCancel ---

func TestGenerationCancel_PendingRowMarkedCancelled(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	// Insert a pending row directly so it never picks up the goroutine.
	g, err := models.CreateGeneration(env.DB, athlete.ID, coach.ID,
		`{"program_name":"x","num_days":1,"num_weeks":1}`)
	if err != nil {
		t.Fatalf("seed pending generation: %v", err)
	}

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/generations/%d/cancel", athlete.ID, g.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerationResponse
	decodeJSON(t, rr, &got)
	if got.Status != models.GenerationCancelled {
		t.Errorf("expected cancelled, got %q", got.Status)
	}
}

// --- GenerationExecute ---

func TestGenerationExecute_AfterSubmit_PersistsProgram(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"Mock","num_days":4,"num_weeks":3}`)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/generations/%d/execute", athlete.ID, genID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateExecuteResponse
	decodeJSON(t, rr, &got)
	if got.ProgramsCreated != 1 {
		t.Errorf("expected 1 program created, got %d", got.ProgramsCreated)
	}
	if got.ExercisesCreated < 1 {
		t.Errorf("expected at least 1 exercise created, got %d", got.ExercisesCreated)
	}

	// Second execute is rejected — prevents double-import of the same draft.
	rr = env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/generations/%d/execute", athlete.ID, genID),
		nil, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestGenerationExecute_RejectsNonSucceeded(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	g, err := models.CreateGeneration(env.DB, athlete.ID, coach.ID,
		`{"program_name":"x","num_days":1,"num_weeks":1}`)
	if err != nil {
		t.Fatalf("seed pending generation: %v", err)
	}

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/generations/%d/execute", athlete.ID, g.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestGenerationExecute_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/athletes/1/generations/1/execute", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

// --- Server restart cleanup ---

func TestResetStaleRunningGenerations_MarksPriorRunsFailed(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)

	g, err := models.CreateGeneration(env.DB, athlete.ID, coach.ID,
		`{"program_name":"x","num_days":1,"num_weeks":1}`)
	if err != nil {
		t.Fatalf("seed generation: %v", err)
	}
	if err := models.MarkGenerationRunning(env.DB, g.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	n, err := models.ResetStaleRunningGenerations(env.DB)
	if err != nil {
		t.Fatalf("reset stale: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row reset, got %d", n)
	}

	got, err := models.GetGeneration(env.DB, g.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != models.GenerationFailed {
		t.Errorf("expected status=failed, got %q", got.Status)
	}
	if !got.Error.Valid || !strings.Contains(got.Error.String, "restart") {
		t.Errorf("expected restart message, got %v", got.Error)
	}
}
