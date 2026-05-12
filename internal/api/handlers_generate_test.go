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

// --- GenerateSubmit ---

func TestGenerateSubmit_HappyPath(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	body := `{
		"program_name": "Mock Program",
		"num_days": 4,
		"num_weeks": 3,
		"is_loop": false,
		"focus_areas": ["strength"]
	}`
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateSubmitResponse
	decodeJSON(t, rr, &got)
	if got.Programs != 1 {
		t.Errorf("expected 1 program from mock catalog, got %d", got.Programs)
	}
	if got.Exercises != 1 {
		t.Errorf("expected 1 exercise from mock catalog, got %d", got.Exercises)
	}
	if !strings.Contains(got.Reasoning, "mock program") {
		t.Errorf("expected reasoning to be extracted from <reasoning> tags, got %q", got.Reasoning)
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

func TestGenerateSubmit_MapsLLMAPIErrorToFriendlyMessage(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	// Replace the mock's response with an llm.APIError to verify the
	// handler unwraps it via errors.As and returns the user-friendly text.
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

	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID),
		`{"program_name":"x","num_days":4,"num_weeks":3}`, cookies)
	requireStatus(t, rr, http.StatusInternalServerError)

	var got APIError
	decodeJSON(t, rr, &got)
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

// --- GenerateExecute ---

func TestGenerateExecute_RequiresSubmitFirst(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/generate/execute", athlete.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestGenerateExecute_AfterSubmit_PersistsProgram(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	// Submit (mock returns the canned catalog).
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID),
		`{"program_name":"Mock","num_days":4,"num_weeks":3}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Execute commits.
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate/execute", athlete.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got map[string]any
	decodeJSON(t, rr, &got)
	if got["programs_created"].(float64) != 1 {
		t.Errorf("expected 1 program created, got %v", got["programs_created"])
	}
	if got["exercises_created"].(float64) < 1 {
		t.Errorf("expected at least 1 exercise created, got %v", got["exercises_created"])
	}

	// Subsequent execute should fail (cache cleared).
	rr = env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate/execute", athlete.ID),
		nil, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestGenerateExecute_NonCoachForbidden(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "athlete_user", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/athletes/1/generate/execute", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}
