# ADR 028 — Stored Program Day

> Status: **Accepted** · Date: 2026-11-23

## Context

A linked session's program week/day was derived by counting the sessions
linked to the same assignment before its date (ADR 027). That only works
when every session is logged in order and on the template's day order. It
breaks for history moved from another app: Ryan trained the Phase One
template as Day 3 on Fri 6 Nov, then Day 1, 2, 3 each Mon/Wed/Fri, so the
count called 6 Nov "Week 1 Day 1" and plan-vs-done compared it against the
wrong day. The web app also started sessions without an assignment link,
so live sessions never advanced the program at all.

## Decision

- `workouts.program_week` and `workouts.program_day` (migration 0028),
  nullable. Both are set or both are NULL; they are set only on a session
  linked to an assignment. Rows from before 0028 stay NULL.
- Reading a date's program day (`GetPrescription`, so plan-vs-done, the
  coach position and next session, the whiteboard, and seeding):
  1. the linked session on that date has a stored day: use it (`stored`);
  2. otherwise the day after the latest earlier linked session with a
     stored day, wrapping at the end of the cycle (`next_after_stored`);
  3. otherwise the count of linked sessions before the date, as before
     (`count`).
  Coach history uses the stored day per session and the count for older
  rows. Plan-vs-done reports the source as `day_source`.
- A template that stores prescribed sets for week 1 only (an imported
  weekly cycle such as Phase One, 8 weeks × 3 days) repeats week 1: week 3
  day 2 reads week 1 day 2's prescription (`TemplateWeekFor`).
- Backfill (`program_week`/`program_day`): the day sent is validated (week
  1..num_weeks, day 1..num_days, and the template has prescribed sets for
  it) and stored. `program_week` without `program_day` is 400;
  `program_day` alone takes the week from the calendar: Monday–Sunday weeks
  counted from the week the assignment starts in (a Thursday 5 Nov start
  makes 2–8 Nov week 1). With no day sent the derived day is stored.
  `program_day_mismatch` is gone; `later_sessions_shift` now counts only
  later linked sessions without a stored day. Preview reports
  `program_day_source` (`given` or `derived`); confirm returns
  `program_week`/`program_day`.
- Live logging: `POST /athletes/{id}/workouts` takes `link_program`,
  `program_week`, `program_day`. `link_program`, `from_prescription`, or
  `program_day` links the session to the assignment on its date (not the
  server's today) and stores the day sent, or the next day by default.
  The web app's Start session and Start Logging now link; Start Logging has
  a program-day picker, and `GET /prescription?week=&day=` shows a chosen
  day first.
- Plan-vs-done no longer lists days before the assignment's start date.

## Consequences

- Plan-vs-done compares each session against the day it really was,
  whatever order sessions are logged or backfilled in.
- Older linked sessions keep their counted day. A session logged after a
  stored one follows the stored day, not the count.
- Live web sessions now advance the program, which they did not before.
