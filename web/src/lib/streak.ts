import { api } from '@/api/client'

// Matches the backend's coach-dashboard streak (internal/models/athlete.go)
// with one athlete-friendly difference: the current week only extends the
// streak, it never breaks it — a 10-week streak shouldn't read 0 on Monday.
export const STREAK_WEEK_CAP = 26

/** Monday (local midnight) of the week containing d. */
function mondayOf(d: Date): Date {
  const m = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  const wd = (m.getDay() + 6) % 7 // Mon=0 … Sun=6
  m.setDate(m.getDate() - wd)
  return m
}

/**
 * Consecutive calendar weeks (Monday-based) with at least one workout,
 * counting back from the current week. Pure — dates are YYYY-MM-DD (a time
 * suffix is tolerated and stripped).
 */
export function weekStreak(dates: string[], today = new Date()): number {
  const currentMonday = mondayOf(today)
  const hasWorkout = new Set<number>()
  for (const raw of dates) {
    const iso = raw.split('T')[0]
    const [y, mo, day] = iso.split('-').map(Number)
    if (!y || !mo || !day) continue
    const wMonday = mondayOf(new Date(y, mo - 1, day))
    // Round absorbs DST hour drift in the day difference.
    const offset = Math.round((currentMonday.getTime() - wMonday.getTime()) / 604_800_000)
    if (offset >= 0 && offset <= STREAK_WEEK_CAP) hasWorkout.add(offset)
  }
  let streak = 0
  // Start at the current week if trained, otherwise last week (grace period).
  for (let i = hasWorkout.has(0) ? 0 : 1; i <= STREAK_WEEK_CAP; i++) {
    if (!hasWorkout.has(i)) break
    streak++
  }
  return Math.min(streak, STREAK_WEEK_CAP)
}

// 26 weeks × 7 sessions ≈ 182 workouts; 5 pages of 50 comfortably covers it.
const MAX_WORKOUT_PAGES = 5

/** Fetch recent workout dates and compute the athlete's week streak. */
export async function fetchWeekStreak(athleteId: number): Promise<number> {
  const cutoff = mondayOf(new Date())
  cutoff.setDate(cutoff.getDate() - 7 * (STREAK_WEEK_CAP + 1))
  const dates: string[] = []
  let offset = 0
  for (let page = 0; page < MAX_WORKOUT_PAGES; page++) {
    const res = await api.listWorkouts(athleteId, offset)
    for (const w of res.workouts) dates.push(w.date)
    offset += res.workouts.length
    const oldest = res.workouts[res.workouts.length - 1]
    if (!res.has_more || !oldest || new Date(oldest.date.split('T')[0]) < cutoff) break
  }
  return weekStreak(dates)
}
