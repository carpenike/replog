# Requirements

> RepLog v1.0 — user stories and acceptance criteria.

## Terminology

- **Athlete**: Any person tracked in the system (kids or adults).
- **Coach**: The human user making training decisions. The app does not automate coaching.
- **Tier**: Foundational → Intermediate → Sport Performance. Applies to kids' progression per the Dr. Yesis methodology.
- **Training max (TM)**: The reference weight used for percentage-based programming.

---

## v1.0 — Core Tracking

### Exercise Management

- [x] **Create exercise** with name, optional tier, optional target reps, optional form notes
- [x] **Edit exercise** — update any field
- [x] **Delete exercise** — only if not referenced by any workout sets (prevent orphaned history)
- [x] **List exercises** — filterable by tier (including "no tier")
- [x] **View exercise detail** — shows form notes, which athletes are assigned, recent log history

### Athlete Profiles

- [x] **Create athlete** with name and optional tier
- [x] **Edit athlete** — update name, tier, notes
- [x] **Promote athlete to next tier** — manual tier change with confirmation (coach decision)
- [x] **List athletes** — shows name, current tier, number of active assignments
- [x] **View athlete detail** — profile, active exercises, recent workouts, training maxes

### Exercise Assignments

- [x] **Assign exercise to athlete** — creates active assignment
- [x] **Unassign exercise** — deactivates (preserves history, does not delete)
- [x] **Reactivate assignment** — creates a new assignment row (preserves audit trail, fresh `assigned_at`)
- [x] **View active assignments** for an athlete — shows exercise name, tier, target reps

### Training Maxes

- [x] **Set training max** for an athlete + exercise with weight and effective date
- [x] **Update training max** — adds a new row (history preserved, not overwritten)
- [x] **View current training max** per exercise for an athlete
- [x] **View training max history** for an athlete + exercise (progression over time)

### Workout Logging (Core Loop)

- [x] **Start workout** for an athlete on a date (creates workout record)
- [x] **Daily workout view** — shows athlete's active exercises with target reps and current TM
- [x] **Log a set** — select exercise (assigned shown first, full library accessible), enter reps, optional weight, optional notes
- [x] **Edit a logged set** — fix typos in reps/weight/notes
- [x] **Delete a logged set** — remove an erroneous entry
- [x] **Add workout notes** — session-level observations
- [x] **One workout per athlete per day** — enforced by schema

### Workout History

- [x] **View workout history** per athlete — list of past workouts with date and summary
- [x] **View workout detail** — all sets logged, grouped by exercise, with notes
- [x] **View exercise history** per athlete — all sets for a specific exercise over time

### Auth & Session

- [x] **Auto-create admin on first run** — if no users exist, create from env vars `REPLOG_ADMIN_USER` / `REPLOG_ADMIN_PASS` / `REPLOG_ADMIN_EMAIL` with `is_coach = 1`
- [x] **Simple login** — username/password, session cookie
- [x] **Coach access** — coaches (`is_coach = 1`) can view/manage all athletes, exercises, assignments, and workouts
- [x] **Kid access** — non-coaches are linked to one athlete and can only view/log/edit their own workouts
- [x] **Unlinked non-coach** — if a non-coach user has no linked athlete, show an informative message (not a blank screen)
- [x] **Athlete selector** — coaches can switch between athletes; non-coaches land directly on their profile
- [x] **Session persistence** — stay logged in across browser restarts (scs defaults: Cookie.Persist=true + 30-day lifetime)

---

## v1.1 — Nice-to-Have

- [x] Rest timer between sets (configurable per exercise or global)
- [x] Weekly completion streaks (did the athlete complete all assigned exercises?)
- [x] Exercise demo video links (URL field on exercise)
- [x] Printable workout cards (HTML print stylesheet)
- [x] RPE (rate of perceived exertion) field on workout sets
- [x] Program templates with structured periodization (5/3/1, GZCL, etc.)
- [x] "Today's prescription" view derived from program template + training maxes
- [x] Body weight tracking

---

## Non-Goals (v1)

- No automated tier progression — the coach decides, period
- No multi-family / multi-coach support — single family, single deployment
- No native mobile app — responsive web is sufficient
- No complex permissions / role-based access — just coach vs non-coach (`is_coach` flag)
- No data export (CSV, etc.) — SQLite file is the export
- No exercise recommendation engine

---

## Acceptance Criteria Notes

Each checkbox above represents a testable feature. "Done" means:

1. The feature works end-to-end in the browser (htmx interactions, form submissions)
2. Data persists in SQLite across server restarts
3. Edge cases handled (empty states, duplicate prevention, validation errors shown to user)
4. No JavaScript build step required — all interactivity via htmx + server-rendered HTML
