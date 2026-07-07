package llm

import (
	"strings"
	"testing"
)

func youthCtx() *AthleteContext {
	tier := "foundational"
	return &AthleteContext{
		Athlete: AthleteProfile{Name: "Kid", Tier: &tier, WeightUnit: "lbs"},
		ExerciseCatalog: []ExerciseEntry{
			{Name: "Goblet Squat", Compatible: true},
			{Name: "Push-up", Compatible: true},
			{Name: "Barbell Bench Press", Compatible: false},
		},
	}
}

func TestLintCatalog_FlagsUnknownExercise(t *testing.T) {
	// "Goblet Squats" (plural) does not resolve against "Goblet Squat".
	catalog := `{"version":"1.0","type":"catalog","exercises":[],"programs":[{"name":"P","num_weeks":1,"num_days":1,"prescribed_sets":[{"exercise":"Goblet Squats","week":1,"day":1,"set_number":1,"reps":10,"rep_type":"reps"}]}]}`
	res := LintCatalog([]byte(catalog), youthCtx())
	if len(res.Warnings) == 0 {
		t.Fatal("expected a warning for the unknown exercise, got none")
	}
	if !strings.Contains(strings.Join(res.Warnings, " "), "Goblet Squats") {
		t.Fatalf("warning should name the offending exercise, got %v", res.Warnings)
	}
}

func TestLintCatalog_CaseInsensitiveMatchIsClean(t *testing.T) {
	catalog := `{"version":"1.0","type":"catalog","exercises":[],"programs":[{"name":"P","num_weeks":1,"num_days":1,"prescribed_sets":[{"exercise":"goblet   squat","week":1,"day":1,"set_number":1,"reps":10,"rep_type":"reps"}]}]}`
	res := LintCatalog([]byte(catalog), youthCtx())
	if len(res.Warnings) != 0 {
		t.Fatalf("case/spacing variation should match cleanly, got %v", res.Warnings)
	}
}

func TestLintCatalog_FlagsIncompatibleAndYouthPercentage(t *testing.T) {
	catalog := `{"version":"1.0","type":"catalog","exercises":[],"programs":[{"name":"P","num_weeks":1,"num_days":1,"prescribed_sets":[{"exercise":"Barbell Bench Press","week":1,"day":1,"set_number":1,"reps":5,"rep_type":"reps","percentage":0.75}]}]}`
	res := LintCatalog([]byte(catalog), youthCtx())
	joined := strings.Join(res.Warnings, " ")
	if !strings.Contains(joined, "incompatible") {
		t.Errorf("expected incompatible-equipment warning, got %v", res.Warnings)
	}
	if !strings.Contains(joined, "percentage") {
		t.Errorf("expected youth percentage-loading warning, got %v", res.Warnings)
	}
}

func TestExtractJSON_BraceInsideStringLiteral(t *testing.T) {
	// A brace inside a notes string must not break extraction — the old
	// depth-counter closed the object early on the inner '}'.
	content := `<reasoning>ok</reasoning>{"version":"1.0","type":"catalog","note":"EMOM {1 set/min}","x":1}`
	got := extractJSON(content)
	if got == nil {
		t.Fatal("extractJSON returned nil for JSON containing a braced string")
	}
	if !strings.Contains(string(got), "EMOM {1 set/min}") {
		t.Fatalf("extracted JSON lost the braced string: %s", got)
	}
}

func TestExtractResponse_ReasoningFallbackOnUnclosedTag(t *testing.T) {
	content := `<reasoning>linear progression, safe loads {"version":"1.0","type":"catalog","exercises":[],"programs":[]}`
	catalog, reasoning := extractResponse(content)
	if catalog == nil {
		t.Fatal("expected JSON to be extracted despite unclosed reasoning tag")
	}
	if reasoning == "" {
		t.Fatal("expected reasoning fallback to capture text before the JSON")
	}
	if strings.Contains(reasoning, "\"version\"") {
		t.Fatalf("reasoning should stop at the JSON start, got %q", reasoning)
	}
}

func TestComposeSystemPrompt_OverrideIsAppendedNotSubstituted(t *testing.T) {
	base := "YOUTH ATHLETE SAFETY RULES ... CATALOGJSON SCHEMA ..."
	out := composeSystemPrompt(base, "Use a terse tone.")
	if !strings.Contains(out, "YOUTH ATHLETE SAFETY RULES") {
		t.Error("override must not strip the youth safety block")
	}
	if !strings.Contains(out, "CATALOGJSON SCHEMA") {
		t.Error("override must not strip the schema")
	}
	if !strings.Contains(out, "Use a terse tone.") {
		t.Error("override text should be present")
	}
	if got := composeSystemPrompt(base, "   "); got != base {
		t.Error("blank override should return base unchanged")
	}
}
