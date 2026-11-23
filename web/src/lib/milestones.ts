// Pure helpers for kid milestones. Milestones compare an athlete only with
// their own history — nothing here ranks one athlete against another.
import type { MilestoneData, MilestoneGoalData } from '@/api/types'
import { parseCalendarDate } from '@/lib/dates'

/** Badges earned within this many days get a "New" marker. */
export const NEW_MILESTONE_DAYS = 14

function dayNumber(iso: string): number | null {
  const c = parseCalendarDate(iso)
  return c ? Math.round(Date.UTC(c.y, c.m - 1, c.d) / 86_400_000) : null
}

/** Whether a milestone was earned within NEW_MILESTONE_DAYS of today (YYYY-MM-DD). */
export function isNewMilestone(m: MilestoneData, today: string): boolean {
  const a = dayNumber(m.achieved_on)
  const t = dayNumber(today)
  if (a == null || t == null) return false
  return t - a >= 0 && t - a < NEW_MILESTONE_DAYS
}

/** Progress toward a goal as a whole percent, clamped to 0–100. */
export function goalPercent(goal: MilestoneGoalData): number {
  if (goal.target <= 0) return 0
  return Math.max(0, Math.min(100, Math.round((goal.current / goal.target) * 100)))
}

/** "3 more sessions", "1 more week", "2 more PRs" — the gap to a goal. */
export function goalRemaining(goal: MilestoneGoalData): string {
  const left = Math.max(goal.target - goal.current, 0)
  const unit = goal.key.startsWith('streak_') ? 'week' : goal.key.startsWith('prs_') ? 'PR' : 'session'
  return `${left} more ${unit}${left === 1 ? '' : 's'}`
}

/**
 * The badges to show first: tier badges always, then the newest of the
 * rest, up to `limit` in all, keeping newest-first order.
 */
export function featuredMilestones(achieved: MilestoneData[], limit: number): MilestoneData[] {
  if (achieved.length <= limit) return achieved
  const keep = new Set<MilestoneData>()
  for (const m of achieved) if (m.kind === 'tier' && keep.size < limit) keep.add(m)
  for (const m of achieved) if (keep.size < limit) keep.add(m)
  return achieved.filter(m => keep.has(m))
}
