# ADR 026 — Exercise Aliases and Matching

> Status: **Accepted** · Date: 2026-11-22

## Context

A coach's program names a movement however the coach writes it: "Pull Up",
"Pull-up", "Squat" for a back squat. The import (ADR 024) matched names
case-insensitively only, so a spelling difference created a second catalog
row and split the athlete's history. `exercise_aliases` already existed
(migration `0019`), but nothing could add an alias after an exercise was
created, and matching ignored punctuation only by accident.

## Decision

- **Key.** Names and aliases are compared by a key: lower case, letters and
  digits only (`models.ExerciseKey`). "Pull Up", "Pull-up", and "pullup"
  share a key. No migration: the key is computed from the catalog, which is
  small, and RepLog uses one database connection, so a check and its write
  in one transaction cannot interleave with another write.
- **Matching order.** A program name matches an exact name, then an exact
  alias, then a name key, then an alias key. A key that reaches two
  exercises is ambiguous: the preview lists the candidates and the import
  refuses with 409 until an alias or rename makes it unique. The preview
  returns `exercise_name` and `matched_on` (`name`, `alias`,
  `normalized_name`, `normalized_alias`) for each match, and up to three
  `suggestions` for each name it would create (same words ignoring a
  plural s, one name contains the other, a shared word of four or more
  letters, or close spelling). `would_create` is unchanged.
- **Uniqueness.** An alias key may not equal any other exercise's name key
  or alias key, or its own exercise's name key. A catalog create or edit
  may not take a name whose key is another exercise's alias.
- **Changes.** `POST /athletes/{id}/coach/exercises/{exerciseID}/aliases/preview`
  and `.../rename/preview` write nothing and return a one-time token
  (15 minutes) bound to the caller, athlete, exercise, and action.
  `.../confirm` with `confirm: true` re-checks and applies. Both need coach
  write access to that athlete, like creating an exercise.
- **Rename.** A rename changes `exercises.name` by id. Logged sets, PRs,
  training maxes, and programs refer to the id, so they follow the new name
  and are not rewritten. The old name is kept as an alias unless it differs
  from the new name only in case or punctuation. Renaming onto one of the
  exercise's own aliases removes that alias. A confirm refuses with 409 if
  the exercise was renamed after the preview.
- **Naming rule.** The specific name is canonical and the generic one is
  the alias: with Front Squat in the catalog, "Squat" becomes Back Squat
  with alias "Squat", not the other way round. RepLog does not decide
  this; the coach applies it through the rename and alias tools.

## Consequences

- Fewer duplicates from imports; an ambiguous legacy pair has to be fixed
  with a rename or alias before an import that names it can run.
- No merge tool: two existing rows for the same movement stay two rows.
  Merging history is a separate decision.
- Matching loads the catalog per preview or import. That is fine at
  household scale and should be revisited if the catalog grows large.
