import { describe, expect, it } from 'vitest'
import { latestDate, parseAthleteTab, splitModalities } from '@/lib/athleteTabs'

describe('athlete tabs', () => {
  it('parses known tabs and falls back to overview', () => {
    expect(parseAthleteTab('program')).toBe('program')
    expect(parseAthleteTab('nope')).toBe('overview')
    expect(parseAthleteTab(null)).toBe('overview')
  })

  it('features modalities with data plus recovery', () => {
    const { featured, others, showLoad } = splitModalities([
      { key: 'throwing', count: 4, last: '2026-11-20' },
      { key: 'conditioning', count: 0 },
      { key: 'skill', count: 0 },
      { key: 'recovery', count: 0 },
    ])
    expect(featured.map(m => m.key)).toEqual(['throwing', 'recovery'])
    expect(others.map(m => m.key)).toEqual(['conditioning', 'skill'])
    expect(showLoad).toBe(true)
  })

  it('hides the load dashboard when only recovery has data', () => {
    expect(splitModalities([
      { key: 'throwing', count: 0 },
      { key: 'recovery', count: 3 },
    ]).showLoad).toBe(false)
  })

  it('finds the latest date', () => {
    expect(latestDate([{ date: '2026-01-02' }, { date: '2026-03-01T00:00:00Z' }, { date: '2026-02-01' }])).toBe('2026-03-01')
    expect(latestDate([])).toBeUndefined()
  })
})
