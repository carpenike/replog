package main

import (
	"sort"
	"strings"
	"testing"
)

// TestMCPTools_ExhaustiveAndStable asserts the MCP tool catalog is EXACTLY
// the curated set HOF-013 specifies — no more, no less.
//
// mcpTools() is the single source of truth: buildMCPServer registers from it,
// and this test asserts it. If a future PR adds a tool without updating this
// list, the test fails LOUDLY. That is the intended behavior — any expansion
// of the MCP surface is a deliberate decision that requires updating this test
// (and re-reading ADR 007 / 015's "no automated coaching" line to confirm the
// new tool is a clerical log/read or a draft proposal, never a coaching
// decision the app commits on its own).
func TestMCPTools_ExhaustiveAndStable(t *testing.T) {
	want := []string{
		// reads
		"dashboard",
		"list_exercises",
		"get_athlete",
		"list_workouts",
		"get_workout",
		"get_prescription",
		"list_training_maxes",
		"get_training_max_history",
		"list_journal",
		"list_athlete_programs",
		"list_athlete_equipment",
		"get_load_summary",
		"get_pitch_smart",
		"list_throwing_sessions",
		"list_conditioning_sessions",
		"list_skill_sessions",
		"list_recovery_checkins",
		"list_bio_samples",
		// clerical writes
		"create_workout",
		"add_workout_set",
		"update_workout_set",
		"delete_workout_set",
		"update_workout_notes",
		"create_body_weight",
		"create_athlete_note",
		"create_throwing_session",
		"create_conditioning_session",
		"create_skill_session",
		"create_recovery_checkin",
		"create_bio_sample",
		// program draft (enqueue + status + cancel)
		"generate_submit",
		"generation_status",
		"generation_cancel",
		// ad-hoc WOD (submit + log; status reuses generation_status)
		"wod_submit",
		"wod_log",
	}

	got := make([]string, 0, len(mcpTools()))
	for _, tl := range mcpTools() {
		got = append(got, tl.name)
	}

	sort.Strings(want)
	sortedGot := append([]string{}, got...)
	sort.Strings(sortedGot)

	if len(sortedGot) != len(want) {
		t.Fatalf("tool count = %d, want %d\ngot:\n  %s\nwant:\n  %s",
			len(sortedGot), len(want),
			strings.Join(sortedGot, "\n  "),
			strings.Join(want, "\n  "))
	}
	for i := range want {
		if sortedGot[i] != want[i] {
			t.Errorf("tool mismatch at index %d:\n  got  = %q\n  want = %q\nfull got:\n  %s\nfull want:\n  %s",
				i, sortedGot[i], want[i],
				strings.Join(sortedGot, "\n  "),
				strings.Join(want, "\n  "))
		}
	}
}

// TestMCPTools_NoCoachingDecisionTools is the doctrine guard: it asserts that
// no tool whose NAME implies a coaching DECISION (a commit the app makes on
// its own, bypassing the human coach's approval) is present in the catalog.
//
// The absence of these tools IS the safety property of ADR 007 / 015 — the
// app is a logbook; the LLM drafts proposals a coach reviews. Each forbidden
// name below maps to a real handler that exists on the REST surface and is
// DELIBERATELY not exposed via MCP:
//
//   - create_season_phase / delete_season_phase — periodization decisions.
//   - create_training_max — a direct TM write IS a progression decision
//     (read history + list current are fine; mutating the TM is not).
//   - assign_program / deactivate_program / reactivate_program /
//     delete_program — program assignment is a coaching decision.
//   - promote_athlete — tier promotion is the coach's call.
//   - apply_cycle_review / cycle_review — applying TM bumps is a decision.
//   - generation_execute — the COMMIT step of a drafted program; the human's
//     click on the webui is the approval (generate_submit/status/cancel only).
//   - program/template/rule/set authoring — operator surface, not MCP.
//
// wod_log is the deliberate, documented exception and the reason this comment
// is load-bearing. It IS exposed (see the allowlist above) even though it is
// "execute-shaped" — it calls MarkGenerationExecuted and sets the SAME
// ExecutedAt column generation_execute sets. The distinction that keeps the
// boundary intact is WHAT it materializes, not whether ExecutedAt is set:
// wod_log writes an assignment_id-NULL ad-hoc resistance workout (no program
// assignment, no progression, no training-max change, fully reversible) — a
// Group-B log in the same class as create_workout. generation_execute commits
// an assigned multi-week program that drives progression — a coaching decision
// that stays on the webui (ADR 007 / 015). If a future change makes wod_log
// (or any "_log") assign a program, bump a TM, or drive progression, it has
// crossed the line and must come off the MCP surface.
func TestMCPTools_NoCoachingDecisionTools(t *testing.T) {
	present := map[string]bool{}
	for _, tl := range mcpTools() {
		present[tl.name] = true
	}

	// Exact names that must NEVER appear.
	mustNotExist := []string{
		"create_season_phase",
		"delete_season_phase",
		"create_training_max",
		"assign_program",
		"deactivate_program",
		"reactivate_program",
		"delete_program",
		"promote_athlete",
		"apply_cycle_review",
		"cycle_review",
		"generation_execute",
		"create_program_template",
		"update_program_template",
		"delete_program_template",
		"copy_week",
		"add_prescribed_set",
		"set_progression_rule",
		"assign_exercise",
	}
	for _, name := range mustNotExist {
		if present[name] {
			t.Errorf("forbidden coaching-decision tool %q is exposed via MCP — remove it (ADR 007/015 no automated coaching)", name)
		}
	}

	// Substring guard: catch renamed variants of the commit/decision verbs.
	forbiddenSubstrings := []string{
		"execute",
		"promote",
		"season_phase",
		"cycle_review",
	}
	for _, tl := range mcpTools() {
		for _, bad := range forbiddenSubstrings {
			if strings.Contains(tl.name, bad) {
				t.Errorf("tool %q contains forbidden substring %q — coaching-decision verbs may not be exposed via MCP", tl.name, bad)
			}
		}
	}
}

// TestBuildMCPServer_RegistersWithoutPanic exercises the real registration
// path: buildMCPServer calls mcp.AddTool for every tool, which panics if a
// tool's input type cannot be reflected into a JSON schema. A nil user/prefs
// is fine here — we are validating registration, not invocation.
func TestBuildMCPServer_RegistersWithoutPanic(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("buildMCPServer panicked during tool registration: %v", rec)
		}
	}()
	if s := buildMCPServer(nil, nil, nil); s == nil {
		t.Fatal("buildMCPServer returned nil")
	}
}
