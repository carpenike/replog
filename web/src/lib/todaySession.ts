import { ApiError } from '@/api/client'
import { formatDate } from '@/lib/utils'

/** A workout row reduced to what "is this today's session?" needs. */
export interface DatedWorkout {
  id: number
  date: string
}

export interface WorkoutListPage {
  workouts: DatedWorkout[]
  has_more?: boolean
}

/**
 * The two calls the home screen needs. Matches `api.listWorkouts` /
 * `api.createWorkout` so the real client passes through unchanged.
 */
export interface TodaySessionApi {
  listWorkouts(athleteId: number, offset?: number): Promise<WorkoutListPage>
  createWorkout(
    athleteId: number,
    date: string,
    notes?: string,
    fromPrescription?: boolean,
    program?: { link?: boolean; week?: number; day?: number },
  ): Promise<{ id: number }>
}

/** Live logging screen for one resistance workout. */
export function sessionPath(athleteId: number, workoutId: number): string {
  return `/athletes/${athleteId}/workouts/${workoutId}/session`
}

export function workoutOnDate(workouts: DatedWorkout[], date: string): DatedWorkout | undefined {
  return workouts.find(w => formatDate(w.date) === date)
}

// List pages are 50 rows, newest first. Five pages covers a long history
// without walking the whole log when today is simply absent.
const MAX_PAGES = 5

/**
 * Find the resistance workout on `date` (YYYY-MM-DD). Pages are newest-first,
 * so a page whose oldest row is already before `date` ends the search.
 */
export async function findWorkoutOnDate(
  client: TodaySessionApi,
  athleteId: number,
  date: string,
): Promise<DatedWorkout | undefined> {
  let offset = 0
  for (let page = 0; page < MAX_PAGES; page++) {
    const res = await client.listWorkouts(athleteId, offset)
    const hit = workoutOnDate(res.workouts, date)
    if (hit) return hit
    const oldest = res.workouts[res.workouts.length - 1]
    if (!res.has_more || !oldest || formatDate(oldest.date) < date) return undefined
    offset += res.workouts.length
  }
  return undefined
}

/**
 * Resume today's resistance workout, or create an empty one and open it.
 * `fromPrescription` stays false: seeding every prescribed set would record
 * work the athlete has not done. The session is linked to today's program
 * so it stores (and advances) the program day (ADR 028). A 409 means the day's row appeared between
 * the read and the insert (one workout per athlete per day) — resume it.
 */
export async function openTodaySession(
  client: TodaySessionApi,
  athleteId: number,
  today: string,
): Promise<string> {
  const existing = await findWorkoutOnDate(client, athleteId, today)
  if (existing) return sessionPath(athleteId, existing.id)
  try {
    const created = await client.createWorkout(athleteId, today, '', false, { link: true })
    return sessionPath(athleteId, created.id)
  } catch (err) {
    if (!(err instanceof ApiError) || err.code !== 409) throw err
    const raced = await findWorkoutOnDate(client, athleteId, today)
    if (!raced) throw err
    return sessionPath(athleteId, raced.id)
  }
}
