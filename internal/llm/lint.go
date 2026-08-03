package llm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/carpenike/replog/internal/importers"
)

// MethodologyKeyGalpinThreeToFive identifies the coach-selected adult
// strength framework whose output envelope receives advisory structural lint.
const MethodologyKeyGalpinThreeToFive = "galpin-3-to-5"

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
	return lintCatalog(catalogJSON, ctx, false)
}

// LintCatalogWithCoachDirections extends LintCatalog with the coach's stated
// intent. Galpin permits a progression rule only when the coach asks for one.
func LintCatalogWithCoachDirections(catalogJSON []byte, ctx *AthleteContext, coachDirections string) LintResult {
	return lintCatalog(catalogJSON, ctx, coachDirectionsRequestProgression(coachDirections))
}

func lintCatalog(catalogJSON []byte, ctx *AthleteContext, progressionRequested bool) LintResult {
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
	if ctx.Methodology != nil && ctx.Methodology.Key == MethodologyKeyGalpinThreeToFive {
		res.Warnings = append(res.Warnings, lintGalpinThreeToFive(parsed, progressionRequested)...)
	}

	return res
}

func lintGalpinThreeToFive(parsed *importers.ParsedFile, progressionRequested bool) []string {
	if len(parsed.Programs) == 0 {
		return []string{"Galpin 3-to-5: the draft must contain one looping program template."}
	}

	var warnings []string
	if len(parsed.Programs) != 1 {
		warnings = append(warnings, fmt.Sprintf("Galpin 3-to-5: the draft contains %d program templates; use one looping program template.", len(parsed.Programs)))
	}
	for _, program := range parsed.Programs {
		template := program.Template
		name := template.Name
		if name == "" {
			name = "generated program"
		}
		prefix := fmt.Sprintf("Galpin 3-to-5 %q:", name)

		if !template.IsLoop || template.NumWeeks != 1 {
			warnings = append(warnings, prefix+" use a one-week looping template.")
		}
		if template.NumDays < 3 || template.NumDays > 5 {
			warnings = append(warnings, prefix+" program 3 to 5 days per week.")
		}

		setsByDay := make(map[int]map[string]int)
		invalidDay := false
		invalidReps := false
		missingRest := false
		invalidRest := make(map[int]struct{})
		for _, set := range template.PrescribedSets {
			if set.Day < 1 || set.Day > template.NumDays {
				invalidDay = true
			}
			exercise := normalizeName(set.Exercise)
			if exercise == "" {
				exercise = "(unnamed exercise)"
			}
			if setsByDay[set.Day] == nil {
				setsByDay[set.Day] = make(map[string]int)
			}
			setsByDay[set.Day][exercise]++

			if set.Reps == nil || *set.Reps < 3 || *set.Reps > 5 {
				invalidReps = true
			}
			if set.RestSeconds == nil {
				missingRest = true
			} else if *set.RestSeconds < 180 || *set.RestSeconds > 300 {
				invalidRest[*set.RestSeconds] = struct{}{}
			}
		}

		if invalidDay {
			warnings = append(warnings, prefix+" keep prescribed sets within the template's declared training days.")
		}
		for day := 1; day <= template.NumDays; day++ {
			exercises := setsByDay[day]
			if len(exercises) < 3 || len(exercises) > 5 {
				warnings = append(warnings, fmt.Sprintf("%s day %d has %d exercises; use 3 to 5.", prefix, day, len(exercises)))
			}
			names := make([]string, 0, len(exercises))
			for exercise := range exercises {
				names = append(names, exercise)
			}
			sort.Strings(names)
			for _, exercise := range names {
				if count := exercises[exercise]; count < 3 || count > 5 {
					warnings = append(warnings, fmt.Sprintf("%s day %d %s has %d work sets; use 3 to 5.", prefix, day, exercise, count))
				}
			}
		}
		if invalidReps {
			warnings = append(warnings, prefix+" use 3 to 5 reps for every work set; AMRAP rows do not fit this framework.")
		}
		if missingRest {
			warnings = append(warnings, prefix+" set rest_seconds on every work set to 180 through 300 seconds.")
		}
		if len(invalidRest) > 0 {
			values := make([]int, 0, len(invalidRest))
			for value := range invalidRest {
				values = append(values, value)
			}
			sort.Ints(values)
			warnings = append(warnings, fmt.Sprintf("%s rest_seconds values %v are outside the 180 through 300 second range.", prefix, values))
		}
		if len(template.ProgressionRules) > 0 && !progressionRequested {
			warnings = append(warnings, prefix+" includes progression rules without an explicit coach request; review or remove them.")
		}
	}

	return warnings
}

func coachDirectionsRequestProgression(directions string) bool {
	directions = strings.ToLower(directions)
	return strings.Contains(directions, "progression") ||
		strings.Contains(directions, "increase load") ||
		strings.Contains(directions, "add weight")
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
