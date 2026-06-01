# ADR 018 — Multi-Modal Athletic Logbook

> Status: **Accepted** · Date: 2026-05-29
>
> Drafted by Coach, reviewed through the basic-memory `handoff/HOF-008`
> review gate, and graduated to **Accepted** with the Phase-1
> implementation (migration `0006_multi_modal_logbook.sql`, models, and
> handlers). The schema specifics below reflect the implemented DDL.

## Context

RepLog is framed everywhere — AGENTS.md, README, `docs/COACH_VOICE.md` — as a
**resistance-training** logbook. The framing is load-bearing in the data
model: the logging unit is a `workouts` row (one per athlete per day —
`UNIQUE(athlete_id, date)`) holding `workout_sets` (one row = one set =
reps + weight). `rep_type` already carries `each_side`, `seconds`, and
`distance`, so a run *can* be crammed into a set — but there is no pace, no
heart rate, no intervals, no throw count, no subjective check-in, and the
one-session-per-day constraint means a kid who lifts and throws on the same
day cannot record both.

A family athlete (12–13, novice) is taking up baseball pitching. The
household now needs to see strength, conditioning, throwing, sport-skill,
and recovery as **one picture**, because in youth baseball the dominant
injury driver is not any single session type — it is **cumulative throwing
load and throwing while fatigued**:

- Pitching regularly while fatigued ≈ **36×** injury odds (Olsen et al.
  2006, *AJSM*).
- **>100 innings/calendar year ≈ 3.5×** (Fleisig et al. 2011, *AJSM*).
- USA Baseball / Pitch Smart publishes age-based daily pitch maxima, a
  tiered rest-days-owed schedule, a no-three-consecutive-days rule, and a
  2–3 month annual throwing-shutdown recommendation.

The single highest-value thing the app can do for this athlete is make
**total workload across every modality** legible and surface the Pitch
Smart guidance — for a human to act on. Generic conditioning is secondary;
arm-care load is the safety centerpiece.

The load-bearing principle is unchanged and extends to every new modality:
**RepLog is a logbook; a human coach makes every decision. The app never
automates coaching** (README, ADR 007, `docs/COACH_VOICE.md`). Pitch Smart
limits, like the ADR 016 youth-safety floors, are **code-enforced reference
checks that emit coach-reviewed proposals — never auto-actions, never hard
blocks on logging.**

## Decision

RepLog becomes a **multi-modal athletic logbook**. Resistance training
remains the most developed surface; conditioning, throwing/arm-care,
sport-skill, and mobility/recovery become first-class peers.

1. **Identity amendment.** Update the "resistance-training logbook" framing
   in AGENTS.md, README, and `COACH_VOICE.md` to "athletic training
   logbook." The no-automated-coaching line is unchanged and is explicitly
   extended to all modalities.

2. **A `discipline` axis on the session parent.** The per-day session
   (today `workouts`) gains a `discipline` discriminator
   (`resistance` | `conditioning` | `throwing` | `skill` | `recovery`),
   defaulting to `resistance` for all existing rows. The per-day uniqueness
   becomes **per `(athlete_id, date, discipline)`** so an athlete can log a
   lift and a bullpen on the same day. Each discipline hangs its own detail
   table off the session parent; resistance keeps `workout_sets` unchanged.
   - **Keep the `workouts` name; add `discipline`** (resolved in HOF-008
     review). A rename to `training_sessions` touches ~23 query sites across
     7 model files for zero functional gain, and the detail tables already
     FK `workouts(id)`. Rename rejected.
   - **`assignment_id` is a resistance-only invariant.** Non-resistance rows
     never carry an `assignment_id` (AI generation stays resistance-only per
     Decision #7), which is what keeps the program position-counting intact.
     Guard it structurally: `CHECK(assignment_id IS NULL OR discipline =
     'resistance')`.
   - **Migration mechanics (review finding).** Swapping the uniqueness
     constraint forces the standard SQLite table rebuild — but goose runs the
     migration inside a transaction, where `PRAGMA foreign_keys=OFF` is a
     no-op, so the textbook 12-step recipe would leave enforcement on and the
     `DROP TABLE workouts` would CASCADE-delete `workout_sets` /
     `workout_reviews`. Use the transaction-safe `PRAGMA
     defer_foreign_keys=ON` and run `PRAGMA foreign_key_check` before commit.

3. **Season phase is first-class and dated.** A new `athlete_season_phases`
   table records `off` / `pre` / `in`-season windows per athlete per sport
   with start/end dates. Season phase changes what is appropriate (off-season
   builds, in-season maintains; lifts scheduled around starts) and is the
   context layer for the weekly-load view and any future proposal surface.

4. **Throwing/arm-care is the safety centerpiece and ships with
   coach-reviewed Pitch Smart flags in Phase 1.** A `throwing_sessions`
   detail table records throw count, throw type (`game` / `bullpen` /
   `lesson` / `long_toss` / `catch` / `flat_ground` / `position`), max-intent %,
   optional velocity, a **fatigue flag**, and a **pain flag**. Pitch Smart
   reference data (daily max by age, tiered rest-days-owed, innings caps,
   shutdown windows) is seeded, and the app computes **rolling rest-days-owed and
   limit/inning checks** as proposals a coach reviews. The computation reads
   workload and emits a flag; it never writes a progression, blocks a log, or
   instructs the athlete.
   - **Amendment (HOF-010, 2026-06-01):** added the `position` throw type for
     infield/position throwing (two-way players), and **scoped the Pitch Smart
     pitch-count advisory to mound pitching (`game`, `bullpen`) only** — `catch`,
     `long_toss`, `flat_ground`, and `position` no longer drive the pitch-count
     rest math (Pitch Smart limits are pitch counts). The cross-modal load view
     is unchanged: it still sums `throw_count` across **all** throw types, so
     total throwing load remains the broader safety number.

5. **Cross-source throwing is in scope.** Throwing sessions carry a
   source/context and external sessions (another team's game, a private
   lesson, a showcase) are manually enterable. Workload aggregation must
   include *all* throwing, because the research is emphatic that injury risk
   tracks total volume across every team and lesson, not just in-program
   throwing.

6. **Bio-indicators as source-tagged dated samples.** A `bio_samples` table
   stores `(athlete_id, recorded_at, metric, value, unit, source)` where
   `source ∈ {manual, watch_import, …}`. Sleep is the first first-class
   metric (a top recovery/injury correlate). This keeps a wearable a **feed,
   not a dependency** — a Garmin/Whoop/Apple Watch later is another `source`,
   not a migration. Ingestion (an Apple Shortcut on the guardian's phone
   POSTing to the API or the ADR 017 MCP layer; or a Health XML import per
   ADR 006) is deferred; the schema is laid now.

7. **AI generation stays resistance-only in v1.** Methodology-driven
   generation (ADR 007 / 015 / 016) does not draft throwing, conditioning,
   or skill programs. New modalities are **log-and-flag**, never generate —
   the conservative default for youth arms. Weighted-ball/velocity
   programming is explicitly out of scope (contraindicated < ~16 yo;
   Reinold et al. 2018).

### Modality detail-table shapes (sketch — settled in HOF-008)

| Discipline    | Detail surface                                                        | Phase |
|---------------|-----------------------------------------------------------------------|-------|
| resistance    | existing `workout_sets` (unchanged)                                   | live  |
| throwing      | `throwing_sessions` — count, type, intent %, velocity, fatigue, pain  | **1** |
| conditioning  | session type (sprint/interval/steady/row) + intervals (work/rest)     | 2     |
| skill         | rep counts, med-ball **load in kg**, optional velocity                | 2     |
| recovery      | subjective check-ins (sleep, soreness, RPE) + links to `bio_samples`  | 2     |

## Schema sketch (new migration `0006`; ADR 002)

Phase-1 tables. The new tables are purely additive; the `discipline` axis on
the session parent is **not** — swapping the uniqueness constraint forces a
SQLite table rebuild (see Decision #2). New mutable tables get the standard
`updated_at` trigger.

```sql
-- (2) discipline axis on the session parent — keep the `workouts` name.
--     Constraint change forces a SQLite table rebuild; do it with
--     PRAGMA defer_foreign_keys=ON (txn-safe) + foreign_key_check before commit.
--     discipline TEXT NOT NULL DEFAULT 'resistance'
--         CHECK(discipline IN ('resistance','conditioning','throwing','skill','recovery'))
--     CHECK(assignment_id IS NULL OR discipline = 'resistance')  -- protects position-counting
--     UNIQUE(athlete_id, date, discipline)   -- replaces UNIQUE(athlete_id, date)

-- (3) season phase, first-class + dated
CREATE TABLE athlete_season_phases (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    sport       TEXT,                              -- e.g. 'baseball'; nullable for general
    phase       TEXT NOT NULL CHECK(phase IN ('off','pre','in')),
    start_date  DATE NOT NULL,
    end_date    DATE,                              -- NULL = current/open-ended
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- (4) throwing detail
CREATE TABLE throwing_sessions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id    INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE, -- the discipline='throwing' parent
    throw_type    TEXT NOT NULL CHECK(throw_type IN ('game','bullpen','lesson','long_toss','catch','flat_ground','position')),  -- 'position' added in HOF-010 (migration 0008)
    throw_count   INTEGER,                         -- pitch/throw count
    max_intent    INTEGER,                         -- % effort, nullable
    velocity      REAL,                            -- optional radar reading; never a target
    fatigue       INTEGER NOT NULL DEFAULT 0 CHECK(fatigue IN (0,1)),
    pain          INTEGER NOT NULL DEFAULT 0 CHECK(pain IN (0,1)),
    source        TEXT NOT NULL DEFAULT 'program' CHECK(source IN ('program','external')),
    team          TEXT,                            -- free text for cross-team aggregation
    notes         TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- (4) Pitch Smart reference data — seeded, read-only reference for the flag engine.
CREATE TABLE pitch_smart_limits (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    age_min         INTEGER NOT NULL,
    age_max         INTEGER NOT NULL,
    daily_max       INTEGER NOT NULL,              -- e.g. 85 (11–12), 95 (13–14)
    rest_thresholds TEXT NOT NULL                  -- JSON: [{pitches:21,rest_days:1}, ...]
);

-- (6) source-tagged bio samples (watch is a feed, not a dependency)
CREATE TABLE bio_samples (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id   INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    recorded_at  DATETIME NOT NULL,
    metric       TEXT NOT NULL,                    -- 'sleep_hours','resting_hr','hrv','session_hr', …
    value        REAL NOT NULL,
    unit         TEXT,
    source       TEXT NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','watch_import')),
    notes        TEXT,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

The journal (`internal/models/journal.go`, data-model design note 17) gains
new `UNION ALL` sources for throwing sessions and season-phase changes, so
multi-modal events land on the existing timeline with no denormalized table.
**Review finding:** the existing `'workout'` UNION branch must gain `AND
w.discipline = 'resistance'` or it will fire for throwing parents and render
"Workout (0 sets)"; the positional arg-slice grows with each new branch.

Import/export: the `discipline` discriminator round-trips on import by
defaulting to `'resistance'` (backward-compatible with existing JSON). There
is **no outbound per-athlete workout export** — it was removed (issue #11),
so there is nothing to round-trip on export. If per-athlete export is ever
reinstated, it must carry the discriminator; not a Phase-1 criterion.

## Implementation plan (phased — each phase is a HOF through the review gate)

- **Phase 1 — `HOF-008`.** The discipline axis on the session parent +
  per-day uniqueness change (additive migration `0006`), the
  `athlete_season_phases` table, the `throwing_sessions` detail with
  fatigue/pain, the seeded `pitch_smart_limits` + the coach-reviewed
  rolling rest-days-owed / limit flag computation, the `bio_samples`
  scaffold (schema + manual entry), and the journal UNION extension. Models
  + handlers + tests. Graduate this ADR to Accepted in the Phase-1 PR.
  *Note: this phase is larger than a pure-additive data-only phase because
  the Pitch Smart flags were pulled forward (host decision) — they are the
  highest-value safety surface.* A required test proves the no-automation
  line: an over-limit throwing session still logs successfully (201) and the
  Pitch Smart result is an advisory flag, never a gate.
- **Phase 2 — remaining modalities.** Conditioning (session type +
  intervals), sport-skill (rep counts + med-ball load), recovery subjective
  check-ins, and the **unified weekly cross-modal load view** (the
  acute-vs-chronic picture across lifting + throwing + conditioning).
- **Phase 3 — wearable ingestion.** The watch→RepLog bridge: an Apple
  Shortcut on the guardian's phone POSTing scoped metrics to the API or the
  ADR 017 MCP layer, with the Health XML import (ADR 006) as fallback. Open
  question to resolve first: whether third-party HealthKit reads can see
  *family-shared* data, or only Apple's own UI can.

## Consequences

### Positive

- Total workload becomes legible across every modality — the actual youth
  injury signal (fatigue + cumulative throwing volume), not a per-session
  blind spot.
- The arm-care surface lands early and conservatively: the app surfaces
  Pitch Smart guidance, the coach decides — a textbook fit for the
  no-automated-coaching line.
- Wearable data is a pluggable feed; no Apple-specific coupling in the model.
- Season phase becomes a real, queryable axis instead of living in a coach's
  head.

### Negative / cost

- The session-parent change is the most invasive migration to date: a SQLite
  table rebuild plus a uniqueness-constraint change. The HOF-008 review
  grep-enumerated the blast radius and found it contained — keep-name avoids
  the ~23-site rename churn, and the rebuild is bounded to the migration plus
  the journal guard, given the `defer_foreign_keys` and `assignment_id`
  guards above.
- A new code surface — the Pitch Smart flag computation — must stay on the
  proposal side of the no-automation line and be tested as such.

### Neutral

- Additive and phased. If only Phase 1 ships, resistance behavior is
  unchanged and the new disciplines are simply available to log.
- Methodology (ADR 016) and discipline are orthogonal axes; methodology
  stays resistance-scoped for now.

## Alternatives considered

1. **Sibling layer (leave `workouts` untouched, add parallel modality
   tables, unify only at reporting).** Rejected — the unified weekly-load
   view and the journal want one session parent; two parallel hierarchies
   duplicate the date/athlete/notes/review scaffolding.
2. **Cram cardio into `workout_sets` via the existing `distance`/`seconds`
   rep types.** Rejected — no pace/HR/interval/throw-count structure, and the
   one-session-per-day constraint still blocks same-day multi-modal logging.
3. **Defer arm-care flags to a later phase (data-only Phase 1).** Rejected by
   host — arm-care is the highest-value safety surface and justifies the
   larger first phase.
4. **Let the LLM generate throwing/conditioning programs.** Rejected —
   too high-stakes for youth arms; new modalities are log-and-flag in v1.

## References

- ADR 007 — LLM-Assisted Program Generation (proposal-not-prescription).
- ADR 002 — Database Migrations (additive-only pre-production policy).
- ADR 016 — First-Class Training Methodology (youth-safety-floors-in-code
  pattern that the Pitch Smart flags mirror).
- ADR 017 — MCP Layer (candidate wearable-ingestion endpoint).
- ADR 006 — Import/Export (fallback Health XML ingestion path).
- `docs/COACH_VOICE.md` — the no-automated-coaching line.
- `docs/data-model.md` — schema source of truth.
- Youth-baseball evidence base (modality research, 2026-05-29):
  - Olsen et al. 2006, *AJSM* — fatigue ≈ 36× injury odds.
  - Fleisig et al. 2011, *AJSM* — >100 IP/yr ≈ 3.5×.
  - Reinold et al. 2018, *Sports Health* — weighted-ball 24% injury rate.
  - von Rosen et al. 2017, *Scand J Med Sci Sports* — sleep & injury.
  - USA Baseball / Pitch Smart — pitch-count & rest guidelines.
  - AAP 2020 (reaff. 2024) / NSCA 2009 — youth resistance-training safety.
