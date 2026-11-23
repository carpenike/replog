import { describe, expect, it } from 'vitest'
import { formatFriendlyDate, formatWithLayout, parseCalendarDate, todayCalendar } from '@/lib/dates'

// Sunday, Nov 22 2026, mid-afternoon UTC.
const now = new Date('2026-11-22T15:00:00Z')

describe('parseCalendarDate', () => {
  it('reads the calendar part of dates and midnight timestamps', () => {
    expect(parseCalendarDate('2026-11-21')).toEqual({ y: 2026, m: 11, d: 21 })
    expect(parseCalendarDate('2026-11-21T00:00:00Z')).toEqual({ y: 2026, m: 11, d: 21 })
  })

  it('rejects junk', () => {
    expect(parseCalendarDate('')).toBeNull()
    expect(parseCalendarDate(null)).toBeNull()
    expect(parseCalendarDate('Nov 21')).toBeNull()
    expect(parseCalendarDate('2026-13-01')).toBeNull()
  })
})

describe('formatFriendlyDate', () => {
  const opts = { now, timeZone: 'UTC' }

  it('says Today, Yesterday and Tomorrow', () => {
    expect(formatFriendlyDate('2026-11-22', opts)).toBe('Today')
    expect(formatFriendlyDate('2026-11-21', opts)).toBe('Yesterday')
    expect(formatFriendlyDate('2026-11-23', opts)).toBe('Tomorrow')
  })

  it('uses weekday + month day this year, adds the year otherwise', () => {
    expect(formatFriendlyDate('2026-11-14', opts)).toBe('Sat, Nov 14')
    expect(formatFriendlyDate('2025-11-21', opts)).toBe('Fri, Nov 21, 2025')
  })

  it('can skip the relative words', () => {
    expect(formatFriendlyDate('2026-11-21', { ...opts, absolute: true })).toBe('Sat, Nov 21')
  })

  it('honors the date-format preference', () => {
    expect(formatFriendlyDate('2026-11-14', { ...opts, layout: '01/02/2006' })).toBe('Sat, 11/14')
    expect(formatFriendlyDate('2026-11-14', { ...opts, layout: '02/01/2006' })).toBe('Sat, 14/11')
    // ISO keeps the year so it stays unambiguous and sortable.
    expect(formatFriendlyDate('2026-11-14', { ...opts, layout: '2006-01-02' })).toBe('Sat, 2026-11-14')
    expect(formatFriendlyDate('2025-01-05', { ...opts, layout: '01/02/2006' })).toBe('Sun, 01/05/2025')
  })

  it('decides "today" in the user time zone', () => {
    // 03:00 UTC on Nov 22 is still Nov 21 in New York.
    const early = new Date('2026-11-22T03:00:00Z')
    expect(formatFriendlyDate('2026-11-21', { now: early, timeZone: 'America/New_York' })).toBe('Today')
    expect(formatFriendlyDate('2026-11-21', { now: early, timeZone: 'UTC' })).toBe('Yesterday')
  })

  it('does not shift a calendar date across midnight', () => {
    // A west-of-UTC zone must not turn "2026-11-21T00:00:00Z" into Nov 20.
    expect(formatFriendlyDate('2026-11-21T00:00:00Z', { now, timeZone: 'America/Los_Angeles', absolute: true })).toBe('Sat, Nov 21')
  })

  it('converts instants to the local day when asked', () => {
    // 02:00 UTC Nov 22 = 21:00 Nov 21 in New York.
    expect(formatFriendlyDate('2026-11-22T02:00:00Z', { now, timeZone: 'America/New_York', timestamp: true })).toBe('Yesterday')
  })

  it('handles missing and unparseable values', () => {
    expect(formatFriendlyDate(null, opts)).toBe('—')
    expect(formatFriendlyDate('soon', opts)).toBe('soon')
  })
})

describe('helpers', () => {
  it('formats with each layout', () => {
    const c = { y: 2026, m: 3, d: 9 }
    expect(formatWithLayout(c, 'Jan 2, 2006')).toBe('Mar 9, 2026')
    expect(formatWithLayout(c, '2006-01-02')).toBe('2026-03-09')
    expect(formatWithLayout(c, '01/02/2006')).toBe('03/09/2026')
    expect(formatWithLayout(c, '02/01/2006')).toBe('09/03/2026')
  })

  it('falls back to the local date for an unknown zone', () => {
    expect(todayCalendar(now, 'Not/AZone')).toEqual({ y: now.getFullYear(), m: now.getMonth() + 1, d: now.getDate() })
  })
})
