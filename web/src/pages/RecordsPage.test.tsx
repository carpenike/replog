import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { RecordsPage } from '@/pages/RecordsPage'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return { ...actual, api: { getAthlete: vi.fn(), getAthleteRecords: vi.fn() } }
})

describe('records board', () => {
  it('lists rep maxes and the estimated 1RM per lift', async () => {
    vi.mocked(api.getAthlete).mockResolvedValue({ id: 7, name: 'Sam', track_body_weight: false, created_at: '', updated_at: '' })
    vi.mocked(api.getAthleteRecords).mockResolvedValue({
      athlete_id: 7, rep_targets: [1, 2, 3, 5, 8, 10, 12], e1rm_source: 'epley', e1rm_max_reps: 12,
      exercises: [{
        exercise_id: 3, name: 'Back Squat', session_count: 4, last_date: '2026-11-20',
        heaviest: { workout_id: 1, date: '2026-11-20', reps: 1, weight: 250 },
        best_e1rm: { workout_id: 2, date: '2026-11-13', reps: 6, weight: 210, e1rm: 252 },
        most_reps: { workout_id: 2, date: '2026-11-13', reps: 6, weight: 210 },
        rep_maxes: [
          { target: 1, workout_id: 1, date: '2026-11-20', reps: 1, weight: 250 },
          { target: 5, workout_id: 2, date: '2026-11-13', reps: 6, weight: 210 },
        ],
      }],
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const router = createMemoryRouter([{ path: '/athletes/:id/records', element: <RecordsPage /> }], { initialEntries: ['/athletes/7/records'] })
    render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)

    expect(await screen.findByRole('heading', { name: 'Personal Records' })).toBeTruthy()
    expect((await screen.findAllByText('252 lb')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('5RM').length).toBeGreaterThan(0)
    expect(screen.getAllByRole('link', { name: /Back Squat/ })[0].getAttribute('href')).toBe('/athletes/7/exercises/3/history')
  })
})
