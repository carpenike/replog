import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { User, Workout } from '@/api/types'
import { fetchWeekStreak } from '@/lib/streak'
import { localDateISO } from '@/lib/utils'
import { Dashboard } from '@/pages/Dashboard'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return {
    ...actual,
    api: {
      dashboard: vi.fn(),
      listWorkouts: vi.fn(),
      createWorkout: vi.fn(),
      getAthleteMilestones: vi.fn(),
    },
  }
})

vi.mock('@/lib/streak', () => ({
  fetchWeekStreak: vi.fn(),
}))

function user(partial: Partial<User> & Pick<User, 'is_coach' | 'is_admin'>): User {
  return {
    id: 2,
    username: 'sam',
    name: 'Sam',
    athlete_id: null,
    mcp_enabled: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...partial,
  }
}

function workout(id: number, date: string): Workout {
  return {
    id,
    athlete_id: 7,
    date,
    created_at: '2026-11-21T00:00:00Z',
    updated_at: '2026-11-21T00:00:00Z',
    set_count: 0,
  }
}

function renderHome(u: User) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  const router = createMemoryRouter(
    [
      { path: '/', element: <Dashboard user={u} /> },
      { path: '/athletes/:id/workouts/:workoutId/session', element: <h1>Session</h1> },
    ],
    { initialEntries: ['/'] },
  )
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return router
}

beforeEach(() => {
  vi.mocked(api.dashboard).mockReset()
  vi.mocked(api.listWorkouts).mockReset()
  vi.mocked(api.createWorkout).mockReset()
  vi.mocked(api.dashboard).mockResolvedValue({ stats: null, review_stats: null, athletes: [] })
  vi.mocked(fetchWeekStreak).mockResolvedValue(0)
  vi.mocked(api.listWorkouts).mockResolvedValue({ workouts: [], has_more: false })
  vi.mocked(api.createWorkout).mockResolvedValue(workout(99, localDateISO()))
  vi.mocked(api.getAthleteMilestones).mockReset()
  vi.mocked(api.getAthleteMilestones).mockResolvedValue({
    athlete_id: 7, total_sessions: 7, current_streak: 3, best_streak: 3, pr_count: 1,
    achieved: [
      { key: 'sessions_5', kind: 'consistency', title: '5 sessions', detail: 'Logged 5 training sessions.', achieved_on: '2026-11-01' },
      { key: 'tier_foundational', kind: 'tier', title: 'Reached Foundational', detail: 'Starting tier, set by the coach.', achieved_on: '2026-10-01' },
    ],
    next: [{ key: 'sessions_10', title: '10 sessions', current: 7, target: 10 }],
  })
})

describe('linked athlete home', () => {
  const kid = user({ is_coach: false, is_admin: false, athlete_id: 7, name: 'Sam' })

  it('shows own-history milestones and the next goal', async () => {
    renderHome(kid)

    expect(await screen.findByText('Reached Foundational')).toBeTruthy()
    expect(screen.getByText('5 sessions')).toBeTruthy()
    expect(screen.getByText('Your own history')).toBeTruthy()
    expect(screen.getByRole('progressbar', { name: '10 sessions' }).getAttribute('aria-valuenow')).toBe('7')
    expect(api.getAthleteMilestones).toHaveBeenCalledWith(7)
  })

  it('opens a new session in one tap at full phone width', async () => {
    renderHome(kid)

    const open = await screen.findByRole('button', { name: "Start today's session" })
    expect(open.className).toContain('w-full')
    expect(open.className).toContain('h-14')
    expect(open.compareDocumentPosition(screen.getByRole('link', { name: "Today's plan" })))
      .toBe(Node.DOCUMENT_POSITION_FOLLOWING)

    await userEvent.click(open)

    expect(await screen.findByRole('heading', { name: 'Session' })).toBeTruthy()
    expect(api.createWorkout).toHaveBeenCalledWith(7, localDateISO(), '', false)
    expect(api.createWorkout).toHaveBeenCalledTimes(1)
  })

  it('continues today in one tap when the session already exists', async () => {
    vi.mocked(api.listWorkouts).mockResolvedValue({
      workouts: [workout(42, `${localDateISO()}T00:00:00Z`)],
      has_more: false,
    })
    renderHome(kid)

    const open = await screen.findByRole('link', { name: "Continue today's session" })
    expect(open.className).toContain('w-full')
    expect(open.className).toContain('h-14')
    expect(open.getAttribute('href')).toBe('/athletes/7/workouts/42/session')

    await userEvent.click(open)

    expect(await screen.findByRole('heading', { name: 'Session' })).toBeTruthy()
    expect(api.createWorkout).not.toHaveBeenCalled()
  })
})

describe('other homes stay put', () => {
  it('gives a coach with an athlete profile the session button and streak above the staff dashboard', async () => {
    vi.mocked(fetchWeekStreak).mockResolvedValue(3)
    renderHome(user({ is_coach: true, is_admin: false, athlete_id: 7, name: 'Coach' }))

    const open = await screen.findByRole('button', { name: "Start today's session" })
    expect(await screen.findByText(/3-week streak/)).toBeTruthy()
    const staff = await screen.findByText('No athletes yet')
    expect(open.compareDocumentPosition(staff)).toBe(Node.DOCUMENT_POSITION_FOLLOWING)

    await userEvent.click(open)

    expect(await screen.findByRole('heading', { name: 'Session' })).toBeTruthy()
    expect(api.createWorkout).toHaveBeenCalledWith(7, localDateISO(), '', false)
  })

  it('does not give a coach without an athlete profile the session control', async () => {
    renderHome(user({ is_coach: true, is_admin: true, athlete_id: null, name: 'Coach' }))

    expect(await screen.findByText('No athletes yet')).toBeTruthy()
    expect(screen.queryByRole('button', { name: /today's session/i })).toBeNull()
    expect(screen.queryByRole('link', { name: /today's session/i })).toBeNull()
  })

  it('keeps the unlinked message for an account with no athlete', async () => {
    renderHome(user({ is_coach: false, is_admin: false, athlete_id: null, name: 'Sam' }))

    expect(await screen.findByText(/hasn't been linked to an athlete profile/i)).toBeTruthy()
    expect(screen.queryByRole('button', { name: /today's session/i })).toBeNull()
  })
})
