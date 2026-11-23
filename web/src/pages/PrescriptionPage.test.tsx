import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { PrescriptionData } from '@/api/types'
import { localDateISO } from '@/lib/utils'
import { PrescriptionPage } from '@/pages/PrescriptionPage'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return {
    ...actual,
    api: {
      getAthlete: vi.fn(),
      getPrescription: vi.fn(),
      createWorkout: vi.fn(),
      listWorkouts: vi.fn(),
    },
  }
})

const prescription: PrescriptionData = {
  program_name: 'Phase One',
  current_week: 4,
  current_day: 1,
  cycle_number: 1,
  num_weeks: 8,
  num_days: 3,
  day_source: 'next_after_stored',
  lines: [
    { exercise_id: 1, exercise_name: 'Back Squat', sets: [{ set_number: 1, reps: 5, rep_type: 'reps', non_working: false }] },
  ],
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter(
    [
      { path: '/athletes/:id/prescription', element: <PrescriptionPage /> },
      { path: '/athletes/:id/workouts/:workoutId/session', element: <h1>Session</h1> },
    ],
    { initialEntries: ['/athletes/7/prescription'] },
  )
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.mocked(api.getAthlete).mockResolvedValue({ id: 7, name: 'Ryan' } as never)
  vi.mocked(api.getPrescription).mockReset().mockResolvedValue(prescription)
  vi.mocked(api.createWorkout).mockReset().mockResolvedValue({ id: 99 } as never)
})

describe('PrescriptionPage', () => {
  it('starts a session linked to the program on the next day by default', async () => {
    renderPage()

    expect(await screen.findByText('Week 4, Day 1')).toBeTruthy()
    expect(screen.getByLabelText('Program day')).toBeTruthy()
    expect(api.getPrescription).toHaveBeenCalledWith(7, undefined)

    await userEvent.click(screen.getByRole('button', { name: /Start Logging/ }))

    expect(await screen.findByRole('heading', { name: 'Session' })).toBeTruthy()
    expect(api.createWorkout).toHaveBeenCalledWith(7, localDateISO(), '', false, { link: true })
  })
})

const cue = 'Bench at 75 degrees; touch the shoulder at the bottom.'

const compact: PrescriptionData = {
  ...prescription,
  lines: [
    {
      exercise_id: 1, exercise_name: 'Back Squat', training_max: 300,
      sets: [
        { set_number: 1, reps: 5, rep_type: 'standard', non_working: true, absolute_weight: 135 },
        { set_number: 2, reps: 5, rep_type: 'standard', percentage: 75, target_weight: 225 },
        { set_number: 3, reps: 5, rep_type: 'standard', percentage: 75, target_weight: 225 },
        { set_number: 4, reps: 5, rep_type: 'amrap', percentage: 85, target_weight: 255 },
      ],
    },
    {
      exercise_id: 2, exercise_name: 'High Incline DB Press',
      sets: [1, 2, 3].map(n => ({ set_number: n, reps: 5, reps_max: 8, rir_min: 1, rir_max: 2, rest_seconds: 90, rep_type: 'standard', notes: cue })),
    },
  ],
}

describe('compact prescription lines', () => {
  beforeEach(() => {
    vi.mocked(api.getPrescription).mockReset().mockResolvedValue(compact)
  })

  it('collapses matching sets into lines and keeps differing sets apart', async () => {
    renderPage()
    expect(await screen.findByText(/1 × 5 @ 135/)).toBeTruthy()
    expect(screen.getByText(/2 × 5 @ 225 .*\(75%\)/)).toBeTruthy()
    expect(screen.getByText(/1 × 5\+ @ 255 .*\(85%\)/)).toBeTruthy()
    expect(screen.getByText('Warm-up')).toBeTruthy()
  })

  it('shows a repeated cue once', async () => {
    renderPage()
    expect(await screen.findByText('3 × 5–8')).toBeTruthy()
    const card = screen.getByText('High Incline DB Press').closest('[data-slot="card"]') as HTMLElement
    expect(within(card).getAllByText(cue)).toHaveLength(1)
    expect(within(card).getByText('· 90s rest')).toBeTruthy()
  })
})
