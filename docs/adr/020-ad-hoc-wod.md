# ADR 020 — Ad-hoc WOD Generation

> Status: **Accepted** · Date: 2026-06-27
>
> Graduated from basic-memory `handoff/HOF-015`; implemented in commit
> `de3579b` (`feat(wod): ad-hoc Sarge-circuit WOD generator`); GitHub issue
> #32. Follow-up review finding tracked as #33.

## Context

The AI Coach pipeline (ADR 007, ADR 015) generates **multi-week programs**: an
async generation produces CatalogJSON that a coach approves into an
athlete-scoped, UNASSIGNED `program_template`. There was no path for a one-off
session.

The need: while running a standard program, the coach (training as an adult
athlete) wants to occasionally "mix it up" — generate a single WOD-style session
for that day, in the Sarge Athletics circuit style, scoped to the equipment on
hand. It is almost a throwaway: reviewed, then either logged or discarded. If
logged, it should feed future generation context like any other session, with no
separate storage surface.

The load-bearing principle is unchanged: **RepLog is a logbook; a human makes
every decision.** A generated WOD is a *proposal* reviewed before it becomes a
logged session — never auto-assigned, never auto-progressed (ADR 007,
`docs/COACH_VOICE.md`). Because the reviewer and the athlete are the same adult
here, the human-in-the-loop gate is the coach reading the WOD before logging it.

## Decision

Add an on-demand single-session WOD generator that **reuses the async generation
engine** but commits to a different artifact.

1. **Kind discriminator.** `generations` gains a `kind` column
   (`'program' | 'wod'`, migration 0011, additive per ADR 002; existing rows
   default to `'program'`). The per-athlete duplicate-submit guard and the
   GeneratePage resume lookup are scoped by `kind`, so a WOD in flight never
   blocks or bleeds into a program draft, and vice versa.

2. **Constrained request.** A WOD fixes `NumDays=1`, `NumWeeks=1`,
   `IsLoop=false`, and resolves the seeded **`sarge-circuit`** methodology
   (ADR 016) explicitly by key — adults have no tier-default methodology. A
   one-session framing preset is prepended to any coach directions; the existing
   adult Sarge block in `buildSystemPrompt` already encodes the circuit shape
   (EMOM opener, paired-exercise circuit rows, absolute-weight loading). The
   Circuit A/B/C reference programs are stylistic exemplars to vary from.

3. **Commit target = ad-hoc resistance workout, not a program template.** "Log
   it" (`LogWODFromCatalog`) parses the stored CatalogJSON, resolves exercise
   names → IDs, and seeds `workout_sets` on a new ad-hoc workout
   (`assignment_id` NULL). The discipline is **`resistance`** — forced by the
   read path: `ListWorkouts`/`buildRecentWorkouts` and the performance-trend
   queries filter `discipline = 'resistance'`, so a `conditioning`-logged WOD
   would be invisible to `BuildAthleteContext` and the feedback loop would
   silently break. A logged WOD therefore appears in recent-workout context and
   feeds the next generation with zero extra storage.

4. **Log-or-discard.** Logging is an explicit `POST`; "discard" is the absence
   of that call (the generation row stays unexecuted, nothing reaches the log).

5. **Same-day collision.** One resistance session per athlete per day
   (`UNIQUE(athlete_id, date, discipline)`). If a resistance workout already
   exists for the date, the log path returns `409 {collision: true}` and the SPA
   prompts **replace-or-cancel**; replace supersedes the existing workout, cancel
   is a no-op. No silent overwrite, no raw 409.

6. **Adult-only (v1).** `sarge-circuit` is adult-audience and youth-safety floors
   are a separate, larger conversation. The endpoint rejects tier-bearing (youth)
   athletes.

7. **Link-addressed resume.** A completed WOD notification links to
   `/athletes/{id}/wod?gen={generation_id}`. `WodPage` hydrates that generation
   through the existing athlete-scoped generation status endpoint and lands on
   the same preview step used by the in-session flow. The gate stays human:
   nothing is logged unless the coach explicitly clicks **Log it**; discard still
   writes nothing and no separate WOD storage surface is introduced.

### Surfaces

- `POST /athletes/{id}/wod` — enqueue (202); polling/cancel reuse the existing
  generation status/cancel endpoints.
- `POST /athletes/{id}/wod/{genID}/log` — commit (201) or `409 {collision:true}`.
- Frontend `WodPage` (form → poll → preview → log/discard) and a "🔥 WOD" entry.

## Consequences

### Positive

- A one-off "mix it up" session without disturbing the assigned program.
- Heavy reuse of the async pipeline, the Sarge methodology, and equipment
  scoping; the only genuinely new surface is the commit-to-workout path.
- Logged WODs feed future generation context for free via the existing
  recent-workout context builder.
- The async notification promise is actionable: a coach can leave the page,
   click the completion notification later, and return to the generated WOD at
   the log-or-discard review step.

### Negative / cost

- The WOD and program paths now duplicate seeding, completion-notify branching,
  the duplicate-submit guard, and resume. Acceptable for now; converge on shared
  helpers as the WOD matures rather than letting conventions drift.
- Known follow-ups (deferred deliberately): `sarge-circuit`'s
  `methodology_allowed_exercises` omits some conditioning/locomotion implements
  that lack movement-pattern tags (#33); `LogWODFromCatalog` auto-creates missing
  exercises in the global catalog (should validate at preview and reject unknowns
  instead); replace+log is not yet atomic (create-before-delete + single tx).

### Neutral / won't-fix

- AMRAP sets seed as `reps = 0` for the athlete to fill — exact parity with
  `SeedSetsFromPrescription` (`workout_sets.reps` is `NOT NULL`); a WOD-specific
  change would create drift. Any fix belongs to a logbook-wide planned-vs-performed
  modeling decision, not here.

## Alternatives considered

1. **Reuse the program execute→`program_template` flow** — rejected: a throwaway
   WOD would pile up one-day templates and route through the approve-as-draft
   path, the wrong artifact and the wrong weight.
2. **Log the WOD as `discipline = 'conditioning'`** — rejected: the
   resistance-only read filters would hide it from `BuildAthleteContext`, killing
   the feedback loop. (The `assignment_id` CHECK permits both disciplines when
   `assignment_id` is NULL, so it does not force the choice — the read path does.)
3. **A separate `wod` table / generatable methodology** — rejected: a WOD is just
   an ad-hoc workout; the existing `workouts` model (ADR 018) already fits.
4. **Synchronous generation for a "right now" feel** — deferred: reuse the async
   path as-is for v1.

## References

- ADR 007 — LLM-Assisted Program Generation (proposal-not-prescription).
- ADR 015 — Async AI Coach Generation (the reused pipeline).
- ADR 016 — First-Class Training Methodology (`sarge-circuit` + allow-list scoping).
- ADR 018 — Multi-Modal Athletic Logbook (`workouts.discipline`, ad-hoc
  `assignment_id` NULL).
- `docs/COACH_VOICE.md` — the no-automated-coaching line.
- basic-memory `handoff/HOF-015` + `HOF-015 DISCUSSION` (review record).
- basic-memory `handoff/HOF-017` + `HOF-017 DISCUSSION` (link-addressed WOD
   resume amendment).
- GitHub #32 (implemented), #33 (allow-list follow-up); commit `de3579b`.
