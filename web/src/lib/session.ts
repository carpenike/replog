// Pure helpers for the live session screen (ActiveWorkout) and the
// prescription page: building sections from the prescription and the log,
// mapping logged sets onto rows, warm-up rows, superset rounds, and the
// rep / RIR target labels. No React or API calls so the rules are tested.

import type { ExerciseGroup, ExerciseSwap, PrescriptionLineData, PrescriptionSetData, WorkoutSet } from '@/api/types'
import { isWarmup } from './records'
import { normalizeGroupKey } from './groups'

/** A warm-up target row: from an imported non-working set or a generated ramp. */
export interface WarmupTarget {
  label: string
  reps?: number | null
  weight?: number | null
  rest_seconds?: number | null
  notes?: string | null
}

export interface Section {
  exerciseId: number
  name: string
  trainingMax?: number | null
  /** Superset/circuit key (A, B, ...), or null. */
  groupKey: string | null
  /** Working prescribed sets (warm-ups excluded). */
  prescribed: PrescriptionSetData[]
  /** Prescribed warm-up (non-working) sets, e.g. from a coach import. */
  prescribedWarmups: PrescriptionSetData[]
  /** Logged working sets. */
  logged: WorkoutSet[]
  /** Logged warm-up sets. */
  loggedWarmups: WorkoutSet[]
  /** Set when the athlete swapped the prescribed exercise for this one. */
  swap?: ExerciseSwap
  /** The prescribed exercise id, which differs from exerciseId after a swap. */
  prescribedExerciseId?: number
}

export type Row =
  | { kind: 'done'; index: number; set: WorkoutSet; prescribed?: PrescriptionSetData }
  | { kind: 'pending'; index: number; prescribed?: PrescriptionSetData }

export type WarmupRow =
  | { kind: 'done'; index: number; set: WorkoutSet; target?: WarmupTarget }
  | { kind: 'pending'; index: number; target: WarmupTarget }

/** A swapped-in lift keeps the prescribed reps but not loads computed for another lift. */
function retargetForSwap(s: PrescriptionSetData): PrescriptionSetData {
  return { ...s, target_weight: null, absolute_weight: null, percentage: null }
}

/**
 * Sections in prescription order, then logged-only exercises, then
 * exercises added this session. A repeated line for the same exercise
 * merges into one section. A swap replaces the prescribed exercise with
 * the substitute; the program itself is unchanged.
 */
export function buildSections(
  lines: PrescriptionLineData[],
  groups: ExerciseGroup[],
  swaps: ExerciseSwap[],
  added: { id: number; name: string }[] = [],
): Section[] {
  const groupsByEx = new Map(groups.map(g => [g.exercise_id, g]))
  const swapByFrom = new Map(swaps.map(s => [s.from_exercise_id, s]))
  const out: Section[] = []
  const seen = new Set<number>()
  const split = (sets: WorkoutSet[]) => ({
    logged: sets.filter(s => !isWarmup(s)),
    loggedWarmups: sets.filter(isWarmup),
  })
  for (const line of lines) {
    const swap = swapByFrom.get(line.exercise_id)
    const exerciseId = swap ? swap.to_exercise_id : line.exercise_id
    const sets = swap ? line.sets.map(retargetForSwap) : line.sets
    const working = sets.filter(s => !s.non_working)
    const warmups = sets.filter(s => s.non_working)
    const existing = out.find(s => s.exerciseId === exerciseId)
    if (existing) {
      existing.prescribed = existing.prescribed.concat(working)
      existing.prescribedWarmups = existing.prescribedWarmups.concat(warmups)
      continue
    }
    seen.add(exerciseId)
    out.push({
      exerciseId,
      name: swap ? swap.to_exercise_name : line.exercise_name,
      trainingMax: swap ? null : line.training_max,
      groupKey: normalizeGroupKey(line.group_key ?? line.sets.find(s => s.group_key)?.group_key),
      prescribed: working,
      prescribedWarmups: warmups,
      ...split(groupsByEx.get(exerciseId)?.sets ?? []),
      swap,
      prescribedExerciseId: line.exercise_id,
    })
  }
  for (const g of groups) {
    if (seen.has(g.exercise_id)) continue
    seen.add(g.exercise_id)
    out.push({ exerciseId: g.exercise_id, name: g.exercise_name, groupKey: null, prescribed: [], prescribedWarmups: [], ...split(g.sets) })
  }
  for (const ex of added) {
    if (seen.has(ex.id)) continue
    seen.add(ex.id)
    out.push({ exerciseId: ex.id, name: ex.name, groupKey: null, prescribed: [], prescribedWarmups: [], logged: [], loggedWarmups: [] })
  }
  return out
}

/**
 * Map logged working sets onto display rows. Sets explicitly claimed by a
 * row (the athlete checked that row this session) keep their slot even when
 * completed out of order; unclaimed sets (e.g. after a reload) fill
 * remaining prescribed slots in set_number order, and anything left over
 * becomes a surplus row.
 */
export function buildRows(section: Section, claims: Record<number, number>, extra: number): Row[] {
  const n = section.prescribed.length
  const slots = new Map<number, WorkoutSet>()
  const unclaimed: WorkoutSet[] = []
  const surplus: WorkoutSet[] = []
  const logged = [...section.logged].sort((a, b) => a.set_number - b.set_number)
  for (const s of logged) {
    const idx = claims[s.id]
    if (idx !== undefined && idx < n && !slots.has(idx)) slots.set(idx, s)
    else unclaimed.push(s)
  }
  let cursor = 0
  for (const s of unclaimed) {
    while (cursor < n && slots.has(cursor)) cursor++
    if (cursor < n) slots.set(cursor++, s)
    else surplus.push(s)
  }
  const rows: Row[] = []
  for (let i = 0; i < n; i++) {
    const set = slots.get(i)
    rows.push(set
      ? { kind: 'done', index: i, set, prescribed: section.prescribed[i] }
      : { kind: 'pending', index: i, prescribed: section.prescribed[i] })
  }
  surplus.forEach((s, j) => rows.push({ kind: 'done', index: n + j, set: s }))
  for (let k = 0; k < extra; k++) rows.push({ kind: 'pending', index: n + surplus.length + k })
  return rows
}

/** Warm-up target for an imported non-working prescribed set. */
export function warmupTargetFromPrescribed(s: PrescriptionSetData): WarmupTarget {
  return {
    label: 'Warm-up',
    reps: s.reps,
    weight: s.target_weight ?? s.absolute_weight ?? null,
    rest_seconds: s.rest_seconds,
    notes: s.notes,
  }
}

/**
 * Warm-up rows: prescribed warm-ups first, then any generated ramp. Logged
 * warm-ups fill the targets in order; extras show as done rows after them.
 */
export function buildWarmupRows(section: Section, generated: WarmupTarget[]): WarmupRow[] {
  const targets = [...section.prescribedWarmups.map(warmupTargetFromPrescribed), ...generated]
  const logged = [...section.loggedWarmups].sort((a, b) => a.set_number - b.set_number)
  const rows: WarmupRow[] = []
  const n = Math.max(targets.length, logged.length)
  for (let i = 0; i < n; i++) {
    const set = logged[i]
    rows.push(set ? { kind: 'done', index: i, set, target: targets[i] } : { kind: 'pending', index: i, target: targets[i] })
  }
  return rows
}

/** Working weight the warm-up ramp climbs to: the first pending working set's target. */
export function firstWorkingTarget(section: Section): number | null {
  for (const s of section.prescribed) {
    const w = s.target_weight ?? s.absolute_weight
    if (w != null && w > 0) return w
  }
  return null
}

export interface RoundCell<T> {
  member: number
  round: number
  row: T
}

/**
 * Interleave per-member rows round by round for a superset/circuit card:
 * A1 set 1, A2 set 1, A1 set 2, A2 set 2, ... A member with fewer rows is
 * skipped in the later rounds.
 */
export function interleaveRounds<T>(memberRows: T[][]): RoundCell<T>[] {
  const rounds = Math.max(0, ...memberRows.map(r => r.length))
  const out: RoundCell<T>[] = []
  for (let round = 0; round < rounds; round++) {
    memberRows.forEach((rows, member) => {
      if (round < rows.length) out.push({ member, round, row: rows[round] })
    })
  }
  return out
}

/**
 * Whether logging `member`'s row in `round` finishes that round of the
 * group, so the one rest timer per round should start. Members with no row
 * in that round don't hold it open.
 */
export function roundComplete(memberRows: { kind: 'done' | 'pending' }[][], member: number, round: number): boolean {
  return memberRows.every((rows, m) => m === member || round >= rows.length || rows[round].kind === 'done')
}

/** Working-set progress for the header: warm-ups never count. */
export function sessionProgress(sections: Section[]): { total: number; done: number; logged: number } {
  let total = 0
  let done = 0
  let logged = 0
  for (const s of sections) {
    total += s.prescribed.length
    done += Math.min(s.logged.length, s.prescribed.length)
    logged += s.logged.length
  }
  return { total, done, logged }
}

/** "8–12", "5+", "5", or null when no rep target is set. */
export function repTargetLabel(s: Pick<PrescriptionSetData, 'reps' | 'reps_max' | 'rep_type'>): string | null {
  if (s.reps == null) return s.rep_type === 'amrap' ? 'AMRAP' : null
  if (s.reps_max != null && s.reps_max > s.reps) return `${s.reps}–${s.reps_max}`
  return s.rep_type === 'amrap' ? `${s.reps}+` : String(s.reps)
}

/** "RIR 2–3", "RIR 2", "RPE 8", or null. */
export function effortLabel(s: Pick<PrescriptionSetData, 'rir_min' | 'rir_max' | 'prescribed_rpe'>): string | null {
  const lo = s.rir_min ?? s.rir_max
  const hi = s.rir_max ?? s.rir_min
  if (lo != null && hi != null) return lo === hi ? `RIR ${lo}` : `RIR ${Math.min(lo, hi)}–${Math.max(lo, hi)}`
  if (s.prescribed_rpe != null) return `RPE ${s.prescribed_rpe}`
  return null
}
