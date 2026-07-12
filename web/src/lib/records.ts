// Pure personal-record helpers. No API or React dependencies so the logic
// stays unit-testable and obvious.

/** Minimal set shape shared by WorkoutSet and ExerciseHistoryEntryData. */
export interface RecordSet {
  reps: number
  weight?: number | null
}

/** A set with the date it was performed, for computing all-time bests. */
export interface DatedSet extends RecordSet {
  date: string
}

export type PRKind = 'weight' | 'e1rm' | 'reps-at-weight' | 'reps'

export interface PRResult {
  kind: PRKind
  /** New best: weight for 'weight', estimated 1RM for 'e1rm', reps otherwise. */
  value: number
  /** Previous best of the same kind, when one existed. */
  prev?: number
  reps: number
  weight?: number | null
}

/** Epley estimated 1RM: weight × (1 + reps/30). */
export function epley(weight: number, reps: number): number {
  return weight * (1 + reps / 30)
}

// Estimates from very high-rep sets are junk; only sets in this rep range
// participate in e1RM comparisons (both the new set and the prior baseline).
const E1RM_MAX_REPS = 12

function isWeighted(s: RecordSet): boolean {
  return s.weight != null && s.weight > 0 && s.reps >= 1
}

function isBodyweight(s: RecordSet): boolean {
  return (s.weight == null || s.weight === 0) && s.reps >= 1
}

/**
 * Classify a just-logged set against all prior sets for the same exercise.
 * Returns null when the set is not a PR — including when there is no prior
 * history at all (a first-ever set is not a record). When a set qualifies on
 * multiple counts the most impressive kind wins: weight > e1rm > reps.
 */
export function classifyPR(prior: RecordSet[], set: RecordSet): PRResult | null {
  if (set.reps < 1) return null

  // Bodyweight sets only compete on plain max reps against other BW sets.
  if (!isWeighted(set)) {
    const bw = prior.filter(isBodyweight)
    if (bw.length === 0) return null
    const best = Math.max(...bw.map(s => s.reps))
    return set.reps > best
      ? { kind: 'reps', value: set.reps, prev: best, reps: set.reps, weight: null }
      : null
  }

  const weighted = prior.filter(isWeighted)
  if (weighted.length === 0) return null
  const weight = set.weight as number

  const bestWeight = Math.max(...weighted.map(s => s.weight as number))
  if (weight > bestWeight) {
    return { kind: 'weight', value: weight, prev: bestWeight, reps: set.reps, weight }
  }

  if (set.reps <= E1RM_MAX_REPS) {
    const priorEstimates = weighted
      .filter(s => s.reps <= E1RM_MAX_REPS)
      .map(s => epley(s.weight as number, s.reps))
    const est = epley(weight, set.reps)
    const bestEst = priorEstimates.length > 0 ? Math.max(...priorEstimates) : -Infinity
    if (est > bestEst) {
      return {
        kind: 'e1rm',
        value: est,
        prev: priorEstimates.length > 0 ? bestEst : undefined,
        reps: set.reps,
        weight,
      }
    }
  }

  const atWeight = weighted.filter(s => (s.weight as number) >= weight)
  if (atWeight.length > 0) {
    const bestReps = Math.max(...atWeight.map(s => s.reps))
    if (set.reps > bestReps) {
      return { kind: 'reps-at-weight', value: set.reps, prev: bestReps, reps: set.reps, weight }
    }
  }

  return null
}

export interface ExerciseBests {
  heaviest?: { weight: number; reps: number; date: string }
  bestE1rm?: { e1rm: number; weight: number; reps: number; date: string }
  mostReps?: { reps: number; weight?: number | null; date: string }
}

/**
 * All-time bests across a full exercise history. Ties keep the earliest date
 * (the day the record was first set). Dates are YYYY-MM-DD strings, so string
 * comparison orders them.
 */
export function computeBests(sets: DatedSet[]): ExerciseBests {
  const out: ExerciseBests = {}
  for (const s of sets) {
    if (s.reps < 1) continue
    if (isWeighted(s)) {
      const w = s.weight as number
      const h = out.heaviest
      if (!h || w > h.weight || (w === h.weight && s.date < h.date)) {
        out.heaviest = { weight: w, reps: s.reps, date: s.date }
      }
      if (s.reps <= E1RM_MAX_REPS) {
        const est = epley(w, s.reps)
        const b = out.bestE1rm
        if (!b || est > b.e1rm || (est === b.e1rm && s.date < b.date)) {
          out.bestE1rm = { e1rm: est, weight: w, reps: s.reps, date: s.date }
        }
      }
    }
    const m = out.mostReps
    if (!m || s.reps > m.reps || (s.reps === m.reps && s.date < m.date)) {
      out.mostReps = { reps: s.reps, weight: s.weight, date: s.date }
    }
  }
  return out
}
