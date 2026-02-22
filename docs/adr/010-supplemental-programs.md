# ADR 010: Supplemental Programs — Multiple Active Programs Per Athlete

**Status:** Proposed
**Date:** 2026-02-22

## Context

Athletes often want to follow a primary strength program (e.g., 5/3/1 on Mon/Wed/Fri) while also doing different work on off-days (e.g., Sarge Athletics circuits on Tue/Thu). Currently, RepLog enforces **exactly one active program per athlete** via a partial unique index:

```sql
CREATE UNIQUE INDEX idx_athlete_programs_active
    ON athlete_programs(athlete_id) WHERE active = 1;
```

This makes it impossible to run two programs concurrently. An athlete must fully deactivate their strength program to try a circuit day, which breaks position tracking and is operationally painful.

### Current limitations

1. **One active program** — the partial unique index prevents simultaneous assignments
2. **Positional day numbering** — `GetPrescription()` counts all workouts since `start_date` and computes `position = count % cycleLength`. It doesn't know which program a workout "belongs to," so adding a second program would advance both programs' counters on every workout
3. **No workout→program link** — the `workouts` table has no `program_id` or `assignment_id` column; the link is computed at runtime
4. **Accessory plans serve a different purpose** — they provide lightweight per-day hints ("do 3×10 curls") but lack template backing, week dimensions, and position tracking needed for structured supplemental programs like circuits

### Use case

> "I want to run 5/3/1 on Monday, Wednesday, Friday and a Sarge circuit on Tuesday and Thursday."

This requires two active programs where each workout is explicitly tagged to a specific program, and each program's position advances independently based only on its own workouts.

## Decision

### Core concept: assignment roles

Add a `role` column to `athlete_programs` with two values: `primary` and `supplemental`. The constraint changes from "one active program" to "one active primary + any number of active supplementals."

When the athlete opens their workout page, the app determines which program to prescribe for today. The coach assigns programs with day-of-week routing so the system knows which days map to which program.

### Schema changes (modify initial migration — pre-v1)

#### 1. `athlete_programs` — add `role` and `schedule`

```sql
CREATE TABLE IF NOT EXISTS athlete_programs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    template_id INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE RESTRICT,
    start_date  DATE    NOT NULL,
    active      INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    role        TEXT    NOT NULL DEFAULT 'primary' CHECK(role IN ('primary', 'supplemental')),
    schedule    TEXT,  -- JSON array of weekday numbers, e.g. '[2,4]' for Tue/Thu; NULL = any day
    notes       TEXT,
    goal        TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One active primary program per athlete.
CREATE UNIQUE INDEX idx_athlete_programs_active_primary
    ON athlete_programs(athlete_id) WHERE active = 1 AND role = 'primary';
```

Key changes:
- **`role`** — `'primary'` (default) or `'supplemental'`
- **`schedule`** — nullable JSON array of ISO weekday numbers (1=Monday through 7=Sunday). NULL means "any day not claimed by another program" (the default for primary programs)
- **Unique index** — now scoped to `role = 'primary'` — only one active primary, but unlimited active supplementals
- No unique index on supplementals — an athlete could have several (e.g., circuit on Tue/Thu, yoga on Sunday)

#### 2. `workouts` — add `assignment_id`

```sql
CREATE TABLE IF NOT EXISTS workouts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id    INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    assignment_id INTEGER REFERENCES athlete_programs(id) ON DELETE SET NULL,
    date          DATE    NOT NULL,
    notes         TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, date)
);
```

Key change:
- **`assignment_id`** — nullable FK to `athlete_programs(id)`. Links a workout to the specific program assignment it was prescribed from. `ON DELETE SET NULL` so deactivating/deleting a program doesn't cascade-delete workout history.
- When a workout is created, the app stamps it with the resolved assignment ID. This makes position tracking unambiguous — count only workouts with matching `assignment_id`.

#### 3. Keep `accessory_plans` (no change)

Supplemental programs and accessory plans serve fundamentally different purposes:

| | Accessory Plans | Supplemental Programs |
|---|---|---|
| Purpose | Per-day hints alongside main work | Structured programs on dedicated days |
| Granularity | "Do 3×10 curls" | Full template with weeks, days, prescribed sets |
| Position tracking | None needed | Yes, per-assignment |
| Schedule | Same day as main workout | Own dedicated days |
| Example | Face pulls after bench day | Sarge Circuit A on Tue/Thu |

Accessories remain lightweight coaching guidance attached to the same workout as the primary program. Supplemental programs are full-fledged programs that own entire days. No change to the `accessory_plans` table.

### Prescription engine changes

#### Layered resolution: `ResolveAssignment` + `GetPrescription`

To keep complexity isolated and testable, split the work into two functions:

**`ResolveAssignment(db, athleteID, date) → *AthleteProgram`** — pure routing logic:

```
func ResolveAssignment(db, athleteID, date) *AthleteProgram:
    1. Load all active assignments for this athlete
    2. Determine today's ISO weekday (1=Mon … 7=Sun)
    3. Find the matching assignment:
       a. If a supplemental has today's weekday in its schedule → use it
       b. Otherwise → use the primary (schedule=NULL means "any unscheduled day")
    4. Return the matched assignment (or nil for rest day)
```

This function is small, pure, and trivially unit-testable — input: list of assignments + weekday, output: one assignment.

**`GetPrescription(db, assignment) → *Prescription`** — unchanged in structure:

```
func GetPrescription(db, assignment) *Prescription:
    1. Count prior workouts WHERE assignment_id = assignment.ID
    2. Compute position = count % cycleLength
    3. Fetch prescribed_sets for (week, day)
    4. Resolve TM-based target weights
    5. Return prescription with assignment context
```

`GetPrescription()` still operates on a **single assignment** — the only change is that it counts workouts by `assignment_id` instead of all workouts since `start_date`. The routing complexity lives entirely in `ResolveAssignment`.

The key insight: **position advances independently per assignment** because we count only workouts with the matching `assignment_id`, not all workouts.

#### Schedule conflict prevention

Schedule conflicts are **rejected at assignment time**, not resolved at runtime. When a coach assigns a supplemental with a schedule:

1. Load all active assignments for this athlete
2. Check if any claimed weekdays overlap with the new assignment's schedule
3. If overlap → reject with a clear error: "Tuesday is already assigned to Sarge Circuit A"
4. The UI grays out already-claimed days in the day-of-week picker

This eliminates all runtime ambiguity. There is never a case where two programs claim the same day.

#### Handling edge cases

- **Rest day** — if today's weekday isn't in any program's schedule and the primary has a schedule set, no prescription is shown. The athlete can still log an ad-hoc workout (no `assignment_id` stamped).
- **No supplementals** — behavior is identical to today. Primary gets all days, `schedule` is NULL by default, `assignment_id` gets stamped on workout creation.

### Workout creation flow

When an athlete opens the workout page for today:

1. `ResolveAssignment()` determines which program owns today
2. If creating a new workout: `INSERT INTO workouts (athlete_id, assignment_id, date) VALUES (?, ?, ?)`
3. Stamp the resolved `assignment_id` so position tracking is correct
4. **Coach-only override** — coaches see a small "Switch to [other program]" link if multiple programs are active. Non-coach athletes get the scheduled program automatically with no decision required. This keeps the workflow frictionless for kids who just need to follow the plan.

### UI changes

#### Assignment flow

The existing "Assign Program" page gains:
- **Role selector** — radio: "Primary program" (default) / "Supplemental program"
- **Schedule picker** — when supplemental is selected, show day-of-week checkboxes (Mon–Sun). At least one day required for supplementals. Optional for primary.
- When assigning a supplemental, the primary stays active — no deactivation prompt
- **Schedule conflict validation** — the UI grays out weekdays already claimed by another active assignment and rejects overlapping schedules on submit

#### Athlete program view

The athlete's program card shows:
- Primary program with "Active" badge and its schedule (or "All other days")
- Supplemental programs listed below with their schedules and individual deactivate buttons

#### Workout detail page

- The prescription scaffold shows which program today's workout comes from: "Today: **5/3/1 for Beginners** — Week 2, Day 3" or "Today: **Sarge Circuit A** — Day 1"
- Coaches see a small "Switch to [other program]" link to override the auto-resolved program; non-coach athletes do not
- Accessory plan section remains for per-day accessory guidance alongside the main prescription

#### AI generation

The generate form gains:
- **Assignment role** — "Generate as primary program" / "Generate as supplemental program"
- **Schedule** — day-of-week picker when supplemental is selected
- The import flow stamps the correct role and schedule on the `athlete_programs` row

### Data model summary

```
athlete_programs
  + role TEXT NOT NULL DEFAULT 'primary' CHECK(role IN ('primary', 'supplemental'))
  + schedule TEXT  -- JSON weekday array, e.g. '[2,4]'
  ~ unique index: scoped to role='primary' only

workouts
  + assignment_id INTEGER REFERENCES athlete_programs(id) ON DELETE SET NULL

accessory_plans
  (no change — kept as-is for lightweight per-day accessory guidance)
```

### Position tracking comparison

| Aspect | Before | After |
|--------|--------|-------|
| Workouts counted | All since `start_date` | Only those with matching `assignment_id` |
| Program link | Computed at runtime from date counting | Explicit FK on `workouts.assignment_id` |
| Day meaning | Positional slot (1st, 2nd, 3rd workout) | Still positional, but per-program |
| Rest days | Implicit (no workout = no advance) | Same, but schedule makes intent explicit |
| Multiple programs | Not possible | Primary + N supplementals with schedules |

## Consequences

### Positive

- Athletes can mix programming styles (strength + circuit, strength + mobility) on different days of the week
- Position tracking becomes **explicit and unambiguous** via `workouts.assignment_id` — no more fragile "count all workouts" heuristic
- Programs advance independently — a missed circuit day doesn't affect the strength program's position
- The `workout_sets.category` column (`main`, `supplemental`, `accessory`) gains real meaning — sets from a primary program are `main`, sets from a supplemental program are `supplemental`
- Accessory plans remain for lightweight per-day guidance — no friction increase for simple "do 3×10 curls" coaching notes
- Coach can assign reference programs (like Sarge circuits) directly as supplementals without AI generation
- AI generation can target supplemental slots specifically
- Schedule conflicts are impossible at runtime — validated and rejected at assignment time
- Non-coach athletes get zero new decision points — the scheduled program loads automatically

### Negative

- Adds a new `ResolveAssignment()` function (small, isolated, testable — but is new code to maintain)
- Coaches must understand the primary vs. supplemental distinction when assigning programs

### Migration path (pre-v1)

Since we haven't released v1, all changes go directly into the initial migration (`0001_initial_schema.sql`). No separate migration needed. Steps:

1. Modify `athlete_programs` DDL — add `role`, `schedule`, change unique index
2. Modify `workouts` DDL — add `assignment_id`
3. Update `data-model.md` to reflect changes
4. Update all model functions, handlers, and templates

### Implementation order

1. **Schema + data-model.md** — modify DDL, update docs
2. **Models** — update `athlete_program.go` (add role/schedule, schedule conflict validation), add `ResolveAssignment()` to `prescription.go`, update `GetPrescription()` to count by `assignment_id`, update `workout.go` to stamp `assignment_id`
3. **Handlers** — update assignment flow (role/schedule picker, conflict validation), workout detail (program indicator, coach-only override), generate import (role/schedule stamping)
4. **Templates** — update assignment form, workout detail (program header, coach switch link), keep accessory plan templates
5. **Tests** — update affected tests, add new tests for multi-program resolution, schedule conflict rejection, coach-only override
6. **LLM context** — update `context.go` to include supplemental program info in athlete context

### Future considerations

- **Schedule templates** — predefined schedules like "M/W/F" or "T/Th" as quick-pick options
- **Auto-schedule detection** — if an athlete consistently logs on certain days, suggest a schedule
- **Cross-program deload** — when one program deloads, should the supplemental also deload? Currently independent.
- **Shared progression rules** — if both programs use Power Clean, do TM bumps from the primary affect the supplemental? Currently yes, since TMs are per-athlete per-exercise, not per-program.
