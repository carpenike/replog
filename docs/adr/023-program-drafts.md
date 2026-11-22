# ADR 023 — Program Drafts

> Status: **Accepted** · Date: 2026-11-21

## Context

A coach service token (ADR 022) can log a set. It must not assign a
program or otherwise make the coaching decision. The connector still
needs a place to put a proposed program — the notes, the sets, where
the idea came from — that a coach reviews in the browser before it
becomes something an athlete can be given (ADR 007).

Program templates today are approved the moment they are inserted.
Reference context (`buildReferencePrograms`) and assignment treat every
template as a plan.

## Decision

- Migration `0018` adds `program_templates.status` (`draft` or
  `approved`, default `approved`), nullable `lineage_id`, and `version`
  (default 1). Existing rows stay approved. `lineage_id` NULL is the
  first version. A later version points `lineage_id` at that root and
  takes the next version. A row does not store its own id in
  `lineage_id`.
- `program_sources` stores the coach, the date, and the source text for
  a proposal. Deleting the template cascades. Deleting the coach sets
  `coach_id` NULL and keeps the text.
- `prescribed_sets.prescribed_rpe` (NULL, or 1 through 10) and
  `group_label` (NULL or text) are optional prescription fields.
- Approved names stay unique per version in their scope (migration
  `0019` includes `version` in the partial unique index, so a second
  import of the same name can be stored). A draft may reuse a name.
  Approving a draft onto the same name and version that is already
  approved is refused. `(COALESCE(lineage_id, id), version)` is unique.
- `buildReferencePrograms` and assignment skip drafts. The coach's
  program list still returns them. Approving sets `status` to
  `approved` and does not assign the program.
- `POST /programs/drafts` writes a draft. A coach service token may call
  it. `POST /programs/{id}/approve` and `POST /programs` (an approved
  template) return 403 for that token. A browser session of the same
  coach can approve.

## Consequences

- A proposal can sit in the logbook with its source text until a person
  approves it. The app does not assign it and does not treat it as a
  reference example.
- Generation execute is unchanged: that click is already the coach's
  approval, and the imported template stays `approved` and unassigned.
- No image workflow and no kid home screen in this change.
