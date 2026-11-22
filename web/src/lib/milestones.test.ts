import { describe, expect, it } from 'vitest'
import type { MilestoneData } from '@/api/types'
import { featuredMilestones, goalPercent, goalRemaining, isNewMilestone } from '@/lib/milestones'

const m = (key: string, kind: MilestoneData['kind'], achieved_on: string): MilestoneData =>
  ({ key, kind, title: key, detail: '', achieved_on })

describe('milestones helpers', () => {
  it('flags badges earned in the last two weeks', () => {
    expect(isNewMilestone(m('a', 'first', '2026-11-20'), '2026-11-22')).toBe(true)
    expect(isNewMilestone(m('a', 'first', '2026-11-08'), '2026-11-22')).toBe(false)
    expect(isNewMilestone(m('a', 'first', '2026-11-30'), '2026-11-22')).toBe(false)
  })

  it('computes clamped goal progress and the remaining gap', () => {
    expect(goalPercent({ key: 'sessions_10', title: '', current: 7, target: 10 })).toBe(70)
    expect(goalPercent({ key: 'sessions_10', title: '', current: 12, target: 10 })).toBe(100)
    expect(goalRemaining({ key: 'sessions_10', title: '', current: 7, target: 10 })).toBe('3 more sessions')
    expect(goalRemaining({ key: 'streak_4', title: '', current: 3, target: 4 })).toBe('1 more week')
    expect(goalRemaining({ key: 'prs_5', title: '', current: 3, target: 5 })).toBe('2 more PRs')
  })

  it('keeps tier badges when trimming to the newest', () => {
    const list = [
      m('sessions_25', 'consistency', '2026-11-20'),
      m('streak_4', 'consistency', '2026-11-10'),
      m('first_pr', 'first', '2026-10-01'),
      m('tier_foundational', 'tier', '2026-01-01'),
    ]
    expect(featuredMilestones(list, 2).map(x => x.key)).toEqual(['sessions_25', 'tier_foundational'])
    expect(featuredMilestones(list, 10)).toHaveLength(4)
  })
})
