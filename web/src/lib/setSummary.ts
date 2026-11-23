// Compact prescription text for read-only program views: identical sets
// collapse to one line ("3 × 5–8 · RIR 1–2 · 120s rest") and a cue that
// repeats on every set is shown once. Pure so the rules are unit-tested.

export interface PrescribedSetLike {
  reps: number | null
  reps_max?: number | null
  percentage: number | null
  absolute_weight: number | null
  rest_seconds: number | null
  prescribed_rpe?: number | null
  rir_min?: number | null
  rir_max?: number | null
  non_working?: boolean
  notes: string | null
}

export interface SummaryLine {
  /** How many consecutive sets share this prescription. */
  count: number
  warmup: boolean
  /** Sets × reps and load, e.g. "3 × 5–8 @ 75%". */
  head: string
  /** The rest of the prescription, e.g. ["RIR 1–2", "120s rest"]. */
  detail: string[]
  /** head and detail joined: "3 × 5–8 @ 75% · RIR 1–2 · 120s rest" */
  text: string
}

/** "5–8", "5", or "AMRAP". */
export function formatReps(s: Pick<PrescribedSetLike, 'reps' | 'reps_max'>): string {
  if (s.reps && s.reps_max && s.reps_max > s.reps) return `${s.reps}–${s.reps_max}`
  if (s.reps) return String(s.reps)
  return 'AMRAP'
}

/** "RIR 1–2", "RIR 2", or null. */
export function formatRir(s: Pick<PrescribedSetLike, 'rir_min' | 'rir_max'>): string | null {
  if (s.rir_min != null && s.rir_max != null && s.rir_max !== s.rir_min) return `RIR ${s.rir_min}–${s.rir_max}`
  if (s.rir_min != null || s.rir_max != null) return `RIR ${s.rir_min ?? s.rir_max}`
  return null
}

/** "120s rest" or "2 min rest" for whole minutes over a minute. */
export function formatRest(seconds: number | null | undefined): string | null {
  if (seconds == null) return null
  if (seconds > 60 && seconds % 60 === 0) return `${seconds / 60} min rest`
  return `${seconds}s rest`
}

/** Fields that make two sets "the same" for display (not notes or ids). */
function signature(s: PrescribedSetLike, omitRest: boolean): string {
  return [
    s.non_working ? 1 : 0, s.reps, s.reps_max ?? null, s.percentage, s.absolute_weight,
    omitRest ? null : s.rest_seconds, s.prescribed_rpe ?? null, s.rir_min ?? null, s.rir_max ?? null,
  ].join('|')
}

function line(s: PrescribedSetLike, count: number, omitRest: boolean): SummaryLine {
  let head = `${count} × ${formatReps(s)}`
  if (s.percentage) head += ` @ ${s.percentage}%`
  else if (s.absolute_weight) head += ` @ ${s.absolute_weight}`
  const detail = [formatRir(s), s.prescribed_rpe != null ? `RPE ${s.prescribed_rpe}` : null, omitRest ? null : formatRest(s.rest_seconds)]
    .filter((p): p is string => p != null)
  return { count, warmup: !!s.non_working, head, detail, text: [head, ...detail].join(' · ') }
}

/**
 * Collapse runs of identical consecutive sets into one line each. Sets that
 * differ (a top set and back-offs, a warm-up before working sets) keep their
 * own lines, in order. omitRest drops rest when a group header shows it.
 */
export function summarizeSets(sets: PrescribedSetLike[], opts: { omitRest?: boolean } = {}): SummaryLine[] {
  const omitRest = !!opts.omitRest
  const lines: { sig: string; first: PrescribedSetLike; count: number }[] = []
  for (const s of sets) {
    const sig = signature(s, omitRest)
    const last = lines[lines.length - 1]
    if (last && last.sig === sig) last.count++
    else lines.push({ sig, first: s, count: 1 })
  }
  return lines.map(l => line(l.first, l.count, omitRest))
}

/** Distinct non-blank notes, in first-seen order. */
export function distinctNotes(sets: Pick<PrescribedSetLike, 'notes'>[]): string[] {
  const seen = new Set<string>()
  for (const s of sets) {
    const n = (s.notes ?? '').trim()
    if (n) seen.add(n)
  }
  return Array.from(seen)
}

/** The one value every item shares, or undefined when they differ or the list is empty. */
export function sharedValue<T, V>(items: T[], pick: (item: T) => V): V | undefined {
  if (items.length === 0) return undefined
  const first = pick(items[0])
  return items.every(i => pick(i) === first) ? first : undefined
}
