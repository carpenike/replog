# ADR 024 — Coach Program Import

> Status: **Accepted** · Date: 2026-11-22

## Context

An outside coach writes the program. They do not get a RepLog login.
The athlete's RepLog coach (patient zero: Ryan, coaching Ryan) stores
that program through a service token so a later agent can log against
it. The agent must not become the coach. ADR 007 still holds: a person
owns the sets, the reps, the exercises, and the notes.

The existing template is a grid of prescribed sets. A coach's day also
has instructions, an equipment list, a duration, and blocks (straight,
superset, circuit) with rep ranges, RIR ranges, per-side work, and rest
at either the exercise or the block. A weekly Mon/Wed/Fri plan and a
count of upcoming sessions are schedule metadata. They are not copies
of the day.

## Decision

- Migration `0019` adds the catalog fields an import needs (`exercise_aliases`,
  `exercise_muscles`, `exercises.unilateral`) and the program document
  (`program_imports`, `program_days`, `program_blocks`, `program_block_exercises`).
  Prescribed sets are still written so the existing week and position
  math can find the day. The block row is what a read returns when the
  caller needs the range, the note, or which side the rest sits on.
- `POST /athletes/{id}/coach/programs/preview` writes nothing and returns
  a one-time token (15 minutes), the normalized plan, exercise matches,
  names that would be created, and fields the schema does not store.
  `POST .../import` with `confirm: true` saves that plan. Missing
  exercises are created in the same transaction. A second import of the
  same name inserts the next version. The previous row is not updated.
  Approved names are unique per version (this relaxes the name-only
  uniqueness in ADR 023).
- The saved template is locked. Update, delete, and prescribed-set edits
  return 409. Gap-fill (`POST .../gap-fill`) may set only a field the
  coach left blank, and it stores that value on `ai_*` columns. A request
  that names a coach-set field, including the exercise, is refused and
  writes nothing.
- Assign (`POST .../programs/assign`) puts one version on the athlete and
  copies the stored weekdays onto the assignment. It does not edit the
  template. The general `POST /athletes/{id}/programs` route still
  refuses a service token (ADR 022).
- These routes take an explicit athlete id. A missing athlete is 404.
  A caller who cannot see that athlete gets 403. A service token does
  not gain an admin's view of every athlete. Writes require the caller
  to be that athlete's coach (a browser admin may also write).
- Coach session, week, and position reads include `program_day` when the
  assignment points at an imported template. Those reads do not change
  the program or a training max.

## Consequences

- Day 1 of a coach's plan can be stored and logged against without
  inventing the days that have not been sent yet.
- A later version is a new row. The locked row stays the record of what
  the coach wrote.
- An AI suggestion for a blank load is visible beside the coach's reps
  and notes, and it is not written into those columns.
