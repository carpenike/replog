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
	t.Run("adult (no tier)", func(t *testing.T) {
		ctx := &AthleteContext{
			Athlete: AthleteProfile{Name: "Adult"},
		}
		prompt := buildSystemPrompt(ctx)
		if !strings.Contains(prompt, "CatalogJSON") {
			t.Error("system prompt should mention CatalogJSON")
		}
		if !strings.Contains(prompt, "prescribed_sets") {
			t.Error("system prompt should mention prescribed_sets")
		}
		if !strings.Contains(prompt, "ADULT ATHLETE PROGRAMMING RULES") {
			t.Error("adult prompt should contain adult rules")
		}
		if strings.Contains(prompt, "YOUTH ATHLETE SAFETY RULES") {
			t.Error("adult prompt should not contain youth rules")
		}
	})

	t.Run("foundational tier", func(t *testing.T) {
		tier := "foundational"
		ctx := &AthleteContext{
			Athlete: AthleteProfile{Name: "Youth", Tier: &tier},
		}
		prompt := buildSystemPrompt(ctx)
		if !strings.Contains(prompt, "YOUTH ATHLETE SAFETY RULES") {
			t.Error("youth prompt should contain youth safety rules")
		}
		if !strings.Contains(prompt, "FOUNDATIONAL TIER RULES") {
			t.Error("foundational prompt should contain foundational rules")
		}
		if strings.Contains(prompt, "ADULT ATHLETE PROGRAMMING RULES") {
			t.Error("youth prompt should not contain adult rules")
		}
		if !strings.Contains(prompt, "bodyweight exercises") {
			t.Error("foundational prompt should mention bodyweight exercises")
		}
		if !strings.Contains(prompt, "Do NOT use barbell") {
			t.Error("foundational prompt should prohibit barbells")
		}
		if !strings.Contains(prompt, "Yessis") {
			t.Error("foundational prompt should reference Dr. Yessis methodology")
		}
		if !strings.Contains(prompt, "1 SET of 20 REPS") {
			t.Error("foundational prompt should describe the 1×20 approach")
		}
		if !strings.Contains(prompt, "joint action") {
			t.Error("foundational prompt should mention joint action coverage")
		}
	})

	t.Run("intermediate tier", func(t *testing.T) {
		tier := "intermediate"
		ctx := &AthleteContext{
			Athlete: AthleteProfile{Name: "Youth", Tier: &tier},
		}
		prompt := buildSystemPrompt(ctx)
		if !strings.Contains(prompt, "INTERMEDIATE TIER RULES") {
			t.Error("intermediate prompt should contain intermediate rules")
		}
		if !strings.Contains(prompt, "light barbell work") {
			t.Error("intermediate prompt should mention barbell introduction")
		}
		if !strings.Contains(prompt, "14") {
			t.Error("intermediate prompt should reference the 1×14 rep target")
		}
	})

	t.Run("sport_performance tier", func(t *testing.T) {
		tier := "sport_performance"
		ctx := &AthleteContext{
			Athlete: AthleteProfile{Name: "Youth", Tier: &tier},
		}
		prompt := buildSystemPrompt(ctx)
		if !strings.Contains(prompt, "SPORT PERFORMANCE TIER RULES") {
			t.Error("sport_performance prompt should contain sport performance rules")
		}
		if !strings.Contains(prompt, "percentage-based loading") {
			t.Error("sport_performance prompt should mention percentage loading")
		}
	})
}

func TestBuildUserPrompt(t *testing.T) {
	tier := "sport_performance"
	athleteCtx := &AthleteContext{
		Athlete: AthleteProfile{Name: "TestAthlete", Tier: &tier},
		Performance: PerformanceData{
			TrainingMaxes: []TMEntry{{Exercise: "Squat", Weight: 200}},
		},
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
	if !strings.Contains(prompt, "sport_performance tier") {
		t.Error("prompt should mention the athlete's tier")
	}
	if !strings.Contains(prompt, "has training maxes") {
		t.Error("prompt should note training max availability")
	}
}

func TestBuildUserPrompt_NoTMs(t *testing.T) {
	athleteCtx := &AthleteContext{
		Athlete: AthleteProfile{Name: "Newbie"},
	}
	req := GenerationRequest{
		ProgramName: "Starter",
		NumWeeks:    4,
		NumDays:     3,
	}

	prompt, err := buildUserPrompt(athleteCtx, req)
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}
	if !strings.Contains(prompt, "NO training maxes") {
		t.Error("prompt should note absence of training maxes")
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
