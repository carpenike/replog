// Whiteboard / TV mode helpers (gap 15). Pure so the board's layout rules
// are unit-tested: which athletes are on the board, how a line's
// prescription and logged sets are summarized in large type, and how
// superset groups are labelled.
import type { PrescriptionSetData, WhiteboardLine, WorkoutSet } from '@/api/types'
import { groupBlocks, memberLabel, type GroupBlock } from '@/lib/groups'
import { repTargetLabel, effortLabel } from '@/lib/session'
import { formatWeight } from '@/lib/utils'

/** The server's cap on one board. */
export const WHITEBOARD_MAX_ATHLETES = 8

/** Parse `?athletes=1,2` into unique positive IDs (at most 8). */
export function parseAthleteIds(param: string | null | undefined): number[] {
  if (!param) return []
  const out: number[] = []
  for (const part of param.split(',')) {
    const n = Number(part.trim())
    if (Number.isInteger(n) && n > 0 && !out.includes(n)) out.push(n)
    if (out.length === WHITEBOARD_MAX_ATHLETES) break
  }
  return out
}

/** Add or remove an athlete, keeping the cap. */
export function toggleAthlete(ids: number[], id: number): number[] {
  if (ids.includes(id)) return ids.filter(x => x !== id)
  if (ids.length >= WHITEBOARD_MAX_ATHLETES) return ids
  return [...ids, id]
}

function setWeight(s: PrescriptionSetData): number | null {
  return s.absolute_weight ?? s.target_weight ?? null
}

/**
 * Prescribed working sets compressed for large type: runs of identical sets
 * become "3 × 5 @ 185", joined with " · ". Warm-up (non-working) sets are
 * left out of the runs and counted separately.
 */
export function prescribedSummary(sets: PrescriptionSetData[], unit = ''): { runs: string[]; warmups: number } {
  const runs: { label: string; count: number }[] = []
  let warmups = 0
  for (const s of sets) {
    if (s.non_working) {
      warmups++
      continue
    }
    const reps = repTargetLabel(s) ?? '—'
    const w = setWeight(s)
    const effort = effortLabel(s)
    let label = reps
    if (w != null && w > 0) label += ` @ ${formatWeight(w)}${unit ? ` ${unit}` : ''}`
    else if (s.percentage != null) label += ` @ ${s.percentage}%`
    if (effort) label += ` (${effort})`
    const last = runs[runs.length - 1]
    if (last && last.label === label) last.count++
    else runs.push({ label, count: 1 })
  }
  return { runs: runs.map(r => (r.count > 1 ? `${r.count} × ${r.label}` : `1 × ${r.label}`)), warmups }
}

/** One logged set as a chip: "185×5", "×12" for bodyweight, warm-ups flagged. */
export function loggedChip(s: Pick<WorkoutSet, 'reps' | 'weight' | 'set_type' | 'reps_label'>): { text: string; warmup: boolean } {
  const reps = s.reps_label ?? String(s.reps)
  const text = s.weight != null && s.weight > 0 ? `${formatWeight(s.weight)}×${reps}` : `×${reps}`
  return { text, warmup: s.set_type === 'warmup' }
}

export type LineStatus = 'done' | 'active' | 'todo' | 'extra'

/** Where a line stands: all working sets in, some, none, or not prescribed. */
export function lineStatus(line: Pick<WhiteboardLine, 'working_target' | 'working_done'>): LineStatus {
  if (line.working_target === 0) return 'extra'
  if (line.working_done >= line.working_target) return 'done'
  return line.working_done > 0 ? 'active' : 'todo'
}

export interface BoardBlock {
  key: string | null
  lines: { line: WhiteboardLine; label: string | null }[]
}

/**
 * Lines in board order with superset labels: grouped lines share a block
 * and are labelled A1, A2, …; a group of one is shown as a plain line.
 */
export function boardBlocks(lines: WhiteboardLine[]): BoardBlock[] {
  const blocks: GroupBlock<WhiteboardLine>[] = groupBlocks(lines, l => l.group_key)
  return blocks.map(b => {
    const grouped = b.key != null && b.items.length > 1
    return {
      key: grouped ? b.key : null,
      lines: b.items.map((line, i) => ({ line, label: grouped ? memberLabel(b.key as string, i) : null })),
    }
  })
}

/** 0–100 progress for an athlete column; 0 when nothing is prescribed. */
export function progressPercent(done: number, target: number): number {
  if (target <= 0) return 0
  return Math.min(100, Math.round((done / target) * 100))
}
