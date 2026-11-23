import { describe, expect, it } from 'vitest'
import type { ProgressExercise } from '@/api/types'
import { nearestIndex, niceTicks, scale, trendChange, trendSeries } from '@/lib/trend'

function point(id: number, date: string, e1rm: number | null, weight: number | null, reps = 5) {
  return {
    session_id: id,
    date,
    e1rm,
    top_set: { session_id: id, date, reps, load: { value: weight, unit: 'lbs' }, e1rm },
  }
}

const squat: ProgressExercise = {
  exercise_id: 1,
  name: 'Back Squat',
  sessions: [
    point(1, '2026-01-05', 233.3, 200),
    point(2, '2026-01-12', null, null, 20), // bodyweight-only session
    point(3, '2026-01-19', 245, 210),
  ],
  prs: [
    { session_id: 1, date: '2026-01-05', e1rm: 233.3, reps: 5, load: { value: 200, unit: 'lbs' } },
    { session_id: 3, date: '2026-01-19', e1rm: 245, reps: 5, load: { value: 210, unit: 'lbs' } },
  ],
}

describe('trendSeries', () => {
  it('plots e1RM per session, skipping sessions with no value', () => {
    const pts = trendSeries(squat, 'e1rm')
    expect(pts.map(p => p.value)).toEqual([233.3, 245])
    expect(pts[1].day - pts[0].day).toBe(14)
  })

  it('plots the top-set load for the top metric', () => {
    expect(trendSeries(squat, 'top').map(p => p.value)).toEqual([200, 210])
  })

  it('marks PRs after the baseline session only', () => {
    const pts = trendSeries(squat, 'e1rm')
    expect(pts.map(p => p.pr)).toEqual([false, true])
  })

  it('returns nothing without an exercise', () => {
    expect(trendSeries(undefined, 'e1rm')).toEqual([])
  })
})

describe('niceTicks', () => {
  it('covers the range with round steps', () => {
    const t = niceTicks(233, 262)
    expect(t[0]).toBeLessThanOrEqual(233)
    expect(t[t.length - 1]).toBeGreaterThanOrEqual(262)
    const step = t[1] - t[0]
    expect([5, 10, 20, 25]).toContain(step)
  })

  it('pads a flat series', () => {
    const t = niceTicks(100, 100)
    expect(t[0]).toBeLessThan(100)
    expect(t[t.length - 1]).toBeGreaterThan(100)
  })
})

describe('scale and nearestIndex', () => {
  it('maps linearly and centers a zero-width domain', () => {
    expect(scale(0, 10, 0, 100)(5)).toBe(50)
    expect(scale(3, 3, 0, 100)(3)).toBe(50)
  })

  it('finds the nearest x', () => {
    expect(nearestIndex([0, 10, 20], 14)).toBe(1)
    expect(nearestIndex([0, 10, 20], 16)).toBe(2)
    expect(nearestIndex([], 5)).toBe(-1)
  })
})

describe('trendChange', () => {
  it('is last minus first', () => {
    expect(trendChange(trendSeries(squat, 'top'))).toBe(10)
    expect(trendChange([])).toBeNull()
  })
})
