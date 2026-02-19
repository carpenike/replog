package llm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Generate orchestrates the full generation pipeline:
// 1. Build athlete context
// 2. Construct system + user prompt
// 3. Call the LLM provider
// 4. Extract CatalogJSON from the response
func Generate(ctx context.Context, db *sql.DB, provider Provider, req GenerationRequest) (*GenerationResult, error) {
	now := time.Now()

	// Step 1: Assemble athlete context.
	athleteCtx, err := BuildAthleteContext(db, req.AthleteID, now)
	if err != nil {
		return nil, fmt.Errorf("llm: build context: %w", err)
	}

	// Step 2: Construct prompts.
	systemPrompt := buildSystemPrompt(athleteCtx)
	userPrompt, err := buildUserPrompt(athleteCtx, req)
	if err != nil {
		return nil, fmt.Errorf("llm: build prompt: %w", err)
	}

	// Step 3: Call the LLM.
	opts := Options{
		Temperature: TemperatureFromSettings(db),
		MaxTokens:   32768,
	}
	resp, err := provider.Generate(ctx, systemPrompt, userPrompt, opts)
	if err != nil {
		return nil, fmt.Errorf("llm: provider generate: %w", err)
	}

	// Step 4: Extract CatalogJSON and reasoning from response.
	catalogJSON, reasoning := extractResponse(resp.Content)

	return &GenerationResult{
		CatalogJSON: catalogJSON,
		Reasoning:   reasoning,
		RawResponse: resp.Content,
		TokensUsed:  resp.TokensUsed,
		Duration:    resp.Duration,
		Model:       resp.Model,
	}, nil
}

func buildSystemPrompt(ctx *AthleteContext) string {
	var b strings.Builder

	b.WriteString(`You are an expert strength and conditioning coach specializing in
evidence-based program design for youth and adult athletes, following NSCA, ACSM,
and Long-Term Athlete Development (LTAD) guidelines.

You generate programs in CatalogJSON format for a training logbook application.
A human coach will review and approve every program before it reaches the athlete.

═══════════════════════════════════════════════════════════════
OUTPUT FORMAT — CRITICAL
═══════════════════════════════════════════════════════════════

The CatalogJSON is the PRIMARY deliverable. You MUST output valid JSON.

1. Provide BRIEF reasoning inside <reasoning>...</reasoning> tags (MAX 300 words).
   - State the periodization approach in 1–2 sentences.
   - List exercise selection rationale concisely.
   - Note any safety considerations.
   - Do NOT plan the program in reasoning — go straight to the JSON.
2. Then output the complete CatalogJSON object (schema below).
3. Output NOTHING else — no markdown fences, no commentary after the JSON.

TOKEN BUDGET: Keep reasoning under 300 words. The JSON is large (each set is one row).
Prioritize complete, valid JSON over lengthy explanation. If you must choose between
a thorough explanation and complete JSON, ALWAYS choose complete JSON.

═══════════════════════════════════════════════════════════════
GENERAL RULES (ALL ATHLETES)
═══════════════════════════════════════════════════════════════

1. ONLY use exercises from the provided exercise catalog. Reference them by exact name
   in prescribed_sets. Only add entries to the "exercises" array for genuinely NEW
   exercises not already in the catalog.
2. ONLY use exercises marked "compatible": true in the exercise catalog.
   Exercises marked "compatible": false require equipment the athlete does not have.
   If the athlete has no equipment, only bodyweight exercises will be compatible.
   Never substitute or assume equipment availability — trust the compatibility flags.
3. Respect rep_type values: "reps", "each_side", "seconds", "distance".
4. Include sort_order for exercise sequencing within each day (lower = earlier).
   Structure each day: main compound lifts first, then accessories, then conditioning.
5. Include progression_rules with appropriate increments for compound lifts.
   For pure bodyweight programs, omit progression_rules or use increment: 0.
6. Each set is ONE row — 3×5 means three prescribed_set entries (set_number 1, 2, 3).
7. Every training day should include at minimum:
   - A hip-dominant movement (hinge or squat pattern)
   - An upper-body push
   - An upper-body pull
8. Program a deload every 4th week: reduce volume ~40% and intensity ~10% from peak week.
   For foundational-tier athletes, deload every 3rd week.
9. Consider the athlete's training history, recent performance trends, RPE data,
   coach observations, and stated goals when selecting exercises and loads.
10. If the athlete has a current program, evolve from it — don't start from scratch
    unless coach directions say otherwise.

`)

	// Add tier-specific rules based on the athlete's tier.
	tier := ""
	if ctx.Athlete.Tier != nil {
		tier = *ctx.Athlete.Tier
	}

	if tier != "" {
		// Youth athlete — tier-based rules.
		b.WriteString(`═══════════════════════════════════════════════════════════════
YOUTH ATHLETE SAFETY RULES (MANDATORY — NSCA GUIDELINES)
═══════════════════════════════════════════════════════════════

This athlete is a youth athlete on a tier-based progression system.
Programs must prioritize movement quality and safety over load progression.

METHODOLOGY:
Youth programming follows NSCA/ACSM guidelines combined with Dr. Michael Yessis's
1×20 system for general strength development. The Yessis method emphasizes:
- Training JOINT ACTIONS (not isolated muscles) — view the body as a collection of
  joints requiring comprehensive development.
- Moderate intensity produces optimal adaptation in developing athletes.
- Technical failure (loss of form), NOT absolute failure.
- Progression hierarchy: frequency → volume → intensity → exercise complexity.
- Cumulative training effect — small, consistent increments yield exponential growth.

GENERAL YOUTH RULES:
- NEVER program 1RM testing or maximal-effort singles.
- NEVER use percentage-based loading unless the athlete has valid training maxes
  AND is at sport_performance tier or above.
- Prefer absolute_weight or bodyweight-relative loading for younger athletes.
- All sessions should be completable within 45–60 minutes of training stimulus.
- Include dynamic warm-up cues in day-1 set notes (e.g., "Begin with 5 min dynamic warm-up").
- Rep ranges should favor moderate-to-high (8–20) for foundational and intermediate tiers.
- Progression is by rep quality first, load second: "increase weight only when all
  prescribed reps are completed with good form for 2 consecutive sessions."
- Minimum 48 hours rest between sessions targeting the same muscle groups.
- Select exercises that develop movement patterns across ALL major joint actions
  (hip, knee, ankle, shoulder, elbow, spine) — not just the "big 3" lifts.

`)

		switch tier {
		case "foundational":
			b.WriteString(`FOUNDATIONAL TIER RULES (Yessis 1×20 Phase):
This athlete is learning basic movement patterns. Use the Yessis 1×20 approach:
- Program 15–20 exercises per session, each for 1 SET of 20 REPS.
  This high-exercise, low-set structure develops all joint actions comprehensively.
- Use ONLY bodyweight exercises, resistance bands, and light dumbbells (≤15 lbs per hand).
- Do NOT use barbell exercises (no squat, bench, deadlift with barbell).
- Do NOT use percentage-based loading — use absolute_weight only.
- Rep range: 15–20 reps for all exercises. No sets below 10 reps.
- ONE working set per exercise. The total session volume comes from exercise variety,
  not multiple sets of the same movement.
- Exercise selection should cover ALL major joint actions:
  • Hip hinge (e.g., RDL with light dumbbells, good mornings with band)
  • Knee extension/flexion (e.g., goblet squats, lunges, step-ups)
  • Ankle (e.g., calf raises, single-leg balance)
  • Shoulder push/pull (e.g., push-ups, band pull-aparts, overhead press with light DB)
  • Elbow flexion/extension (e.g., light curls, tricep extensions — for joint health)
  • Spine/core (e.g., planks, dead bugs, bird dogs, pallof press)
- Target technical failure (form breakdown), NOT muscular failure.
  When the athlete can complete 20 reps with perfect form, increase load by 1–2.5 lbs.
- Plateau trigger: if the athlete stalls at the same weight for 2–3 sessions,
  vary the exercise (e.g., swap goblet squat for split squat) before increasing load.
- Sessions should feel moderate — the athlete should leave feeling capable of more.
- Include form_notes on every prescribed set emphasizing technique cues.
- Training frequency: 2–3 sessions per week with the SAME exercises to build
  motor learning through repetition.

`)
		case "intermediate":
			b.WriteString(`INTERMEDIATE TIER RULES (Yessis 1×14 Phase):
This athlete has demonstrated foundational movement competency. Transition to
the Yessis 1×14 approach with introduction of barbell movements:
- Reduce rep target from 20 to 14 reps per set, allowing slightly higher intensity.
- Program 12–15 exercises per session, 1 set each (same high-variety, low-set structure).
- May introduce light barbell work (empty bar to ~95 lbs) for main lifts.
- Dumbbells up to 25–30 lbs per hand are appropriate.
- Do NOT use percentage-based loading — use absolute_weight.
- Rep range: 10–14 for compound movements, 12–15 for single-joint/accessory work.
- ONE working set per exercise remains the standard. Add a second set ONLY for
  main compound lifts (squat, bench, deadlift) if the coach explicitly requests it.
- Continue comprehensive joint action coverage — don't drop isolation/accessory
  exercises just because barbell movements are introduced.
- Introduce compound barbell movements (squat, bench, deadlift) with emphasis on
  technique — keep loads light and controlled.
- Progression increments: 2.5–5 lbs for barbell lifts when form is solid.
- Plateau trigger: if stalled for 3 sessions, reduce reps to 10–12 range for that
  exercise and increase load modestly (the 1×14 to 1×10 micro-progression).
- Include form_notes on main lifts with coaching cues.

`)
		case "sport_performance":
			b.WriteString(`SPORT PERFORMANCE TIER RULES:
This athlete has solid technique on compound lifts and is ready for
structured percentage-based programming:
- May use percentage-based loading IF the athlete has training maxes set.
  If no training maxes exist, use absolute_weight and note in reasoning
  that TMs should be established.
- Rep range: 5–10 for main lifts, 8–12 for accessories.
- Maximum 5–6 exercises per session, 3–4 working sets for main lifts.
- Can include power-oriented work (box jumps, med ball throws, light
  Olympic lift variations) with controlled volume (2–3 sets of 3–5 reps).
- Progression increments: 5 lbs for upper body, 5–10 lbs for lower body.
- Program periodization: use linear or simple block periodization.
  Undulating periodization only if training age > 12 months.

`)
		}
	} else {
		// Adult athlete — no tier.
		b.WriteString(`═══════════════════════════════════════════════════════════════
ADULT ATHLETE PROGRAMMING RULES
═══════════════════════════════════════════════════════════════

This athlete does not have a tier — treat as an adult trainee.

LOADING:
- Use percentage-based loading (fraction of training max) when TMs are available.
  Example: "percentage": 0.75 means 75% of TM.
- If no training maxes exist for an exercise, use absolute_weight and note in
  reasoning that TMs should be established.
- For bodyweight or fixed-weight exercises, use absolute_weight.

INTENSITY BY TRAINING GOAL:
- Strength: 80–90% TM, 3–5 reps, 3–5 sets, 3–5 min rest
- Hypertrophy: 65–80% TM, 6–12 reps, 3–4 sets, 60–120 sec rest
- Power: 50–65% TM, 1–5 reps (explosive intent), 2–5 min rest
- Conditioning: <65% TM, 12+ reps, short rest (30–60 sec)

PERIODIZATION — select based on training history:
- Linear (few workouts or new to structured training): volume decreases,
  intensity increases across the program block.
- Undulating (experienced, 50+ workouts): vary intensity and volume across
  the week (e.g., heavy/light/moderate).
- Block (intermediate+): 3–4 week mesocycles progressing from accumulation
  (higher volume, moderate intensity) to intensification (lower volume,
  higher intensity) to realization (peak).
- 5/3/1 and GZCL patterns: if coach directions mention these frameworks,
  follow their specific set/rep/percentage schemes faithfully.

AUTOREGULATION:
- Include RPE guidance in set notes for main lifts (e.g., "Target RPE 7–8").
- AMRAP sets (reps: null) are appropriate for final sets in strength blocks.

`)
	}

	b.WriteString(`═══════════════════════════════════════════════════════════════
CATALOGJSON SCHEMA
═══════════════════════════════════════════════════════════════

{
  "version": "1.0",
  "type": "catalog",
  "exercises": [
    {
      "name": "Exercise Name",
      "tier": "sport_performance",
      "form_notes": "Optional coaching cues",
      "equipment": [{"name": "Barbell", "optional": false}]
    }
  ],
  "programs": [
    {
      "name": "Program Name",
      "description": "Brief program description including periodization approach",
      "num_weeks": 4,
      "num_days": 4,
      "is_loop": false,
      "prescribed_sets": [
        {
          "exercise": "Exercise Name",
          "week": 1,
          "day": 1,
          "set_number": 1,
          "reps": 5,
          "rep_type": "reps",
          "percentage": 0.75,
          "sort_order": 1,
          "notes": "Optional set notes (RPE targets, form cues, tempo, etc.)"
        }
      ],
      "progression_rules": [
        {"exercise": "Exercise Name", "increment": 5.0}
      ]
    }
  ]
}

FIELD DETAILS:
- "reps": null means AMRAP (as many reps as possible).
- "percentage": fraction of training max (0.65 = 65%). Only use when athlete has TMs.
- "absolute_weight": use instead of percentage for bodyweight, fixed-weight, or
  exercises without a training max. Value is in the athlete's preferred unit (lbs/kg).
- "sort_order": controls exercise display order within a day (lower = earlier).
  Main lifts get 1–3, accessories get 4–6, conditioning/finishers get 7+.
- Each set is ONE row — 3×5 means three entries with set_number 1, 2, 3.
- "exercises" array: ONLY include genuinely new exercises. For existing catalog
  exercises, reference them by exact name in prescribed_sets.
- "progression_rules": define weight increment (lbs) when the athlete completes
  all prescribed reps. Use smaller increments for upper body (2.5–5) and
  isolation exercises, larger for lower body compounds (5–10).
`)

	return b.String()
}

func buildUserPrompt(athleteCtx *AthleteContext, req GenerationRequest) (string, error) {
	contextJSON, err := json.MarshalIndent(athleteCtx, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal context: %w", err)
	}

	var b strings.Builder

	b.WriteString("ATHLETE CONTEXT:\n")
	b.Write(contextJSON)
	b.WriteString("\n\n")

	if req.CoachDirections != "" {
		b.WriteString("COACH DIRECTIONS:\n")
		b.WriteString(req.CoachDirections)
		b.WriteString("\n\n")
	}

	if len(req.FocusAreas) > 0 {
		b.WriteString("FOCUS AREAS: ")
		b.WriteString(strings.Join(req.FocusAreas, ", "))
		b.WriteString("\n\n")
	}

	b.WriteString(fmt.Sprintf("REQUEST:\nGenerate \"%s\" — a %d-day/week",
		req.ProgramName, req.NumDays))
	if req.IsLoop {
		b.WriteString(", looping")
	} else {
		b.WriteString(fmt.Sprintf(", %d-week", req.NumWeeks))
	}
	b.WriteString(fmt.Sprintf(" program for %s.\n", athleteCtx.Athlete.Name))

	// Add tier-aware instructions.
	if athleteCtx.Athlete.Tier != nil {
		b.WriteString(fmt.Sprintf("This athlete is at the %s tier. ", *athleteCtx.Athlete.Tier))
		b.WriteString("Follow the tier-specific rules from the system instructions strictly.\n")
	}

	// Note training max availability.
	if len(athleteCtx.Performance.TrainingMaxes) > 0 {
		b.WriteString("The athlete has training maxes set — you may use percentage-based loading where appropriate.\n")
	} else {
		b.WriteString("The athlete has NO training maxes — use absolute_weight for all loading.\n")
	}

	// Note equipment availability.
	if len(athleteCtx.Equipment) == 0 {
		b.WriteString("The athlete has NO equipment configured. Only use exercises marked compatible: true in the catalog (these require no equipment).\n")
	}

	b.WriteString("Consider their performance trends, coach observations, goals, and available equipment.\n\n")
	b.WriteString("IMPORTANT: Keep your <reasoning> section under 300 words. Then output the complete CatalogJSON.\n")
	b.WriteString("Do NOT plan or draft the program in reasoning — go directly to the JSON output.\n")
	b.WriteString("The JSON is the deliverable. Each set is one row, so the output will be large. Prioritize complete JSON.")

	return b.String(), nil
}

// extractResponse separates reasoning and CatalogJSON from the LLM response.
func extractResponse(content string) (catalogJSON []byte, reasoning string) {
	// Extract reasoning from <reasoning>...</reasoning> tags.
	if start := strings.Index(content, "<reasoning>"); start != -1 {
		if end := strings.Index(content, "</reasoning>"); end != -1 {
			reasoning = strings.TrimSpace(content[start+len("<reasoning>") : end])
		}
	}

	// Extract JSON — find the outermost { ... } block.
	catalogJSON = extractJSON(content)

	// If no JSON found and no reasoning tags, the entire response might be
	// unstructured reasoning (model used all tokens on thinking). Capture
	// the response as reasoning so the error message can include it.
	if catalogJSON == nil && reasoning == "" {
		reasoning = strings.TrimSpace(content)
		// Truncate very long reasoning to keep the error message readable.
		if len(reasoning) > 2000 {
			reasoning = reasoning[:2000] + "... [truncated]"
		}
	}

	return catalogJSON, reasoning
}

// extractJSON finds the first complete JSON object in the text.
func extractJSON(s string) []byte {
	// Try to find JSON within code fences first.
	if idx := strings.Index(s, "```json"); idx != -1 {
		start := idx + len("```json")
		if end := strings.Index(s[start:], "```"); end != -1 {
			candidate := strings.TrimSpace(s[start : start+end])
			if json.Valid([]byte(candidate)) {
				return []byte(candidate)
			}
		}
	}

	// Fall back to finding first { ... } block.
	depth := 0
	start := -1
	for i, ch := range s {
		switch ch {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				candidate := s[start : i+1]
				if json.Valid([]byte(candidate)) {
					return []byte(candidate)
				}
				start = -1
			}
		}
	}
	return nil
}
