# ADR 016 — First-Class Training Methodology

> Status: **Accepted** · Date: 2026-05-27
>
> Graduated to **Accepted** with the Phase-1 implementation (basic-memory
> `handoff/HOF-003`, GitHub issue #19).

## Context

The AI coach (ADR 007, ADR 015) drafts per-athlete program proposals. A review
of how those drafts are built (2026-05-27) found that **"methodology" is never
modeled — it is emergent from a tangle**:

- Youth methodology (Dr. Michael Yessis's 1×20 / 1×15 GPP system) is **hardcoded**
  in `buildSystemPrompt` as Go string literals, selected by the athlete's `tier`.
- Adults get **one generic block** plus a single line — "if the coach mentions
  5/3/1 or GZCL, follow it." 5/3/1, 5×5, Greyskull, and GZCLP have **no stored
  definition**; their prescription comes from the LLM's prior knowledge plus
  whichever seeded reference programs land in context.
- The only stored prompt knob is `llm.system_prompt_override` — **global,
  all-or-nothing**.
- The generate page presents a **flat, unfiltered list** of every program
  template as "references," with no concept of which methodology is in play.
- The exercise catalog handed to the LLM is the **full library**
  (`buildExerciseCatalog` → `ListExercises(db, "")`, equipment-filtered only).
  For youth athletes the methodology boundary is therefore **prompt-instructed,
  not enforced** — nothing structurally stops a foundational-tier draft from
  selecting a barbell clean or a sled push.

Multiple methodologies already coexist by design (Yessis tiers, adult barbell
strength, the Sarge circuits). They simply are not first-class.

The load-bearing principle is unchanged: **RepLog is a logbook; a human coach
makes every progression decision.** Methodology is a *proposal driver*, never an
automated progression (ADR 007, `docs/COACH_VOICE.md`).

## Decision

Introduce a first-class **Methodology**: a stored, coach-selectable definition
that drives a generation.

1. **Data model — dedicated `methodologies` table** (plus link tables). Not
   `app_settings`-backed and not a hybrid; the table is the explicit home.
2. **Selection — coach-explicit at generation time**, for adult programs (5/3/1)
   and youth programs (Yessis) alike. The coach picks the methodology; it is not
   silently inferred.
3. **Scoping — a methodology declares an allow-list of exercises and equipment.**
   The generation catalog is filtered to that allow-list *before the LLM sees it*.
   This is structural enforcement on top of the prompt rules: a Yessis-foundational
   generation cannot reach a barbell clean or a sled because they are out of scope.
4. **Editability vs. safety.** Methodology copy (the philosophy/prescription
   text) may be editable data. **Youth-safety floors stay code-enforced
   invariants**, keyed on `tier`, regardless of methodology copy — no barbell or
   Olympic lifts at foundational, no 1RM testing or maximal singles for youth,
   etc. Editable methodology text must not become a back door around the
   conservative defaults. Concretely: only the **methodology-specific per-tier
   block** becomes the seeded, editable `definition`; the **shared youth preamble
   (general youth rules) and the safety floors stay in code**, emitted for every
   youth athlete regardless of which methodology is selected.
5. **Methodology stays a proposal driver.** The coach reviews, may edit, and
   approves every generated program (ADR 007). Generation never auto-assigns or
   auto-progresses.

### Three kinds of "methodology-ish" thing — only one is generatable

The youth-methodology research (2026-05-27) made a structural point clear:

- **Prescriptive methodologies** have a concrete recipe (set/rep scheme,
  exercise-selection rules, progression) and **populate the `methodologies`
  table** — they are what a generation instantiates.
- **Frameworks** (Long-Term Athletic Development; Lloyd & Oliver's Youth Physical
  Development; the Athletic Skills Model) are planning/philosophy models with **no
  sets×reps recipe**. They are **NOT** seeded as generatable methodologies. Their
  value is maturation-stage quality-emphasis and movement variety, which in
  RepLog is best expressed by **grounding the `tier` definitions and youth-safety
  doctrine in them** — the `tier` already is the maturation-stage proxy, and the
  generator already keys on it.
- **Movement patterns** (Dan John: push, pull, hinge, squat, loaded carry, plus
  ground/locomotion) are a **classification taxonomy**, modeled as **exercise
  tags**. The same tags power the methodology allow-list scoping and the
  joint-action / movement-coverage checks Yessis already requires.

### Seed set

- **Youth (prescriptive):** one Yessis-family methodology per youth tier so the
  tier→methodology map is total — Yessis 1×20 (foundational), Yessis 1×15
  (intermediate), and **Yessis Sport-Performance** (sport_performance — the
  monthly clean-based blocks; the existing per-tier prompt block + the seeded
  Sport Performance Month programs are its definition + exemplars) — plus
  **Integrative Youth GPP**, grounded in Faigenbaum & Myer's Integrative
  Neuromuscular Training (1–3 sets × 6–15 reps, multifaceted: strength +
  plyometrics + balance + agility + coordination, technique before load), as a
  complementary youth option.
- **Adult (prescriptive):** 5/3/1, 5/3/1 BBB, Greyskull LP, GZCLP, 5×5, Galpin
  3-to-5, Sarge circuit — already seeded as programs or added as an explicit
  proposal framework; promoted to methodologies.
- **Frameworks (doctrine, not methodologies):** LTAD, YPD, ASM — cited as the
  evidence base behind the tier definitions and youth-safety floors.
- **Movement patterns (taxonomy):** push / pull / hinge / squat / loaded-carry /
  ground — exercise tags.

### Schema sketch (additive; ADR 002 pre-production policy)

```sql
CREATE TABLE methodologies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    key             TEXT NOT NULL UNIQUE,        -- e.g. 'yessis-1x20', '531', 'int-youth-gpp'
    name            TEXT NOT NULL,
    audience        TEXT,                        -- 'youth' | 'adult' | NULL
    applicable_tiers TEXT,                       -- optional CSV/JSON of tiers it fits
    philosophy      TEXT,                        -- short human description
    definition      TEXT NOT NULL,               -- the prompt block (editable copy)
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- exemplar reference programs for a methodology
CREATE TABLE methodology_reference_programs (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    template_id    INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    PRIMARY KEY (methodology_id, template_id)
);

-- allow-list scoping
CREATE TABLE methodology_allowed_equipment (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    equipment_id   INTEGER NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    PRIMARY KEY (methodology_id, equipment_id)
);

-- movement-pattern tags on exercises (Dan John taxonomy)
CREATE TABLE exercise_movement_patterns (
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    pattern     TEXT NOT NULL,   -- 'push'|'pull'|'hinge'|'squat'|'carry'|'ground'
    PRIMARY KEY (exercise_id, pattern)
);

-- a methodology's allowed exercise scope, by pattern …
CREATE TABLE methodology_allowed_patterns (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    pattern        TEXT NOT NULL,
    PRIMARY KEY (methodology_id, pattern)
);

-- … and an explicit exercise allow-list override (e.g. 5/3/1 barbell mains,
-- the Sarge bespoke list). Ship BOTH surfaces in Phase 1 so Phase 2 needs no
-- second migration; allow-by-pattern + override-by-list semantics are settled
-- at Phase-2 prompt-composition time.
CREATE TABLE methodology_allowed_exercises (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    exercise_id    INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    PRIMARY KEY (methodology_id, exercise_id)
);
```

Both allow-list surfaces ship in Phase 1 (`methodology_allowed_patterns` +
`methodology_allowed_exercises`): Yessis 1×20 is cleanly pattern-scoped; 5/3/1
adds an explicit barbell-mains list; the Sarge circuit is mostly a bespoke
explicit list.

### 2026-08 Amendment — Galpin 3-to-5

Galpin 3-to-5 is an adult methodology whose prescription is a bounded
framework rather than a universal exercise split: the coach selects a point in
the 3-to-5 range for weekly sessions, exercises, work sets, reps, and minutes
of rest. Generation produces a one-week looping proposal with a per-set
180-300 second rest override. The deterministic lint surfaces structural
deviations for review; it neither edits the draft nor applies progression.
Loading, exercise selection, and any progression rule remain coach decisions.

The `exercise_movement_patterns` tags are seeded by extending the catalog
importer additively — `importers.ParsedExercise` gains an optional
`MovementPatterns []string`, and `ExecuteCatalogImport` writes the tag rows in
the same transaction. Omitted field → no tags; existing exports keep importing.

Youth-safety floors are **not** in these tables — they stay in code, keyed on
`tier`. So does the shared youth-rules preamble: only the **methodology-specific
per-tier block** becomes the seeded, editable `definition`.

## Implementation plan (phased — each phase is a HOF through the review gate)

- **Phase 1 — data model + seeding (`HOF-003`).** Tables, additive migration
  `0004`, models, the movement-pattern tags, and seeding: create the
  methodologies (Yessis ×2, Integrative Youth GPP, the adult set), link their
  exemplar programs and allow-lists, and tag exercises with movement patterns.
  **Additive only — no change to generation behavior or the UI.** Graduate this
  ADR to Accepted in the Phase-1 PR.
- **Phase 2 — generation wiring.** `buildSystemPrompt` / `BuildAthleteContext`
  consume the selected methodology's stored `definition` and apply its
  exercise+equipment allow-list to the catalog handed to the LLM. Methodology
  copy moves from Go literals to data; youth-safety floors stay in code.
- **Phase 3 — generate-page UI.** Replace the flat reference list with an
  explicit methodology selector scoped by the athlete's audience/tier
  (`GenerateFormData`, `web/src/pages/GeneratePage.tsx`); show the selected
  methodology's exemplars; surface the scoped catalog.

## Consequences

### Positive

- The coach selects a methodology explicitly — the generate page stops being a
  guess from a flat program list.
- Adult frameworks (5/3/1 vs. Greyskull vs. 5×5) get real, differentiated
  definitions instead of one generic block plus LLM memory.
- Methodology definitions become editable data.
- **Structural catalog scoping for kids** — the youth methodology boundary moves
  from prompt-instructed to enforced.
- `tier` semantics gain a cited evidence base (LTAD/YPD/ASM) instead of ad-hoc
  rules; methodology and developmental phase become separable axes.

### Negative / cost

- New tables and an additive migration; a prompt-composition refactor (Phase 2);
  a UI change (Phase 3); seeding and exercise-tagging effort.
- Youth-safety floors cannot be fully data-driven — they remain a code surface
  that must be maintained alongside the editable methodology copy.

### Neutral

- Subsumes HOF-002's tier→phase mapping (foundational = 1×20, intermediate =
  1×15) into methodology selection.
- Additive and phased — if the feature is never finished past Phase 1, no
  generation behavior changes.

## Alternatives considered

1. **`app_settings`-backed definitions (Option B)** — rejected: the KV store is
   too weak for the link/scoping structure.
2. **Hybrid table + settings (Option C)** — rejected for less explicitness than a
   dedicated table.
3. **Frameworks as generatable methodologies** — rejected: LTAD/YPD/ASM have no
   sets×reps recipe; modeling them as "generate this" would misrepresent them.
4. **Selectable framework-overlay axis (methodology × framework, composable)** —
   deferred: a possible future extension, likely overkill for a single-family
   program.
5. **Status quo (emergent methodology)** — rejected per the review findings.

## References

- ADR 007 — LLM-Assisted Program Generation (proposal-not-prescription; the
  `app_settings` infra).
- ADR 002 — Database Migrations (additive-only pre-production policy).
- ADR 015 — Async AI Coach Generation.
- `docs/COACH_VOICE.md` — the no-automated-coaching line.
- `docs/data-model.md` — schema source of truth.
- Youth-methodology evidence base:
  - NSCA Youth Resistance Training position stand (2009).
  - NSCA Long-Term Athletic Development position statement.
  - Faigenbaum & Myer — Integrative Neuromuscular Training.
  - Lloyd & Oliver — Youth Physical Development model.
  - Wormhoudt et al. — Athletic Skills Model.
  - Dan John — fundamental human movement patterns.
