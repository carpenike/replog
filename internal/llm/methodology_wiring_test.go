package llm

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// TestExerciseInMethodologyScope unit-tests the per-exercise admit/drop
// decision used by buildExerciseCatalog when a methodology is bound.
// Covers the central HOF-005 D3 semantics (pattern + explicit-list +
// equipment gate; untagged-doesn't-pattern-admit; dual-tag conservative).
func TestExerciseInMethodologyScope(t *testing.T) {
	allowedPatterns := map[string]struct{}{"push": {}, "pull": {}, "squat": {}}
	allowedExercises := map[int64]struct{}{42: {}}
	allowedEquipment := map[int64]struct{}{1: {}, 2: {}}

	cases := []struct {
		name       string
		exerciseID int64
		patterns   []string
		required   []int64
		want       bool
	}{
		// Pattern admission
		{"in by single allowed pattern", 1, []string{"push"}, nil, true},
		{"in by multiple allowed patterns", 2, []string{"push", "pull"}, nil, true},
		{"out by single disallowed pattern", 3, []string{"carry"}, nil, false},
		// Conservative dual-tag: ANY disallowed → out
		{"out by dual-tag with one disallowed (carry sneak)", 4, []string{"squat", "carry"}, nil, false},
		// Untagged: not pattern-admitted, not in explicit list → out
		{"out when untagged", 5, nil, nil, false},
		// Explicit-list override admits even untagged or disallowed-tagged
		{"in by explicit list (untagged)", 42, nil, nil, true},
		{"in by explicit list (disallowed patterns)", 42, []string{"carry", "ground"}, nil, true},
		// Equipment gate
		{"out by required equipment not allowed", 6, []string{"push"}, []int64{99}, false},
		{"in when required equipment is allowed", 7, []string{"push"}, []int64{1}, true},
		{"out when ANY required equipment disallowed", 8, []string{"push"}, []int64{1, 99}, false},
		// Equipment gate applies to explicit-list admits too
		{"out by equipment even with explicit-list admission", 42, nil, []int64{99}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := exerciseInMethodologyScope(tc.exerciseID, tc.patterns, tc.required, allowedPatterns, allowedExercises, allowedEquipment)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBuildAthleteContext_ScopesCatalogByMethodology is the end-to-end
// integration test that confirms BuildAthleteContext resolves the
// methodology and returns a scoped catalog (HOF-005 success criteria).
func TestBuildAthleteContext_ScopesCatalogByMethodology(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")

	// Generation path — Methodology resolves from tier default (yessis-1x20).
	ctx, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{RequireMethodology: true})
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if ctx.Methodology == nil {
		t.Fatal("Methodology projection should be set (tier-default resolution)")
	}
	if ctx.Methodology.Key != "yessis-1x20" {
		t.Errorf("Methodology.Key = %q, want yessis-1x20", ctx.Methodology.Key)
	}

	// Catalog should NOT contain barbell exercises (the structural
	// "foundational athlete cannot reach a barbell" guarantee).
	for _, e := range ctx.ExerciseCatalog {
		if strings.Contains(strings.ToLower(e.Name), "barbell") {
			t.Errorf("yessis-1x20 catalog should exclude %q (Barbell equipment not in allow-list)", e.Name)
		}
		if e.Name == "Squat" || e.Name == "Bench Press" || e.Name == "Deadlift" || e.Name == "Overhead Press" {
			t.Errorf("yessis-1x20 catalog should exclude %q (barbell main)", e.Name)
		}
		if e.Name == "Power Clean" {
			t.Errorf("yessis-1x20 catalog should exclude %q (barbell + sport_performance tier)", e.Name)
		}
	}

	// AND it should contain bodyweight / band / DB exercises that fit
	// the allow-list — at least a handful, otherwise the scope is too tight.
	expectedInScope := []string{"Push-up", "Goblet Squat", "Plank"}
	have := map[string]bool{}
	for _, e := range ctx.ExerciseCatalog {
		have[e.Name] = true
	}
	for _, want := range expectedInScope {
		if !have[want] {
			t.Errorf("yessis-1x20 catalog should include %q", want)
		}
	}
}

// TestBuildAthleteContext_NoMethodologyForUnmappedYouthTier confirms the
// fail-fast behavior — a youth athlete whose tier has no mapped methodology
// must NOT generate a rules-less program (ADR 016 D2).
func TestBuildAthleteContext_NoMethodologyForUnmappedYouthTier(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	// Intentionally NOT calling seedMethodologies — yessis-1x20 absent.
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")

	_, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{RequireMethodology: true})
	if err == nil {
		t.Fatal("expected error: youth athlete without resolvable methodology")
	}
	if !strings.Contains(err.Error(), "methodology") {
		t.Errorf("error should mention methodology resolution; got %v", err)
	}
}

// TestBuildAthleteContext_AdultFallback confirms an adult athlete with no
// methodology selection returns a nil methodology projection (back-compat
// fallback to the in-code generic adult block; ADR 016 D1).
func TestBuildAthleteContext_AdultFallback(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	athleteID := seedAthlete(t, db, "Adult", "", "")

	ctx, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{RequireMethodology: true})
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if ctx.Methodology != nil {
		t.Errorf("adult with no MethodologyID should have nil Methodology; got %+v", ctx.Methodology)
	}
}

// TestBuildAthleteContext_ExplicitMethodologyOverridesTier confirms a coach
// can hand a youth athlete a non-default methodology (e.g. int-youth-gpp
// for a foundational athlete) — that's a valid coach decision.
func TestBuildAthleteContext_ExplicitMethodologyOverridesTier(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")

	int_, err := models.GetMethodologyByKey(context.Background(), db, "int-youth-gpp")
	if err != nil {
		t.Fatalf("get int-youth-gpp: %v", err)
	}
	ctx, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{
		RequireMethodology: true,
		MethodologyID:      &int_.ID,
	})
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if ctx.Methodology == nil || ctx.Methodology.Key != "int-youth-gpp" {
		t.Errorf("expected int-youth-gpp; got %+v", ctx.Methodology)
	}
}

// TestBuildAthleteContext_EmptyExemplarFallback confirms HOF-005 D4:
// int-youth-gpp seeds with an empty reference_programs array, so when a
// coach picks it and supplies no override list, the references fall back
// to the audience-filtered default (today's HOF-002 behavior) — NOT zero.
func TestBuildAthleteContext_EmptyExemplarFallback(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")

	int_, _ := models.GetMethodologyByKey(context.Background(), db, "int-youth-gpp")
	ctx, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{
		RequireMethodology: true,
		MethodologyID:      &int_.ID,
	})
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if len(ctx.ReferencePrograms) == 0 {
		t.Error("empty-exemplar methodology should fall back to audience-filtered references, not zero")
	}
}

// TestBuildAthleteContext_MethodologyExemplarsOverrideAudienceDefault
// confirms that when a methodology has exemplars AND the coach supplied
// no override, the exemplars (not the broad audience list) are used.
func TestBuildAthleteContext_MethodologyExemplarsOverrideAudienceDefault(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")

	// yessis-1x20 has exactly one exemplar (Foundations 1×20).
	yessis, _ := models.GetMethodologyByKey(context.Background(), db, "yessis-1x20")
	ctx, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{
		RequireMethodology: true,
		MethodologyID:      &yessis.ID,
	})
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if len(ctx.ReferencePrograms) != 1 {
		t.Errorf("expected 1 reference (yessis-1x20's single exemplar), got %d", len(ctx.ReferencePrograms))
	}
	if len(ctx.ReferencePrograms) > 0 && ctx.ReferencePrograms[0].Name != "Foundations 1×20" {
		t.Errorf("expected Foundations 1×20; got %q", ctx.ReferencePrograms[0].Name)
	}
}

// TestBuildAthleteContext_CoachReferenceIDsOverrideMethodologyExemplars
// confirms the coach-supplied ReferenceTemplateIDs always win over the
// methodology's default exemplars (HOF-005 D4 precedence).
func TestBuildAthleteContext_CoachReferenceIDsOverrideMethodologyExemplars(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")

	yessis, _ := models.GetMethodologyByKey(context.Background(), db, "yessis-1x20")

	// Find a template id that's NOT one of yessis-1x20's exemplars.
	allTemplates, _ := models.ListProgramTemplates(context.Background(), db)
	var overrideID int64
	for _, t := range allTemplates {
		if t.Name == "Foundations 1×15" {
			overrideID = t.ID
			break
		}
	}
	if overrideID == 0 {
		t.Fatal("could not find Foundations 1×15 in seeded catalog")
	}

	ctx, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{
		RequireMethodology:   true,
		MethodologyID:        &yessis.ID,
		ReferenceTemplateIDs: []int64{overrideID},
	})
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}
	if len(ctx.ReferencePrograms) != 1 {
		t.Fatalf("expected 1 reference (the coach override), got %d", len(ctx.ReferencePrograms))
	}
	if ctx.ReferencePrograms[0].Name != "Foundations 1×15" {
		t.Errorf("expected coach-supplied override; got %q", ctx.ReferencePrograms[0].Name)
	}
}

// TestAthleteContext_AuditPayloadLeanness is the audit-bloat guard
// (HOF-005 [fix] AUDIT clause). The marshalled AthleteContext that lands
// in generations.context_json must NOT include the full methodology
// Definition text — that text already lives in generations.prompt and
// duplicating it costs ~1.5KB per audit row.
func TestAthleteContext_AuditPayloadLeanness(t *testing.T) {
	db := testDB(t)
	seedCatalogForTest(t, db)
	seedMethodologies(t, db)
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")

	ctx, err := BuildAthleteContext(context.Background(), db, athleteID, time.Now(), BuildContextOptions{RequireMethodology: true})
	if err != nil {
		t.Fatalf("BuildAthleteContext: %v", err)
	}

	// Confirm the methodology projection is set (so we're actually
	// testing the leanness invariant, not just "nothing was attached").
	if ctx.Methodology == nil {
		t.Fatal("Methodology projection should be set")
	}

	// Marshal as the Generate() audit path does, then look for tell-tale
	// strings from the foundational definition (~1.7KB of text).
	marshalled, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}
	body := string(marshalled)

	for _, leaked := range []string{
		"FOUNDATIONAL TIER RULES",
		"1 SET of 20 REPS",
		"Do NOT use barbell",
	} {
		if strings.Contains(body, leaked) {
			t.Errorf("marshalled AthleteContext leaked %q from methodology definition — only the lean projection should ship", leaked)
		}
	}

	// Spot-check the lean projection IS present.
	if !strings.Contains(body, `"key":"yessis-1x20"`) {
		t.Error("lean Methodology projection should include key")
	}
	if !strings.Contains(body, `"reference_programs_count"`) {
		t.Error("lean Methodology projection should include reference_programs_count")
	}
}

// TestBuildSystemPrompt_AdultWithMethodology confirms adults with a
// coach-selected methodology get the methodology Definition (not the
// generic adult block) — the path Phase 3's adult selector enables.
func TestBuildSystemPrompt_AdultWithMethodology(t *testing.T) {
	ctx := &AthleteContext{
		Athlete: AthleteProfile{Name: "Adult"},
	}
	ctx.methodology = loadSeededMethodology(t, "531")
	prompt := buildSystemPrompt(ctx)
	if !strings.Contains(prompt, "5/3/1 METHODOLOGY") {
		t.Error("adult-with-methodology prompt should contain the 5/3/1 definition block")
	}
	if strings.Contains(prompt, "ADULT ATHLETE PROGRAMMING RULES") {
		t.Error("adult-with-methodology prompt should suppress the generic ADULT block")
	}
}

// TestBuildSystemPrompt_YouthSafetyFloorsAlwaysEmit confirms the youth
// preamble + safety floors emit for every youth generation regardless of
// which methodology is bound — the floors are NEVER controlled by data.
func TestBuildSystemPrompt_YouthSafetyFloorsAlwaysEmit(t *testing.T) {
	tier := "foundational"
	for _, key := range []string{"yessis-1x20", "yessis-1x15", "yessis-sport-performance", "int-youth-gpp"} {
		t.Run(key, func(t *testing.T) {
			ctx := &AthleteContext{
				Athlete: AthleteProfile{Name: "Youth", Tier: &tier},
			}
			ctx.methodology = loadSeededMethodology(t, key)
			prompt := buildSystemPrompt(ctx)
			for _, floor := range []string{
				"YOUTH ATHLETE SAFETY RULES (MANDATORY",
				"GENERAL YOUTH RULES",
				"NEVER program 1RM testing",
				"48 hours rest between sessions",
			} {
				if !strings.Contains(prompt, floor) {
					t.Errorf("%s: safety-floor phrase %q missing from prompt", key, floor)
				}
			}
		})
	}
}

// TestGenerate_YouthMissingMethodologyFailsFast is the integration-level
// fail-fast assertion (corresponds to BuildAthleteContext's error path).
// Confirms Generate surfaces the error rather than producing a draft.
func TestGenerate_YouthMissingMethodologyFailsFast(t *testing.T) {
	db := testDB(t)
	athleteID := seedAthlete(t, db, "Youth", "foundational", "")
	// Intentionally NOT seeding methodologies.

	_, err := Generate(context.Background(), db, &MockProvider{}, GenerationRequest{
		AthleteID:   athleteID,
		ProgramName: "Test",
		NumWeeks:    4,
		NumDays:     3,
	})
	if err == nil {
		t.Fatal("expected error when youth methodology cannot resolve")
	}
	if !strings.Contains(err.Error(), "methodology") {
		t.Errorf("error should mention methodology resolution; got %v", err)
	}
}

// resolveMethodology unit-test for the unmapped-tier path (covered via
// integration above; this asserts the message format).
func TestResolveMethodology_UnmappedTier(t *testing.T) {
	db := testDB(t)
	tier := "bogus_tier"
	profile := &AthleteProfile{Tier: &tier}

	_, err := resolveMethodology(context.Background(), db, profile, BuildContextOptions{RequireMethodology: true})
	if err == nil {
		t.Fatal("expected error for unmapped tier")
	}
	if !strings.Contains(err.Error(), "bogus_tier") {
		t.Errorf("error should name the offending tier; got %v", err)
	}
}

// TestResolveMethodology_NotRequireSkipsResolution confirms the form-preview
// path (RequireMethodology=false) returns nil without error even for youth.
func TestResolveMethodology_NotRequireSkipsResolution(t *testing.T) {
	db := testDB(t)
	tier := "foundational"
	profile := &AthleteProfile{Tier: &tier}

	m, err := resolveMethodology(context.Background(), db, profile, BuildContextOptions{RequireMethodology: false})
	if err != nil {
		t.Fatalf("form-preview path should not error; got %v", err)
	}
	if m != nil {
		t.Errorf("form-preview path should return nil methodology; got %+v", m)
	}
}

// avoid unused-import warning under refactors
var _ = sql.Open
