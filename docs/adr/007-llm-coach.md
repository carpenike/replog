# ADR 007 — LLM-Assisted Program Generation

> Status: **Proposed** · Date: 2026-02-19

## Context

RepLog's core principle is *"the app is a logbook — a human coach makes all
progression decisions."*  That principle doesn't change.  What an LLM can do is
act as a **research assistant** that drafts program proposals based on live
athlete data, which the coach then reviews, edits, and approves before anything
touches the database.

### The Specific Use Case

We have three months of sport performance programming (Month 1 → 2 → 3).  A
coach needs to build Months 4, 5, and 6.  Today that requires:

1. Open each athlete's workout history and review performance trends.
2. Read coach notes, workout reviews, and AMRAP results.
3. Check what equipment each athlete has available.
4. Remember the progression philosophy (Yessis → sport performance pipeline).
5. Manually author prescribed sets in a new program template.

An LLM can do steps 1–4 automatically and produce a draft for step 5 — in
**CatalogJSON format** that the existing import pipeline already understands.

### Per-Athlete Tailored Programs

The critical insight is that identical athletes don't exist.  Two siblings who
both finish Sport Performance Month 3 need **different** Month 4 programs
because they have different:

- **Performance trajectories**: one plateaued on power cleans (RPE 8+), the
  other is still progressing (RPE 6).
- **Injury flags**: coach noted knee issues on one, the other is healthy.
- **Goals**: one is prepping for football combine, the other for volleyball.
- **Equipment access**: one trains at a school gym, the other at home with
  dumbbells and bands.
- **Adherence patterns**: one missed sessions due to illness, the other has
  100% attendance.
- **Body composition**: one is gaining weight rapidly (growth spurt), the
  other is stable.

The context assembly layer gathers **all of this per-athlete data** and the LLM
produces a program tailored to that specific athlete.  Same coach, same
"Generate" button, completely different outputs.  The more data in the system
(more workouts, more notes, more TM history), the richer the context becomes —
by Month 6 the LLM has 6 months of longitudinal data per athlete to work with.

### Data Signals Available in RepLog Today

The system already captures everything an LLM needs for informed program generation:

| Signal | Source Table(s) | What It Tells the LLM |
|--------|----------------|----------------------|
| **Workout performance** | `workout_sets` (reps, weight, RPE, rep_type) | Actual vs prescribed load/volume, RPE trends |
| **Prescriptions** | `prescribed_sets` + `athlete_programs` | What was *supposed* to happen (baseline) |
| **AMRAP results** | `workout_sets` where `prescribed_sets.reps IS NULL` | Top-set capacity — key strength indicator |
| **Training max history** | `training_maxes` (weight, effective_date) | TM trajectory over time |
| **Coach notes** | `athlete_notes` (content, is_private, pinned) | Qualitative observations (form issues, motivation, injury flags) |
| **Workout reviews** | `workout_reviews` (status, notes) | Per-session coach feedback |
| **Goals** | `athletes.goal` + `athlete_programs.goal` + `goal_history` | Long-term and cycle-specific objectives |
| **Tier history** | `tier_history` | Progression through foundational → intermediate → sport_performance |
| **Body weight** | `body_weights` | Growth/weight trends (important for youth athletes) |
| **Equipment inventory** | `athlete_equipment` + `exercise_equipment` | What the athlete can actually do |
| **Exercise catalog** | `exercises` (tier, form_notes, equipment deps) | Full movement library with metadata |
| **Existing templates** | `program_templates` + `prescribed_sets` + `progression_rules` | Prior programs as few-shot examples |
| **Adherence** | `workout_sets` vs `prescribed_sets` (computed) | Did the athlete actually complete the prescribed work? |
| **Streaks** | Computed from `workouts` | Consistency / attendance patterns |

## Decision

### Architecture: Three-Layer Pipeline

```
┌─────────────────────────────────────────────────┐
│  1. CONTEXT ASSEMBLY                            │
│     Gather per-athlete data → structured prompt │
│     (all server-side, no LLM call yet)          │
└──────────────────┬──────────────────────────────┘
                   │ JSON context document
                   ▼
┌─────────────────────────────────────────────────┐
│  2. LLM GENERATION                              │
│     System prompt + context → CatalogJSON       │
│     (single API call to configurable provider)  │
└──────────────────┬──────────────────────────────┘
                   │ CatalogJSON program draft
                   ▼
┌─────────────────────────────────────────────────┐
│  3. COACH REVIEW                                │
│     Preview → edit → approve/reject             │
│     (reuses existing import preview UI)         │
└─────────────────────────────────────────────────┘
```

### Application Settings (`app_settings` table)

RepLog currently uses environment variables exclusively for configuration
(`REPLOG_DB_PATH`, `REPLOG_ADDR`, `REPLOG_ADMIN_*`, etc.).  This works for
infra-level settings that are set once at deploy time, but LLM configuration
needs to be manageable post-deployment from the admin UI — an admin shouldn't
have to SSH into the server and restart the process to change an API key or
switch from OpenAI to Ollama.

#### New table: `app_settings`

```sql
CREATE TABLE app_settings (
    key   TEXT PRIMARY KEY NOT NULL,
    value TEXT NOT NULL
);
```

A simple key-value store.  No timestamps, no audit trail — this is
configuration, not user data.

#### Resolution order: env var overrides DB

```
ENV VAR (if set) → app_settings row (if exists) → built-in default
```

Environment variables **always win**.  This preserves the existing deployment
pattern (Nix flake, Docker, systemd) where operators expect env vars to be
authoritative.  But if no env var is set, the app falls back to the database,
which can be configured from the admin UI without restarting the server.

```go
// GetSetting returns a configuration value using the resolution chain:
// env var → app_settings → default.
func GetSetting(db *sql.DB, key, envVar, defaultVal string) string {
    // 1. Environment variable always wins.
    if v := os.Getenv(envVar); v != "" {
        return v
    }
    // 2. Database setting.
    var v string
    err := db.QueryRow(
        `SELECT value FROM app_settings WHERE key = ?`, key,
    ).Scan(&v)
    if err == nil {
        return v
    }
    // 3. Built-in default.
    return defaultVal
}
```

#### Settings registry

| Setting Key | Env Var Override | Default | UI Field Type |
|-------------|----------------|---------|---------------|
| `llm.provider` | `REPLOG_LLM_PROVIDER` | `""` (disabled) | Dropdown: openai, anthropic, ollama |
| `llm.model` | `REPLOG_LLM_MODEL` | `""` | Text input |
| `llm.api_key` | `REPLOG_LLM_API_KEY` | `""` | Password input (masked) |
| `llm.base_url` | `REPLOG_LLM_BASE_URL` | `""` | Text input (for Ollama, proxies) |
| `llm.temperature` | `REPLOG_LLM_TEMPERATURE` | `0.7` | Number input (0.0–2.0) |
| `llm.max_tokens` | `REPLOG_LLM_MAX_TOKENS` | `4096` | Number input |
| `llm.system_prompt_override` | — | `""` | Textarea (optional; replaces default system prompt) |

Settings with an env var override show a "(set via environment)" badge in the
admin UI so admins know the value can't be changed from the web.

#### Why not just env vars?

- **Post-deploy configuration**: Admin can set up LLM access from the browser
  after deploying the binary.  No SSH, no restart, no redeploy.
- **API key rotation**: Change keys from the UI immediately.
- **A/B testing providers**: Quickly switch between OpenAI and Anthropic to
  compare output quality without touching infrastructure.
- **Ollama discovery**: If Ollama is running locally, the admin can point
  `llm.base_url` at it from the settings page.

#### Why not *only* DB settings?

- **Existing pattern**: Every other RepLog setting uses env vars.  Operators
  who deploy via Nix, Docker, or systemd expect env vars to work.
- **Secrets management**: Many deployment platforms inject secrets as env vars
  (Kubernetes secrets, systemd `EnvironmentFile`, Nix `sops`).  The env var
  override respects this.
- **Bootstrap safety**: The app must start and run without any DB settings
  configured — env vars handle initial setup.

#### Sensitive value storage

API keys stored in `app_settings` are encrypted at rest using AES-256-GCM.
The encryption key is derived from `REPLOG_SECRET_KEY` (a new required env var
for installations using DB-stored secrets).  If `REPLOG_SECRET_KEY` is not set,
DB-stored API keys are rejected and the admin UI shows a warning.

```go
// Sensitive settings are stored as: "enc:" + base64(nonce + ciphertext)
// Non-sensitive settings are stored as plain text.
var sensitiveKeys = map[string]bool{
    "llm.api_key": true,
}
```

#### Admin UI for settings

```
/admin/settings          GET  — show all configurable settings
/admin/settings          POST — update settings (admin-only)
```

The settings page is grouped by category (LLM, future categories) with:
- Current value display (API keys masked as `sk-...xxxx`)
- "(set via environment)" badge for env-overridden values (read-only in UI)
- "Test Connection" button for LLM provider (sends a minimal prompt, confirms response)
- Save button per section

This is admin-only — coaches and athletes never see the settings page.

#### Future-proofing

The `app_settings` table is not LLM-specific.  It can absorb other settings
that currently require env vars:

| Potential Future Settings | Current Env Var |
|--------------------------|-----------------|
| Default weight unit | — (hardcoded `lbs`) |
| Session lifetime | — (hardcoded 30 days) |
| Max upload size | — (hardcoded) |
| SMTP settings for email notifications | — (not implemented) |
| Backup schedule | — (not implemented) |

These are out of scope for this ADR but the table design accommodates them.

### Layer 1: Context Assembly (`internal/llm/context.go`)

A new `BuildAthleteContext()` function gathers all relevant data for **one
specific athlete** into a structured JSON document.  This is the LLM's
per-athlete "briefing packet."

```go
// AthleteContext is the structured data package sent to the LLM.
// Every field is specific to one athlete — the same function called
// for two different athletes produces completely different contexts.
type AthleteContext struct {
    Athlete        AthleteProfile       `json:"athlete"`
    Equipment      []string             `json:"available_equipment"`
    CurrentProgram *ProgramSummary      `json:"current_program"`
    PriorPrograms  []ProgramSummary     `json:"prior_programs"`
    Performance    PerformanceData      `json:"performance"`
    CoachNotes     []NoteEntry          `json:"coach_notes"`
    Goals          GoalContext          `json:"goals"`
    ExerciseCatalog []ExerciseEntry     `json:"exercise_catalog"`
}

type AthleteProfile struct {
    Name   string  `json:"name"`
    Tier   *string `json:"tier"`
    Goal   *string `json:"goal"`
    Notes  *string `json:"notes"`
    // Computed summaries
    Age            *string `json:"age,omitempty"`          // if derivable
    TrainingMonths int     `json:"training_months"`        // months since first workout
    TotalWorkouts  int     `json:"total_workouts"`
}

type PerformanceData struct {
    // Per-exercise: actual vs prescribed, RPE trends, TM history
    ExerciseStats   []ExercisePerformance `json:"exercise_stats"`
    // Adherence: % of prescribed sets completed per week
    WeeklyAdherence []AdherenceWeek       `json:"weekly_adherence"`
    // Body weight trend
    BodyWeights     []BodyWeightEntry     `json:"body_weights"`
    // AMRAP bests from recent cycles
    AMRAPResults    []AMRAPEntry          `json:"amrap_results"`
}

type ExercisePerformance struct {
    Exercise       string    `json:"exercise"`
    // Training max trajectory
    TMHistory      []TMEntry `json:"tm_history"`
    // Recent performance (last N sessions)
    RecentSets     []SetSummary `json:"recent_sets"`
    // Computed trends
    AvgRPE         *float64  `json:"avg_rpe"`
    RPETrend       string    `json:"rpe_trend"`  // "rising", "stable", "falling"
    VolumePerWeek  float64   `json:"volume_per_week"`
    VolumeTrend    string    `json:"volume_trend"`
    CompletionRate float64   `json:"completion_rate"` // % of prescribed sets completed
}

type GoalContext struct {
    LongTerm  string   `json:"long_term"`   // athletes.goal
    CycleGoal string   `json:"cycle_goal"`  // athlete_programs.goal
    History   []string `json:"history"`     // goal_history entries
}
```

**Key design choices:**

- Context is assembled server-side as **pure Go queries** — no LLM involved yet.
- The output is a JSON document that can be **inspected, cached, and audited** before
  any LLM call.  A coach can view "what data would the LLM see?" in the UI.
- Equipment filtering happens here: only exercises the athlete has equipment for
  are included in the catalog passed to the LLM.
- Private notes (`is_private = 1`) are **included** because only coaches can
  trigger generation.  The LLM output is also coach-only until approved.
- **Computed trends** (RPE trend, volume trend, completion rate) are calculated
  server-side, not left for the LLM to derive.  This makes the context more
  compact and the LLM's job easier.

### Layer 2: LLM Generation (`internal/llm/generate.go`)

A single function takes the athlete context + a generation request and returns
a CatalogJSON program template.  Provider/model configuration comes from
`app_settings` (with env var override).

```go
type GenerationRequest struct {
    AthleteID    int64
    // What to generate
    ProgramName  string // e.g., "Sport Performance Month 4"
    NumWeeks     int    // 1 (loop) or N (fixed block)
    NumDays      int    // training days per week
    IsLoop       bool
    // Constraints
    MaxExercisesPerDay int
    FocusAreas         []string // e.g., ["power", "conditioning"]
    CoachDirections    string   // free-text: "start introducing hang cleans"
}

type GenerationResult struct {
    CatalogJSON  []byte          // valid CatalogJSON ready for import
    Reasoning    string          // LLM's explanation of choices
    TokensUsed   int             // for cost tracking
    Duration     time.Duration
    Model        string          // which model actually responded
}
```

**System prompt structure:**

```
You are a strength & conditioning programming assistant for youth athletes.

RULES:
- Output MUST be valid CatalogJSON (schema provided below).
- Only use exercises from the provided catalog.
- Only use equipment the athlete has available.
- Respect rep_type (reps, each_side, seconds, distance).
- Include sort_order for exercise sequencing.
- Include progression_rules for compound lifts.
- Explain your reasoning in <reasoning> tags before the JSON.
- Tailor the program to THIS athlete's specific metrics, trends, and goals.

ATHLETE CONTEXT:
{assembled JSON from Layer 1}

PRIOR PROGRAMS (as few-shot examples):
{recent program templates in CatalogJSON format}

COACH DIRECTIONS:
{free-text from the generation form}

REQUEST:
Generate "{program_name}" — a {num_days}-day/week program for {athlete_name}.
Consider their performance trends, coach observations, goals, and available equipment.
```

**Provider abstraction:**

```go
// Provider is the interface for LLM backends.
type Provider interface {
    Generate(ctx context.Context, systemPrompt, userPrompt string, opts Options) (*Response, error)
    // Ping validates connectivity and credentials (for "Test Connection" button).
    Ping(ctx context.Context) error
}

// Implementations:
// - OpenAI (GPT-4o, o1, and any OpenAI-compatible API)
// - Anthropic (Claude)
// - Ollama (local models for self-hosted)
// - Mock (for testing — returns canned CatalogJSON)
```

Provider configuration is resolved at call time from `app_settings`:

```go
func NewProviderFromSettings(db *sql.DB) (Provider, error) {
    provider := GetSetting(db, "llm.provider", "REPLOG_LLM_PROVIDER", "")
    if provider == "" {
        return nil, ErrLLMNotConfigured
    }
    model   := GetSetting(db, "llm.model",    "REPLOG_LLM_MODEL",    "")
    apiKey  := GetSetting(db, "llm.api_key",   "REPLOG_LLM_API_KEY",  "")
    baseURL := GetSetting(db, "llm.base_url",  "REPLOG_LLM_BASE_URL", "")

    switch provider {
    case "openai":
        return NewOpenAIProvider(apiKey, model, baseURL)
    case "anthropic":
        return NewAnthropicProvider(apiKey, model)
    case "ollama":
        return NewOllamaProvider(baseURL, model)
    default:
        return nil, fmt.Errorf("unknown LLM provider: %q", provider)
    }
}
```

### Layer 3: Coach Review (reuses existing import UI)

The LLM output is **CatalogJSON** — the exact same format the import system
already processes.  The review flow is:

1. LLM returns CatalogJSON with a new program template + any new exercises.
2. Feed it through `importers.ParseCatalogJSON()` (already exists).
3. Run `importers.BuildProgramMappings()` + `BuildExerciseMappings()` (already exists).
4. Show the import preview UI (already exists) — coach sees:
   - New exercises to be created (if the LLM suggested novel movements)
   - Program template with all prescribed sets
   - Progression rules
5. Coach can **approve**, **edit mappings** (swap exercises), or **reject**.
6. On approve: `models.ExecuteCatalogImport()` (already exists) writes to DB.

**The entire import pipeline is reused unchanged.**  The only difference is the
source: instead of a file upload, the CatalogJSON comes from the LLM.

```
                    ┌──────────────┐
File Upload ───────►│              │
                    │  ParseCatalog│
LLM Output ───────►│  JSON()      ├──► BuildMappings() ──► Preview UI ──► ExecuteCatalogImport()
                    │              │
                    └──────────────┘
```

### UI Flow

#### Coach triggers generation from the athlete's program page:

```
/athletes/{id}/programs/generate          GET  — show generation form
/athletes/{id}/programs/generate          POST — assemble context, call LLM, redirect to preview
/athletes/{id}/programs/generate/preview  GET  — show import preview (reused template)
/athletes/{id}/programs/generate/execute  POST — approve and import
```

The generation form includes:
- Program name (pre-filled: "Sport Performance Month 4" — auto-incremented from current)
- Days per week (pre-filled from current program)
- Focus areas (checkboxes: power, conditioning, hypertrophy, mobility, etc.)
- Coach directions (free-text: "Start introducing hang cleans, pull back on carries")
- "What data will the LLM see?" expandable section (shows assembled context JSON)
- Generate button (with loading indicator — LLM calls take 10-30s)

The LLM's **reasoning** is shown alongside the preview so the coach
understands *why* the LLM made specific choices:

> *"Increased power clean volume from 3×3 to 4×3 based on Tommy's RPE
> averaging 6.5 in Month 3. Replaced lateral lunges with pistol squats —
> Tommy completed all prescribed lateral lunge sets with RPE < 5, indicating
> readiness for progression. Maintained suitcase carries at 30s based on
> coach note about grip fatigue. Did NOT include depth jumps despite box jump
> progression because coach note flagged left knee concerns."*

#### Admin configures LLM settings:

```
/admin/settings          GET  — show settings page (grouped by category)
/admin/settings          POST — save settings
/admin/settings/test-llm POST — test LLM connection (returns success/error)
```

### Data Flow for Per-Athlete Tailored Generation

```
1. Coach opens Tommy → Programs → "Generate Next Program"

2. Context Assembly gathers TOMMY'S data:
   - Athlete: "Tommy", tier: sport_performance, goal: "prepare for football"
   - Training history: 48 workouts over 3 months, 92% adherence
   - Equipment: [Barbell, Squat Rack, Dumbbells, Kettlebell, Plyo Box, ...]
   - Current program: Sport Performance Month 3 (assigned 2026-01-15)
   - Prior programs: Month 1 (completed), Month 2 (completed)
   - Performance:
     - Power Clean: TM 95→105→115 lbs, RPE 7→7→8, completion 100%
     - Box Jump: RPE 5 (mastered), coach noted "20" → 24" progression"
     - Lateral Lunge: RPE 4, completion 100% — ready to advance
     - Suitcase Carry: RPE 8 — coach noted "grip fatigue"
   - Coach notes: "Ready to increase explosive work", "Watch left knee on lunges"
   - Body weight: 142→150 lbs over 3 months (growth spurt)
   - AMRAP results: N/A (sport perf doesn't use AMRAPs)

3. Coach types direction: "Start introducing hang cleans"

4. LLM receives Tommy's context + Month 1/2/3 templates as examples

5. LLM outputs CatalogJSON for "Sport Performance Month 4 — Tommy":
   - 4 days/week, 1-week loop
   - Hang clean introduced (coach requested), building on power clean base
   - Pistol squat replaces lateral lunge (mastery signals)
   - Box jump stays at 24" (knee caution from coach note)
   - Carries maintained at 30s (grip fatigue note)
   - Reasoning explains each choice with specific data references

6. Coach reviews:
   - "Good, but bump box jumps to 26" — his knee is fine now"
   - Edits prescribed set → approves
   - ExecuteCatalogImport writes the template

7. Coach assigns "Sport Performance Month 4 — Tommy" to Tommy

--- Meanwhile, for Tommy's sister: ---

8. Coach opens Sarah → Programs → "Generate Next Program"

9. Context Assembly gathers SARAH'S data:
   - Different TM trajectory, different RPE trends, different equipment
   - Goal: "volleyball preseason"
   - Equipment: home gym only (dumbbells, KB, resistance bands)
   - 100% adherence, stable body weight
   - No injury flags in coach notes

10. LLM outputs a COMPLETELY DIFFERENT "Sport Performance Month 4 — Sarah":
    - Dumbbell variants instead of barbell (equipment constraint)
    - Lateral plyometrics emphasis (volleyball-specific)
    - Higher volume (100% adherence + lower RPE = capacity for more)
    - No power clean at all (no barbell)
```

## Implementation Plan

### Phase 0: App Settings Infrastructure

Build the `app_settings` table and admin settings UI.  This is independent of
the LLM feature and useful across the application.

**Schema (added to `0001_initial_schema.sql`):**
- `CREATE TABLE app_settings` — key-value store for runtime configuration

**New files:**
- `internal/models/app_settings.go` — `GetSetting()`, `SetSetting()`, `ListSettings()`, encryption helpers
- `internal/models/app_settings_test.go` — tests for resolution chain (env → DB → default)
- `internal/handlers/settings.go` — admin settings page handler
- Settings page template

**Modified files:**
- `cmd/replog/main.go` — add settings routes (admin-only)

### Phase 1: Context Assembly (no LLM dependency)

Build `internal/llm/context.go` with `BuildAthleteContext()`.  This is useful
even without an LLM — coaches can export the context JSON and paste it into
ChatGPT/Claude manually.  Zero external dependencies.

**New files:**
- `internal/llm/context.go` — context assembly with per-athlete queries
- `internal/llm/context_test.go` — tests against test DB

**New endpoint (optional):**
- `GET /athletes/{id}/context.json` — download the assembled context for manual
  use in external LLM interfaces.  Useful as a standalone feature even if the
  built-in LLM integration is never configured.

### Phase 2: Provider Abstraction + Generation

Build the provider interface and implementations.  Provider/model selection
reads from `app_settings` (configured in Phase 0).

**New files:**
- `internal/llm/provider.go` — interface + factory (`NewProviderFromSettings`)
- `internal/llm/openai.go` — OpenAI/compatible API (covers GPT-4o, o1, and
  any OpenAI-compatible endpoint like Azure OpenAI or local vLLM)
- `internal/llm/anthropic.go` — Anthropic Claude API
- `internal/llm/ollama.go` — local Ollama implementation
- `internal/llm/mock.go` — mock provider for testing
- `internal/llm/generate.go` — orchestration (context + prompt + parse)
- `internal/llm/generate_test.go` — tests with mock provider
- `internal/llm/prompt.go` — system prompt template + CatalogJSON schema docs

### Phase 3: UI Integration

Wire up handlers and templates.  The heavy lifting (preview + import execution)
is already built — we just add a generation form and pipe LLM output into the
existing import flow.

**New files:**
- `internal/handlers/generate.go` — HTTP handlers for generation flow
- Templates for the generation form, context preview, and reasoning display

**Modified files:**
- `cmd/replog/main.go` — add generation routes
- Athlete program page template — add "Generate Next Program" button
  (only visible when `llm.provider` is configured in app settings)

### Phase 4: Feedback Loop (future)

After a generated program is used for a full cycle, capture:
- Did the coach modify the LLM's suggestions before approving?
- How did the athlete perform on the generated program vs prior ones?
- What exercises did the coach swap (reveals LLM blind spots)?
- This data feeds back into future context assembly (meta-learning).

**New table (future):**
```sql
CREATE TABLE generation_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    template_id INTEGER REFERENCES program_templates(id) ON DELETE SET NULL,
    provider    TEXT NOT NULL,
    model       TEXT NOT NULL,
    context_json TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    response    TEXT NOT NULL,
    reasoning   TEXT,
    tokens_used INTEGER,
    duration_ms INTEGER,
    coach_edits TEXT,        -- JSON diff of what the coach changed
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

This is opt-in analytics, not automated retraining.

## Alternatives Considered

### 1. Fully automated progression (rejected)

Auto-apply LLM suggestions without coach review.  Violates the core principle.
The coach **must** be in the loop — especially for youth athletes where injury
risk from inappropriate loading is a real concern.

### 2. Embedded model (rejected for now)

Run a small fine-tuned model directly in the Go binary (via GGML/llama.cpp
bindings).  Appealing for self-hosted deployments, but:
- Dramatically increases binary size (2-7 GB for a useful model)
- Quality of small models for programming is insufficient today
- Ollama support achieves the same goal with better model flexibility

### 3. External webhook / plugin system (deferred)

Let users configure arbitrary external endpoints for program generation.
More flexible but harder to get right.  Start with built-in providers,
add webhooks later if needed.

### 4. JSON-only output (no reasoning) (rejected)

Having the LLM explain its choices is critical for coach trust and
educational value.  The reasoning also helps the coach catch errors
("it said it increased volume because RPE was low — but I know that
athlete was sandbagging RPE scores").

### 5. Env-var-only configuration (rejected)

Requiring env vars for all LLM settings means admins must SSH into the server,
edit environment files, and restart the process to change an API key or switch
providers.  This is friction that discourages experimentation.  The hybrid
approach (env var overrides DB) gives operators full control while making
day-to-day configuration accessible from the browser.

### 6. Separate config file (rejected)

A YAML/TOML config file would be another thing to manage alongside env vars.
The `app_settings` table is simpler — it's already in the SQLite database
that's being backed up, and the admin UI provides a better editing experience
than a text file for non-technical users.

## Cost Considerations

- **Context size**: A fully assembled athlete context is ~2-4K tokens.
  Prior program templates add ~3-6K tokens.  System prompt ~1K.
  Total input: ~8-12K tokens per generation.
- **Output size**: A 4-day program template in CatalogJSON is ~2-4K tokens.
  Reasoning adds ~500-1K tokens.  Total output: ~3-5K tokens.
- **Per-generation cost** (at Feb 2026 pricing):
  - GPT-4o: ~$0.04-0.08 per generation
  - Claude Sonnet: ~$0.03-0.06 per generation
  - Ollama (local): $0 (hardware cost only)
- **Frequency**: A coach generates maybe 1-3 programs per month per athlete.
  For a family of 4 athletes: ~$0.50-1.00/month at API pricing.
- **Ollama** is the recommended default for self-hosted deployments — no API
  costs, full data privacy, runs on modest hardware for this use case.

## Security & Privacy

- LLM API calls send athlete performance data to external APIs (OpenAI, Anthropic).
  For families this is acceptable; for gym/team deployments, Ollama is preferred.
- API keys in `app_settings` are encrypted at rest (AES-256-GCM, keyed from
  `REPLOG_SECRET_KEY` env var).  API keys set via env var are not stored in DB.
- LLM responses are treated as **untrusted input**: the CatalogJSON is validated
  through the same parsing pipeline as file uploads.
- Generation requests are **coach-only** (middleware auth check).
- Settings management is **admin-only** (middleware auth check).
- Private notes are included in context (coach-only feature).
- No athlete data is persisted outside RepLog — LLM calls are stateless.
  (Phase 4's `generation_history` stores context locally for audit, never externally.)

## Consequences

### Positive

- Coaches get AI-drafted programs grounded in **individual athlete data** —
  not generic templates, but tailored programs reflecting each athlete's
  specific performance, equipment, goals, and coach observations.
- `app_settings` table provides a clean, extensible configuration layer
  that benefits the entire app, not just the LLM feature.
- Admins can configure, test, and switch LLM providers from the browser
  without restarting the server or touching deployment infrastructure.
- Env var overrides preserve full compatibility with existing deployment
  patterns (Nix, Docker, systemd, Kubernetes secrets).
- Zero new tables for the LLM feature itself — programs flow through the
  existing import pipeline.  Only `app_settings` is new (Phase 0), and
  `generation_history` is deferred to Phase 4.
- CatalogJSON as the interchange format means the LLM integration is
  **decoupled** from the core app.  If the LLM feature is never used,
  zero code paths are affected.
- Phase 1 (context assembly) is useful standalone — coaches can download
  the athlete context JSON and paste it into any LLM chat interface.
- Ollama support means fully self-hosted, zero-cost, private operation.
- The richer the logbook data, the better the generated programs — this
  creates a positive feedback loop that incentivizes thorough logging.

### Negative

- External API dependency (mitigated by Ollama fallback and "not configured"
  graceful degradation — the Generate button simply doesn't appear).
- LLM output quality varies — coaches must always review.
- Generation latency (10-30s) requires async UX (loading states).
- Prompt engineering maintenance — as the CatalogJSON schema evolves,
  the system prompt must be updated to match.
- API key encryption requires `REPLOG_SECRET_KEY` — one more env var for
  installations that want DB-stored secrets.

### Neutral

- The feature is entirely additive — no changes to existing functionality.
- The `internal/llm/` package is self-contained with a clean boundary.
- If a coach never clicks "Generate," no LLM code runs.
- `app_settings` is a general-purpose table that can absorb future
  configuration needs beyond LLM settings.
