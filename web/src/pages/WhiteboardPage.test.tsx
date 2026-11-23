import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { WhiteboardData } from '@/api/types'
import { WhiteboardPage } from '@/pages/WhiteboardPage'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return { ...actual, api: { getWhiteboard: vi.fn(), getPreferences: vi.fn().mockResolvedValue({ weight_unit: 'lbs', timezone: 'UTC', date_format: 'Jan 2, 2006' }) } }
})

const board: WhiteboardData = {
  date: '2026-11-22',
  truncated: false,
  available: [{ id: 7, name: 'Dana' }, { id: 8, name: 'Sam' }],
  athletes: [{
    athlete_id: 7, name: 'Dana', program_name: 'Base', current_week: 2, current_day: 1, workout_id: 41,
    working_target: 3, working_done: 2,
    lines: [
      {
        exercise_id: 1, exercise_name: 'Back Squat', group_key: null, working_target: 2, working_done: 2,
        prescribed: [
          { set_number: 1, reps: 5, absolute_weight: 185, rep_type: 'reps', non_working: false },
          { set_number: 2, reps: 5, absolute_weight: 185, rep_type: 'reps', non_working: false },
        ],
        logged: [
          { id: 10, workout_id: 41, exercise_id: 1, set_number: 1, reps: 5, weight: 95, rep_type: 'reps', category: 'main', set_type: 'warmup', created_at: '', updated_at: '' },
          { id: 11, workout_id: 41, exercise_id: 1, set_number: 2, reps: 5, weight: 185, rep_type: 'reps', category: 'main', set_type: 'working', created_at: '', updated_at: '' },
        ],
      },
      { exercise_id: 2, exercise_name: 'Row', group_key: null, working_target: 1, working_done: 0, prescribed: [{ set_number: 1, reps: 10, rep_type: 'reps', non_working: false }], logged: [] },
    ],
  }],
}

describe('whiteboard', () => {
  it('shows each athlete column read-only with the athletes param passed through', async () => {
    vi.mocked(api.getWhiteboard).mockResolvedValue(board)
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const router = createMemoryRouter([{ path: '/whiteboard', element: <WhiteboardPage /> }], { initialEntries: ['/whiteboard?athletes=7'] })
    render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)

    expect(await screen.findByRole('heading', { name: 'Dana' })).toBeTruthy()
    expect(api.getWhiteboard).toHaveBeenCalledWith([7])
    expect(screen.getByText('2 × 5 @ 185 lb')).toBeTruthy()
    expect(screen.getByText('Done')).toBeTruthy()
    expect(screen.getByText('0/1')).toBeTruthy()
    expect(screen.getByText('2 of 3 working sets')).toBeTruthy()
    // Read-only: no inputs on the board.
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('spinbutton')).toBeNull()
  })
})
