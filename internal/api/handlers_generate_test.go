package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/database"
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

// TestGenerateFormData_MethodologyOptions_Youth confirms ADR 016 Phase 3:
// a youth athlete gets the youth-filtered methodology list AND a
// pre-selected default keyed by the athlete's tier (foundational →
// yessis-1x20).
func TestGenerateFormData_MethodologyOptions_Youth(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthleteWithTier(t, "Foundling", "foundational", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateFormResponse
	decodeJSON(t, rr, &got)

	if len(got.AvailableMethodologies) == 0 {
		t.Fatal("expected youth methodology options")
	}
	for _, m := range got.AvailableMethodologies {
		if m.Audience != "" && m.Audience != models.MethodologyAudienceYouth {
			t.Errorf("youth athlete should not see adult methodology %q (audience=%q)", m.Name, m.Audience)
		}
	}
	if got.DefaultMethodologyID == nil {
		t.Fatal("youth athlete should have a default methodology id")
	}
	def, err := models.GetMethodologyByID(env.DB, *got.DefaultMethodologyID)
	if err != nil {
		t.Fatalf("look up default: %v", err)
	}
	if def.Key != "yessis-1x20" {
		t.Errorf("foundational default = %q, want yessis-1x20", def.Key)
	}
}

// TestGenerateFormData_MethodologyOptions_Adult confirms the adult-optional
// selector contract (ADR 016 Phase 3 D1): adult athletes get the adult list
// AND a null DefaultMethodologyID so the SPA leaves the selector unselected
// and the coach may submit blank.
func TestGenerateFormData_MethodologyOptions_Adult(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Adult", coach.ID) // no tier
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateFormResponse
	decodeJSON(t, rr, &got)

	if len(got.AvailableMethodologies) == 0 {
		t.Fatal("expected adult methodology options")
	}
	for _, m := range got.AvailableMethodologies {
		if m.Audience != "" && m.Audience != models.MethodologyAudienceAdult {
			t.Errorf("adult athlete should not see youth methodology %q (audience=%q)", m.Name, m.Audience)
		}
	}
	if got.DefaultMethodologyID != nil {
		t.Errorf("adult should have NO default_methodology_id (selector is optional); got %d", *got.DefaultMethodologyID)
	}
}

// TestGenerateFormData_ReferencePoolNotFilteredByMethodology guards
// HOF-006 D2's explicit invariant: the reference_programs list is the
// FULL unfiltered pool (it's the coach's override surface, not the
// methodology's exemplar set). A future cleanup PR that pre-filters this
// by audience would silently narrow the override pool.
func TestGenerateFormData_ReferencePoolNotFilteredByMethodology(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	youth := env.createAthleteWithTier(t, "Y", "foundational", coach.ID)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", youth.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerateFormResponse
	decodeJSON(t, rr, &got)

	// Expect BOTH youth and adult program templates to appear — the
	// reference pool is the full ListProgramTemplates output.
	allTemplates, _ := models.ListProgramTemplates(env.DB)
	if len(got.ReferencePrograms) != len(allTemplates) {
		t.Errorf("reference pool was filtered (got %d, want %d) — D2 invariant violated", len(got.ReferencePrograms), len(allTemplates))
	}
}

// TestGenerateSubmit_AcceptsMethodologyID confirms the request DTO carries
// methodology_id through to the backend (Phase 2 wiring's input field).
func TestGenerateSubmit_AcceptsMethodologyID(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	seedMethodologiesForTest(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthleteWithTier(t, "Y", "foundational", coach.ID)
	cookies := env.loginAs(t, coach)

	// Pick the intermediate methodology — different from the tier default
	// — so we can confirm the explicit ID actually flows through.
	m, err := models.GetMethodologyByKey(env.DB, "yessis-1x15")
	if err != nil {
		t.Fatalf("get methodology: %v", err)
	}
	body := fmt.Sprintf(`{"program_name":"X","num_days":4,"num_weeks":3,"methodology_id":%d}`, m.ID)
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), body, cookies)
	requireStatus(t, rr, http.StatusAccepted)
	env.Handlers.WaitForGenerations()
}

// TestGenerateFormData_SuggestsProgramName confirms HOF-007: the form-data
// endpoint suggests "{AthleteName} — Block N" where N counts existing
// athlete-scoped program templates. Fresh athlete starts at Block 1; after
// the first generation imports as a template, the next suggestion is Block 2.
func TestGenerateFormData_SuggestsProgramName(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Sammy", coach.ID)
	cookies := env.loginAs(t, coach)

	// Fresh athlete → Block 1.
	rr := env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got GenerateFormResponse
	decodeJSON(t, rr, &got)
	if got.SuggestedProgramName != "Sammy — Block 1" {
		t.Errorf("first suggestion = %q, want %q", got.SuggestedProgramName, "Sammy — Block 1")
	}

	// Generate + execute → adds one athlete-scoped template.
	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"Sammy — Block 1","num_days":3,"num_weeks":4}`)
	executeRR := env.do(t, "POST",
		fmt.Sprintf("/api/athletes/%d/generations/%d/execute", athlete.ID, genID),
		`{}`, cookies)
	requireStatus(t, executeRR, http.StatusOK)

	// Next suggestion bumps to Block 2.
	rr = env.do(t, "GET", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &got)
	if got.SuggestedProgramName != "Sammy — Block 2" {
		t.Errorf("after-import suggestion = %q, want %q", got.SuggestedProgramName, "Sammy — Block 2")
	}
}

// TestGenerateSubmit_NormalizesNumWeeksWhenLooping is the regression guard
// for HOF-007 D3's defense-in-depth amendment. A non-SPA client (notably
// the HOF-004 MCP enqueue tool) might submit `is_loop: true` with
// `num_weeks > 1`; the LLM prompt would silently ignore the value, so the
// handler normalizes it to 1 (with a log line) before persisting the
// request_json on the generation row.
func TestGenerateSubmit_NormalizesNumWeeksWhenLooping(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Adult", coach.ID)
	cookies := env.loginAs(t, coach)

	// Submit the broken combo: is_loop=true AND num_weeks=12. Handler
	// must accept (202) — silent normalize, not a 400.
	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"X","num_days":3,"num_weeks":12,"is_loop":true}`)

	// Read the persisted request_json off the generation row and confirm
	// the normalize landed BEFORE the marshal.
	gen, err := models.GetGeneration(env.DB, genID)
	if err != nil {
		t.Fatalf("get generation: %v", err)
	}
	var persisted struct {
		NumWeeks int  `json:"num_weeks"`
		IsLoop   bool `json:"is_loop"`
	}
	if err := json.Unmarshal([]byte(gen.RequestJSON), &persisted); err != nil {
		t.Fatalf("decode persisted request_json: %v", err)
	}
	if !persisted.IsLoop {
		t.Errorf("persisted IsLoop = false, want true (sanity)")
	}
	if persisted.NumWeeks != 1 {
		t.Errorf("persisted NumWeeks = %d, want 1 (normalize-on-write failed)", persisted.NumWeeks)
	}
}

// seedMethodologiesForTest applies the embedded catalog + methodology
// seeds to the test DB. Required by any handler test that exercises the
// ADR 016 Phase 2/3 path.
func seedMethodologiesForTest(t *testing.T, env *testEnv) {
	t.Helper()
	// Apply catalog so the methodology link references resolve.
	if _, err := env.DB.Exec("SELECT 1 FROM exercises LIMIT 1"); err != nil {
		t.Fatalf("test DB missing exercises table: %v", err)
	}
	// If the catalog hasn't been seeded yet, do it now.
	var exCount int
	env.DB.QueryRow("SELECT COUNT(*) FROM exercises").Scan(&exCount)
	if exCount == 0 {
		// Use models.ExecuteCatalogImport via importers.ParseCatalogJSON.
		// Reuse the same path bootstrapCatalog does.
		// We import inline to avoid pulling in the importers package
		// in every test file.
		seedCatalogInline(t, env)
	}
	if _, err := models.ApplyMethodologySeedFromBytes(env.DB, database.SeedMethodologies()); err != nil {
		t.Fatalf("seed methodologies: %v", err)
	}
}

// seedCatalogInline applies the embedded seed catalog to the test DB
// using the same pipeline cmd/replog/main.go bootstrapCatalog uses.
func seedCatalogInline(t *testing.T, env *testEnv) {
	t.Helper()
	t.Cleanup(func() {})
	if err := applyCatalogSeed(env.DB); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
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

// --- HOF-001: duplicate-submit guard ---

func TestGenerateSubmit_DuplicateRejectedWith409(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	// Seed a pending generation for this athlete WITHOUT waiting on it,
	// to simulate "a draft is in flight". We bypass the handler so the
	// goroutine doesn't race the second submit.
	if _, err := models.CreateGeneration(env.DB, athlete.ID, coach.ID,
		`{"program_name":"first","num_days":3,"num_weeks":2}`); err != nil {
		t.Fatalf("seed pending generation: %v", err)
	}

	// Second submit must fail fast — same athlete, draft in flight.
	rr := env.do(t, "POST", fmt.Sprintf("/api/athletes/%d/generate", athlete.ID),
		`{"program_name":"second","num_days":4,"num_weeks":3}`, cookies)
	requireStatus(t, rr, http.StatusConflict)
}

// --- HOF-001: no auto-assign ---

func TestGenerationExecute_DoesNotAssignProgramToAthlete(t *testing.T) {
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

	// The athlete must have NO active program after approving the draft —
	// ADR 007 / HOF-001: approve creates an unassigned template; explicit
	// assignment via POST /athletes/{id}/programs is a separate coach step.
	progs, err := models.ListAthletePrograms(env.DB, athlete.ID)
	if err != nil {
		t.Fatalf("ListAthletePrograms: %v", err)
	}
	for _, p := range progs {
		if p.Active {
			t.Errorf("expected no active athlete_programs row after execute, got %+v", p)
		}
	}

	// The template SHOULD have been created (athlete-scoped via athlete_id).
	templates, err := models.ListProgramTemplates(env.DB)
	if err != nil {
		t.Fatalf("ListProgramTemplates: %v", err)
	}
	found := false
	for _, tpl := range templates {
		if tpl.Name == "Generated Program" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected template 'Generated Program' to be created (athlete-scoped, unassigned)")
	}
}

// --- HOF-001: notify on failure ---

func TestRunGeneration_NotifiesOnProviderFailure(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	env.Handlers.LLMProviderFactory = mockLLMFactory(&llm.MockProvider{
		GenerateErr: &llm.APIError{Provider: "Anthropic", StatusCode: 401, Message: "invalid api key"},
	})
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"x","num_days":4,"num_weeks":3}`)

	notifs, err := models.ListNotifications(env.DB, coach.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListNotifications: %v", err)
	}
	found := false
	for _, n := range notifs {
		if n.Type == models.NotifyGenerationFailed {
			found = true
			if !strings.Contains(n.Title, "Charlie") {
				t.Errorf("expected athlete name in title, got %q", n.Title)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected NotifyGenerationFailed notification, got %+v", notifs)
	}
}

// --- HOF-001: truncation hint ---

func TestRunGeneration_TruncationHintInError(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	// Return invalid JSON with stop_reason=max_tokens to trigger the
	// truncation-specific error message.
	env.Handlers.LLMProviderFactory = mockLLMFactory(&llm.MockProvider{
		FixedContent:    "<reasoning>truncated</reasoning>\n{\"version\":\"1\",\"type\":\"catalog\",\"programs\":[",
		FixedStopReason: "max_tokens",
	})
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"x","num_days":4,"num_weeks":3}`)

	g, err := models.GetGeneration(env.DB, genID)
	if err != nil {
		t.Fatalf("reload generation: %v", err)
	}
	if g.Status != models.GenerationFailed {
		t.Fatalf("expected failed, got %q", g.Status)
	}
	if !g.Error.Valid || !strings.Contains(g.Error.String, "truncated") {
		t.Errorf("expected truncation hint, got %q", g.Error.String)
	}
	if !strings.Contains(g.Error.String, "max_tokens") {
		t.Errorf("expected actionable advice mentioning max_tokens, got %q", g.Error.String)
	}
}

// --- HOF-001: set-level preview ---

func TestGenerationStatus_IncludesSetLevelPreviewOnSuccess(t *testing.T) {
	env := setupTest(t)
	// Use a richer mock catalog with prescribed sets so we can verify
	// the per-day projection.
	const richCatalog = `{
  "version": "1",
  "type": "catalog",
  "exercises": [{"name": "Bench Press"}],
  "equipment": [],
  "programs": [
    {
      "name": "Preview Program",
      "num_weeks": 2,
      "num_days": 1,
      "is_loop": false,
      "prescribed_sets": [
        {"exercise": "Bench Press", "week": 1, "day": 1, "set_number": 1, "reps": 5, "percentage": 0.70, "sort_order": 1},
        {"exercise": "Bench Press", "week": 1, "day": 1, "set_number": 2, "reps": 5, "percentage": 0.80, "sort_order": 1},
        {"exercise": "Bench Press", "week": 2, "day": 1, "set_number": 1, "reps": 3, "percentage": 0.85, "sort_order": 1}
      ],
      "progression_rules": [{"exercise": "Bench Press", "increment": 5.0}]
    }
  ]
}`
	useMockLLM(t, env)
	env.Handlers.LLMProviderFactory = mockLLMFactory(&llm.MockProvider{
		FixedContent: "<reasoning>preview test</reasoning>\n" + richCatalog,
	})
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"x","num_days":1,"num_weeks":2}`)

	rr := env.do(t, "GET",
		fmt.Sprintf("/api/athletes/%d/generations/%d", athlete.ID, genID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got GenerationResponse
	decodeJSON(t, rr, &got)
	if got.Status != models.GenerationSucceeded {
		t.Fatalf("expected succeeded, got %q (err: %q)", got.Status, got.Error)
	}
	if got.Preview == nil {
		t.Fatalf("expected preview to be populated on success")
	}
	if len(got.Preview.Programs) != 1 {
		t.Fatalf("expected 1 program in preview, got %d", len(got.Preview.Programs))
	}
	pp := got.Preview.Programs[0]
	if pp.Name != "Preview Program" {
		t.Errorf("expected program name 'Preview Program', got %q", pp.Name)
	}
	if len(pp.Weeks) != 2 {
		t.Fatalf("expected 2 weeks, got %d", len(pp.Weeks))
	}
	if len(pp.Weeks[0].Days) != 1 || len(pp.Weeks[0].Days[0].Sets) != 2 {
		t.Errorf("expected week 1 day 1 to have 2 sets, got %+v", pp.Weeks[0].Days)
	}
	if len(got.Preview.ProgressionRules) != 1 {
		t.Errorf("expected 1 progression rule, got %d", len(got.Preview.ProgressionRules))
	}
}

// --- HOF-001: prompt + context persisted ---

func TestGenerationExecute_PersistsContextAndPrompt(t *testing.T) {
	env := setupTest(t)
	useMockLLM(t, env)
	coach := env.createUser(t, "coach", true, false)
	athlete := env.createAthlete(t, "Charlie", coach.ID)
	cookies := env.loginAs(t, coach)

	genID := submitAndWait(t, env, athlete.ID, cookies,
		`{"program_name":"Mock","num_days":4,"num_weeks":3}`)

	g, err := models.GetGeneration(env.DB, genID)
	if err != nil {
		t.Fatalf("reload generation: %v", err)
	}
	if g.Status != models.GenerationSucceeded {
		t.Fatalf("expected succeeded, got %q", g.Status)
	}
	if !g.ContextJSON.Valid || g.ContextJSON.String == "" {
		t.Errorf("expected context_json to be persisted, got %v", g.ContextJSON)
	}
	if !g.Prompt.Valid || !strings.Contains(g.Prompt.String, "USER PROMPT") {
		t.Errorf("expected prompt to be persisted with system+user delimiter, got %q", g.Prompt.String)
	}
}
