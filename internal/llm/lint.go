package llm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/carpenike/replog/internal/importers"
)

// LintResult holds the outcome of the deterministic post-generation lint.
type LintResult struct {
	// Warnings are human-readable advisories surfaced to the coach in the
	// preview. A non-empty list does NOT block the draft — the coach reviews
	// and edits before anything is committed.
	Warnings []string

	// UnknownExercises and IncompatibleExercises are the structured name
	// lists behind the corresponding warnings (sorted, deduplicated). They
	// drive Generate's bounded lint-repair retry: both classes are fixable
	// by substituting a valid catalog exercise, unlike the loading-rule
	// advisories which stay warning-only.
	UnknownExercises      []string
	IncompatibleExercises []string
}

// LintCatalog runs deterministic checks over a generated CatalogJSON against
// the context the model was given, catching the gap between what the prompt
// asks for and what the system can guarantee. The LLM is instructed to only
// use catalog exercises and to respect youth loading rules, but nothing
// enforces it — an invented or incompatible exercise name is otherwise
// silently dropped at import time. This lint makes those violations visible.
//
// It is intentionally advisory (warnings, not hard failure): the coach is the
// backstop, and a false positive must never block a legitimate draft. The
// caller persists Warnings on the generation row for display in the preview.
func LintCatalog(catalogJSON []byte, ctx *AthleteContext) LintResult {
	var res LintResult
	if ctx == nil {
		return res
	}

	parsed, err := importers.ParseCatalogJSON(strings.NewReader(string(catalogJSON)))
	if err != nil {
		// Parse errors are handled separately on the generation path; nothing
		// to lint against a payload we couldn't parse.
		return res
	}

	// Build a case-insensitive lookup of catalog exercises → compatibility.
	type catEntry struct{ compatible bool }
	catalog := make(map[string]catEntry, len(ctx.ExerciseCatalog))
	for _, e := range ctx.ExerciseCatalog {
		catalog[normalizeName(e.Name)] = catEntry{compatible: e.Compatible}
	}

	youth := ctx.Athlete.Tier != nil
	foundationalOrIntermediate := youth &&
		(*ctx.Athlete.Tier == "foundational" || *ctx.Athlete.Tier == "intermediate")

	// Track unique violations so a set-per-row program doesn't emit the same
	// warning dozens of times.
	unknown := map[string]bool{}
	incompatible := map[string]bool{}
	percentageYouth := map[string]bool{}

	for _, prog := range parsed.Programs {
		for _, ps := range prog.Template.PrescribedSets {
			name := strings.TrimSpace(ps.Exercise)
			if name == "" {
				continue
			}
			entry, ok := catalog[normalizeName(name)]
			if !ok {
				unknown[name] = true
				continue
			}
			if !entry.compatible {
				incompatible[name] = true
			}
			if foundationalOrIntermediate && ps.Percentage != nil {
				percentageYouth[name] = true
			}
		}
	}

	res.UnknownExercises = sortedKeys(unknown)
	res.IncompatibleExercises = sortedKeys(incompatible)

	for _, name := range res.UnknownExercises {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"Exercise %q is not in the athlete's catalog — it will be dropped on import. Rename it to an existing exercise or add it first.", name))
	}
	for _, name := range res.IncompatibleExercises {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"Exercise %q requires equipment the athlete does not have (marked incompatible).", name))
	}
	for _, name := range sortedKeys(percentageYouth) {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"Exercise %q uses percentage-based loading, which is not appropriate for a foundational/intermediate youth athlete — use absolute weight.", name))
	}

	return res
}

// sortedKeys returns the map's keys sorted, for deterministic warning order.
func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MarshalWarnings serializes lint warnings to a JSON array string for
// persistence. Returns "" when there are no warnings so the column stays NULL.
func MarshalWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	b, err := json.Marshal(warnings)
	if err != nil {
		return ""
	}
	return string(b)
}

// normalizeName lowercases and collapses whitespace for tolerant exercise-name
// matching (the LLM may vary capitalization or spacing).
func normalizeName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
