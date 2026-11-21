import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import {
  findWorkoutOnDate,
  openTodaySession,
  sessionPath,
  type TodaySessionApi,
} from '@/lib/todaySession'

const today = '2026-11-21'

function client(overrides: Partial<TodaySessionApi> = {}): TodaySessionApi {
  return {
    listWorkouts: vi.fn().mockResolvedValue({ workouts: [], has_more: false }),
    createWorkout: vi.fn().mockResolvedValue({ id: 99 }),
    ...overrides,
  }
}

describe('openTodaySession', () => {
  it('resumes the workout already logged today without creating another', async () => {
    const api = client({
      listWorkouts: vi.fn().mockResolvedValue({
        workouts: [{ id: 42, date: `${today}T00:00:00Z` }],
        has_more: false,
      }),
    })

    await expect(openTodaySession(api, 7, today)).resolves.toBe(sessionPath(7, 42))
    expect(api.createWorkout).not.toHaveBeenCalled()
  })

  it('creates an empty session for today when none exists', async () => {
    const api = client()

    await expect(openTodaySession(api, 7, today)).resolves.toBe(sessionPath(7, 99))
    expect(api.createWorkout).toHaveBeenCalledWith(7, today, '', false)
  })

  it('creates nothing when today is on a later page', async () => {
    const listWorkouts = vi.fn()
      .mockResolvedValueOnce({
        workouts: [{ id: 1, date: '2026-11-22' }],
        has_more: true,
      })
      .mockResolvedValueOnce({
        workouts: [{ id: 8, date: today }],
        has_more: false,
      })
    const api = client({ listWorkouts })

    await expect(openTodaySession(api, 7, today)).resolves.toBe(sessionPath(7, 8))
    expect(listWorkouts).toHaveBeenCalledTimes(2)
    expect(api.createWorkout).not.toHaveBeenCalled()
  })

  it('stops paging once a row is already older than today', async () => {
    const listWorkouts = vi.fn().mockResolvedValue({
      workouts: [{ id: 1, date: '2026-11-01' }],
      has_more: true,
    })

    await expect(findWorkoutOnDate(client({ listWorkouts }), 7, today)).resolves.toBeUndefined()
    expect(listWorkouts).toHaveBeenCalledTimes(1)
  })

  it('resumes the row that won the one-workout-per-day race', async () => {
    const listWorkouts = vi.fn()
      .mockResolvedValueOnce({ workouts: [], has_more: false })
      .mockResolvedValueOnce({
        workouts: [{ id: 15, date: today }],
        has_more: false,
      })
    const api = client({
      listWorkouts,
      createWorkout: vi.fn().mockRejectedValue(new ApiError('a resistance workout already exists for this date', 409)),
    })

    await expect(openTodaySession(api, 7, today)).resolves.toBe(sessionPath(7, 15))
  })

  it('leaves other failures alone', async () => {
    const api = client({
      createWorkout: vi.fn().mockRejectedValue(new ApiError('nope', 500)),
    })

    await expect(openTodaySession(api, 7, today)).rejects.toMatchObject({ code: 500 })
  })
})
