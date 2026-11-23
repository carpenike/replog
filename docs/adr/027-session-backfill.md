# ADR 027 — Session Backfill

> Status: **Accepted** · Date: 2026-11-22

## Context

Athletes move to RepLog with weeks of history in another app. A coach (or
the coach agent through homelab-mcp) needs to record a past resistance
session in one step: the workout and all of its sets. Logging set by set
through `POST /athletes/{id}/workouts/{workoutID}/sets` is slow, is not
atomic, and does not check the program link or a duplicate date.

## Decision

- Two coach routes, the same preview/confirm shape as ADR 024 and 026:
  `POST /athletes/{id}/coach/sessions/backfill/preview` and
  `POST /athletes/{id}/coach/sessions/backfill/confirm`. Both require the
  athlete's coach (`coachProgramAccess` with write). They sit under
  `/api/athletes/{id}/`, so the on-behalf allowlist (ADR 025) admits them
  for a coach and admin routes stay closed.
- Preview takes `date` (YYYY-MM-DD, not after the caller's today), an
  optional program link (`assignment_id`, and/or `template_id` with
  `template_version`, plus `program_week`/`program_day`), optional
  `session_rpe` (1–10) and `notes`, and `sets` (1–200). A set is
  `exercise_id`, `reps`, `weight_lbs` (null is bodyweight, stored like the
  set route stores `weight`), `rep_type`, `set_order` (unique, the session
  order), and `warmup` (stored as `set_type` `warmup`, so it stays out of
  volume, PRs, e1RM, and set counts). Unknown keys are 400.
- The link is checked, not trusted. `assignment_id` must be the athlete's;
  `template_id` and `template_version` must match that assignment's
  template. A template without an assignment resolves to the athlete's
  latest assignment of that template starting on or before the date. A
  date before the assignment's start is 400.
- *Superseded by [ADR 028](028-stored-program-day.md): the program
  week/day is now stored on the session, and a `program_day` sent is the
  day stored, with no `program_day_mismatch` warning.* A session's program
  position is derived, as everywhere else, from the
  number of linked sessions before it (`GetPrescription`). It is not
  stored. Preview reports the position the session will take and warns
  when it differs from the `program_week`/`program_day` sent
  (`program_day_mismatch`), when the date is not on the assignment's
  weekday schedule (`off_schedule`), when later linked sessions exist and
  will each move one position (`later_sessions_shift`; backfill oldest
  first), when a non-loop cycle is already full (`cycle_complete`), and
  when no link was sent but an assignment covers the date (`unlinked`).
- `session_rpe` has no column. It is written as the first line of the
  workout note (`Session RPE: 6/10`), followed by `notes`.
- Duplicates. `workouts` is unique on athlete, date, and discipline, so a
  resistance session on that date, linked or not, is a duplicate whatever
  the program day. Preview sets `duplicate_session_id` and issues no
  token. Confirm re-checks inside its transaction and refuses with 409.
  Nothing is merged or replaced.
- Unknown exercise ids are listed (`unknown_exercise`) and no token is
  issued.
- Confirm takes `preview_token` and `confirm: true`. The token is bound to
  the user and athlete, is hashed in memory, lasts 15 minutes, and is
  deleted on first use, so a replay is refused and cannot write twice.
  Confirm writes the workout (with the assignment link) and every set in
  one transaction; any failure writes nothing. It then records the same
  auto-approved review a set logged through the API gets.
- Existing workouts, sets, and PRs are never changed. PRs, e1RM, and
  plan-vs-done are computed from sets on read, so a backfilled session
  counts exactly like a live-logged one.

## Consequences

- A coach can move history in one call per session and see exactly what
  will be stored first.
- Backfilling out of order shifts the derived program day of later linked
  sessions. Preview says so; the coach agent should backfill oldest first.
- A per-session RPE is readable in the note but not queryable. A column can
  be added later without changing the API.
