package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerate_MockProvider(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "TestGen", "foundational", "get strong")

	mockJSON := `{"version": "1.0", "type": "catalog", "exercises": [], "programs": [{"name": "Test Program", "description": "A test", "num_weeks": 4, "num_days": 3, "is_loop": false, "prescribed_sets": [], "progression_rules": []}]}`
	provider := &MockProvider{
		FixedContent: "<reasoning>Chose exercises based on goals.</reasoning>\n```json\n" + mockJSON + "\n```",
	}

	req := GenerationRequest{
		AthleteID:   athleteID,
		ProgramName: "Test Program",
		NumWeeks:    4,
		NumDays:     3,
		IsLoop:      false,
	}

	result, err := Generate(context.Background(), db, provider, req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Reasoning != "Chose exercises based on goals." {
		t.Errorf("reasoning = %q", result.Reasoning)
	}
	if !json.Valid(result.CatalogJSON) {
		t.Errorf("CatalogJSON is not valid JSON: %s", result.CatalogJSON)
	}
	if result.RawResponse == "" {
		t.Error("RawResponse should not be empty")
	}
}

func TestGenerate_ProviderError(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "TestErr", "", "")

	provider := &MockProvider{
		GenerateErr: context.DeadlineExceeded,
	}

	req := GenerationRequest{
		AthleteID:   athleteID,
		ProgramName: "Test",
		NumWeeks:    1,
		NumDays:     3,
		IsLoop:      true,
	}

	_, err := Generate(context.Background(), db, provider, req)
	if err == nil {
		t.Fatal("expected error from provider")
	}
	if !strings.Contains(err.Error(), "provider generate") {
		t.Errorf("error = %q, want to contain 'provider generate'", err.Error())
	}
}

func TestExtractResponse_WithReasoning(t *testing.T) {
	content := "<reasoning>I chose compound lifts.</reasoning>\n```json\n{\"version\": \"1.0\", \"programs\": []}\n```"
	catalogJSON, reasoning := extractResponse(content)
	if reasoning != "I chose compound lifts." {
		t.Errorf("reasoning = %q", reasoning)
	}
	if !json.Valid(catalogJSON) {
		t.Errorf("catalogJSON is not valid: %s", catalogJSON)
	}
}

func TestExtractResponse_NoReasoning(t *testing.T) {
	content := `{"version": "1.0", "programs": []}`
	catalogJSON, reasoning := extractResponse(content)
	if reasoning != "" {
		t.Errorf("reasoning = %q, want empty", reasoning)
	}
	if !json.Valid(catalogJSON) {
		t.Errorf("catalogJSON is not valid: %s", catalogJSON)
	}
}

func TestExtractJSON_CodeFence(t *testing.T) {
	input := "Here is the program:\n```json\n{\"version\": \"1.0\"}\n```\nDone."
	result := extractJSON(input)
	if string(result) != `{"version": "1.0"}` {
		t.Errorf("extractJSON = %q", string(result))
	}
}

func TestExtractJSON_BareJSON(t *testing.T) {
	input := "Some text before {\"key\": \"value\"} and after."
	result := extractJSON(input)
	if string(result) != `{"key": "value"}` {
		t.Errorf("extractJSON = %q", string(result))
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	result := extractJSON("This has no JSON at all.")
	if result != nil {
		t.Errorf("expected nil, got %q", string(result))
	}
}

func TestExtractJSON_InvalidJSON(t *testing.T) {
	input := "{not valid json}"
	result := extractJSON(input)
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %q", string(result))
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	prompt := buildSystemPrompt()
	if !strings.Contains(prompt, "CatalogJSON") {
		t.Error("system prompt should mention CatalogJSON")
	}
	if !strings.Contains(prompt, "prescribed_sets") {
		t.Error("system prompt should mention prescribed_sets")
	}
}

func TestBuildUserPrompt(t *testing.T) {
	athleteCtx := &AthleteContext{
		Athlete: AthleteProfile{Name: "TestAthlete"},
	}
	req := GenerationRequest{
		ProgramName:     "Month 4",
		NumWeeks:        4,
		NumDays:         3,
		IsLoop:          false,
		FocusAreas:      []string{"power", "conditioning"},
		CoachDirections: "Start introducing hang cleans",
	}

	prompt, err := buildUserPrompt(athleteCtx, req)
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}
	if !strings.Contains(prompt, "TestAthlete") {
		t.Error("prompt should contain athlete name")
	}
	if !strings.Contains(prompt, "Month 4") {
		t.Error("prompt should contain program name")
	}
	if !strings.Contains(prompt, "hang cleans") {
		t.Error("prompt should contain coach directions")
	}
	if !strings.Contains(prompt, "power") {
		t.Error("prompt should contain focus areas")
	}
}

func TestBuildUserPrompt_Loop(t *testing.T) {
	athleteCtx := &AthleteContext{
		Athlete: AthleteProfile{Name: "Looper"},
	}
	req := GenerationRequest{
		ProgramName: "Weekly Loop",
		NumWeeks:    1,
		NumDays:     4,
		IsLoop:      true,
	}

	prompt, err := buildUserPrompt(athleteCtx, req)
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}
	if !strings.Contains(prompt, "looping") {
		t.Error("prompt should mention looping for IsLoop=true")
	}
}
