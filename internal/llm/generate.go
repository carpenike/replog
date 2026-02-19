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
	systemPrompt := buildSystemPrompt()
	userPrompt, err := buildUserPrompt(athleteCtx, req)
	if err != nil {
		return nil, fmt.Errorf("llm: build prompt: %w", err)
	}

	// Step 3: Call the LLM.
	opts := Options{
		Temperature: TemperatureFromSettings(db),
		MaxTokens:   8192,
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

func buildSystemPrompt() string {
	return `You are an expert strength and conditioning programming assistant for youth and adult athletes.

RULES:
1. Output MUST be valid JSON in CatalogJSON format (schema below).
2. Only use exercises from the provided exercise catalog.
3. Only use equipment the athlete has available.
4. Respect rep_type values: "reps", "each_side", "seconds", "distance".
5. Include sort_order for exercise sequencing within each day.
6. Include progression_rules with appropriate increments for compound lifts.
7. Provide your reasoning BEFORE the JSON inside <reasoning>...</reasoning> tags.
8. Tailor the program to THIS specific athlete's metrics, trends, and goals.
9. Consider the athlete's training history, RPE trends, and coach observations.
10. Ensure appropriate volume and intensity for the athlete's experience level.

CATALOGJSON SCHEMA:
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
      "description": "Program description",
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
          "notes": "Optional set notes"
        }
      ],
      "progression_rules": [
        {"exercise": "Exercise Name", "increment": 5.0}
      ]
    }
  ]
}

NOTES ON PRESCRIBED SETS:
- "reps": null means AMRAP (as many reps as possible)
- "percentage": fraction of training max (0.65 = 65%)
- "absolute_weight": use instead of percentage for bodyweight or fixed-weight exercises
- "sort_order": controls exercise display order within a day (lower = earlier)
- Each set is one row — 3x5 = three separate set entries with set_number 1, 2, 3

Only include exercises in the "exercises" array if they are NEW exercises not already in the catalog.
For existing catalog exercises, just reference them by name in prescribed_sets.`
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
	b.WriteString("Consider their performance trends, coach observations, goals, and available equipment.\n")
	b.WriteString("Provide your reasoning in <reasoning> tags, then output the CatalogJSON.")

	return b.String(), nil
}

// extractResponse separates reasoning and CatalogJSON from the LLM response.
func extractResponse(content string) (catalogJSON []byte, reasoning string) {
	// Extract reasoning.
	if start := strings.Index(content, "<reasoning>"); start != -1 {
		if end := strings.Index(content, "</reasoning>"); end != -1 {
			reasoning = strings.TrimSpace(content[start+len("<reasoning>") : end])
		}
	}

	// Extract JSON — find the outermost { ... } block.
	catalogJSON = extractJSON(content)
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
