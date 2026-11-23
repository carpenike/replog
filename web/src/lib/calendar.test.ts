import { describe, expect, it } from 'vitest'
import type { CalendarDay, ComplianceAthlete } from '@/api/types'
import { addDays, complianceTotals, dayTone, humanize, mondayOf, monthGrid, parseMonth, parseWeekStart, phasesOn, shiftMonth } from './calendar'

describe('date math', () => {
  it('adds days across month and year ends', () => {
    expect(addDays('2026-11-30', 1)).toBe('2026-12-01')
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28')
  })

  it('finds the Monday of a week', () => {
    expect(mondayOf('2026-11-22')).toBe('2026-11-16') // Sunday
    expect(mondayOf('2026-11-16')).toBe('2026-11-16') // Monday
    expect(mondayOf('2026-11-18')).toBe('2026-11-16')
  })

  it('shifts months', () => {
    expect(shiftMonth('2026-11', 1)).toBe('2026-12')
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
  })

  it('validates month and week params', () => {
    expect(parseMonth('2026-11')).toBe('2026-11')
    expect(parseMonth('2026-13')).toBeNull()
    expect(parseMonth(null)).toBeNull()
    expect(parseWeekStart('2026-11-16')).toBe('2026-11-16')
    expect(parseWeekStart('2026-11-17')).toBeNull() // Tuesday
    expect(parseWeekStart('2026-02-30')).toBeNull()
  })
})

describe('monthGrid', () => {
  it('lays November 2026 out Monday first with padding', () => {
    const weeks = monthGrid('2026-11')
    // Nov 1 2026 is a Sunday, so the first row is six blanks then the 1st.
    expect(weeks[0]).toEqual([null, null, null, null, null, null, '2026-11-01'])
    expect(weeks[1][0]).toBe('2026-11-02')
    expect(weeks.at(-1)).toEqual(['2026-11-30', null, null, null, null, null, null])
    expect(weeks).toHaveLength(6)
    expect(weeks.flat().filter(Boolean)).toHaveLength(30)
  })

  it('handles a February that starts on Monday', () => {
    const weeks = monthGrid('2027-02')
    expect(weeks[0][0]).toBe('2027-02-01')
    expect(weeks).toHaveLength(4)
  })
})

describe('phasesOn', () => {
  const phases = [
    { id: 1, sport: 'baseball', phase: 'pre', start_date: '2026-11-01', end_date: '2026-11-15' },
    { id: 2, sport: 'baseball', phase: 'in', start_date: '2026-11-16', end_date: null },
  ]
  it('includes start and end days and open-ended phases', () => {
    expect(phasesOn('2026-11-15', phases).map(p => p.id)).toEqual([1])
    expect(phasesOn('2026-11-16', phases).map(p => p.id)).toEqual([2])
    expect(phasesOn('2027-04-01', phases).map(p => p.id)).toEqual([2])
    expect(phasesOn('2026-10-31', phases)).toEqual([])
  })
})

describe('dayTone', () => {
  const base: CalendarDay = { date: '2026-11-09', scheduled: false, program_id: null, program_name: null, status: null, sessions: [] }
  it('uses the plan status on scheduled days', () => {
    expect(dayTone({ ...base, scheduled: true, status: 'missed' })).toBe('missed')
  })
  it('marks unscheduled days with sessions as logged', () => {
    expect(dayTone({ ...base, sessions: [{ workout_id: 1, discipline: 'throwing', label: 'bullpen', set_count: 0 }] })).toBe('logged')
    expect(dayTone(base)).toBe('none')
  })
})

describe('humanize and totals', () => {
  it('humanizes snake_case labels', () => {
    expect(humanize('long_toss')).toBe('Long toss')
    expect(humanize(null)).toBe('')
  })
  it('sums compliance counts', () => {
    const row = (done: number, planned: number, extra: number): ComplianceAthlete => ({
      athlete_id: 1, name: 'x', planned_count: planned, done_count: done, partial_count: 0,
      skipped_count: planned - done, prescribed_count: 0, extra_count: extra, summary: '', days: [],
    })
    expect(complianceTotals([row(2, 3, 1), row(1, 1, 0)])).toEqual({ planned: 4, done: 3, partial: 0, skipped: 1, extra: 1 })
  })
})
