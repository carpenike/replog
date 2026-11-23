import { describe, expect, it } from 'vitest'
import { nextTier, promoteCopy, tierLabel } from '@/lib/tiers'

describe('tiers', () => {
  it('walks foundational → intermediate → sport performance and stops', () => {
    expect(nextTier('foundational')).toBe('intermediate')
    expect(nextTier('intermediate')).toBe('sport_performance')
    expect(nextTier('sport_performance')).toBeNull()
    expect(nextTier('unknown')).toBeNull()
    expect(nextTier(null)).toBeNull()
  })

  it('labels tiers and passes unknown values through', () => {
    expect(tierLabel('sport_performance')).toBe('Sport Performance')
    expect(tierLabel('custom')).toBe('custom')
  })

  it('writes from → to copy for the promote confirm', () => {
    const copy = promoteCopy('Sam', 'foundational')
    expect(copy?.title).toBe('Promote to Intermediate?')
    expect(copy?.description).toContain('from Foundational to Intermediate')
    expect(copy?.confirmLabel).toBe('Promote to Intermediate')
    expect(promoteCopy('Sam', 'sport_performance')).toBeNull()
  })
})
