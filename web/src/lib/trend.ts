// Pure helpers for the exercise trend chart: series extraction, time-scaled
// x positions and "nice" y ticks. No React so the math stays unit-testable.
import type { ProgressExercise } from '@/api/types'
import { parseCalendarDate } from '@/lib/dates'

export type TrendMetric = 'e1rm' | 'top'

export interface TrendPoint {
  date: string
  /** Days since the epoch, for a time-proportional x axis. */
  day: number
  value: number
  /** The set behind the value: e1RM's top set or the top set itself. */
  reps: number
  weight: number | null
  /** This session set a new estimated-1RM best (after the first session). */
  pr: boolean
}

function dayOf(iso: string): number | null {
  const c = parseCalendarDate(iso)
  return c ? Math.round(Date.UTC(c.y, c.m - 1, c.d) / 86_400_000) : null
}

/**
 * One point per session. 'e1rm' plots the session's best estimated 1RM;
 * 'top' plots the top set's load. Sessions without a value (bodyweight-only)
 * are skipped. PR flags mark sessions that beat every earlier session; the
 * first session is the baseline, not a PR.
 */
export function trendSeries(exercise: ProgressExercise | undefined, metric: TrendMetric): TrendPoint[] {
  if (!exercise) return []
  const prSessions = new Set(exercise.prs.slice(1).map(p => p.session_id))
  const out: TrendPoint[] = []
  for (const s of exercise.sessions) {
    const day = dayOf(s.date)
    if (day == null) continue
    const value = metric === 'e1rm' ? s.e1rm : s.top_set.load.value
    if (value == null || !Number.isFinite(value) || value <= 0) continue
    out.push({
      date: s.date.slice(0, 10),
      day,
      value,
      reps: s.top_set.reps,
      weight: s.top_set.load.value,
      pr: prSessions.has(s.session_id),
    })
  }
  return out
}

/** Round a step up to 1, 2, 2.5 or 5 × 10^n. */
function niceStep(raw: number): number {
  if (!(raw > 0)) return 1
  const pow = Math.pow(10, Math.floor(Math.log10(raw)))
  const f = raw / pow
  const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 2.5 ? 2.5 : f <= 5 ? 5 : 10
  return nice * pow
}

/**
 * About `count` evenly spaced, round y ticks that cover [min, max]. A flat
 * series gets a small band around its value so the line sits mid-chart.
 */
export function niceTicks(min: number, max: number, count = 4): number[] {
  if (!Number.isFinite(min) || !Number.isFinite(max)) return []
  let lo = min
  let hi = max
  if (hi - lo < 1e-9) {
    const pad = Math.max(Math.abs(lo) * 0.05, 5)
    lo -= pad
    hi += pad
  }
  const step = niceStep((hi - lo) / Math.max(count - 1, 1))
  const start = Math.floor(lo / step) * step
  const end = Math.ceil(hi / step) * step
  const ticks: number[] = []
  for (let v = start; v <= end + step / 2; v += step) ticks.push(Number(v.toFixed(6)))
  return ticks
}

/** Linear map from [d0, d1] onto [r0, r1]; a zero-width domain maps to the midpoint. */
export function scale(d0: number, d1: number, r0: number, r1: number) {
  return (v: number) => (d1 === d0 ? (r0 + r1) / 2 : r0 + ((v - d0) / (d1 - d0)) * (r1 - r0))
}

/** Index of the point whose x is nearest to `x` (points sorted by x). */
export function nearestIndex(xs: number[], x: number): number {
  if (xs.length === 0) return -1
  let best = 0
  for (let i = 1; i < xs.length; i++) {
    if (Math.abs(xs[i] - x) < Math.abs(xs[best] - x)) best = i
  }
  return best
}

/** Change from the first to the last point, or null with fewer than two. */
export function trendChange(points: TrendPoint[]): number | null {
  if (points.length < 2) return null
  return points[points.length - 1].value - points[0].value
}
